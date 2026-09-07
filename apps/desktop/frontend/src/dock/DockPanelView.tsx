import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { DockPointer } from "../lib/dock-bridge";
import { computeLayout, effectiveStyle } from "../lib/layout";
import { type MaterialStatus, materialClass } from "../lib/material";
import { truncateTitle } from "../lib/text";
import type { DockItem, DockViewState, WindowAction } from "../lib/types";
import { FolderPanel, type FolderPanelHandlers } from "./FolderPanel";
import { MediaPanel, type MediaPanelHandlers } from "./MediaPanel";
import "./dock.css";

let dockDragGesture = 0;
const nextDockDragGesture = () => ++dockDragGesture;
const clamp01 = (value: number) => Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
type DragState = {
  pointerId: number;
  startX: number;
  startY: number;
  rect: DOMRect;
  windowId: number;
  appId: number;
  gesture: number;
  suppress: boolean;
  terminal: boolean;
  dragging: boolean;
};
export interface DockPanelHandlers {
  onSelectWindow: (session: number, id: number) => void;
  onFocusWindow: (session: number, id: number, appId: number) => void;
  onAction: (session: number, kind: WindowAction, id: number, appId: number) => void;
  onSize: (session: number, width: number, height: number) => void;
  onSelectContent?: (session: number, revision: number, kind: string) => void;
  onRegions?: (session: number, revision: number, regions: PreviewRegion[]) => void;
  onBeginDrag?: (request: PreviewDragRequest) => Promise<void> | void;
  onCancelDrag?: (session: number, gesture: number) => Promise<void> | void;
  onFolderSort?: FolderPanelHandlers["onSort"];
  onRequestFolderAccess?: FolderPanelHandlers["onRequestAccess"];
  onCancelFolderAccess?: FolderPanelHandlers["onCancelAccess"];
  onOpenFolderEntry?: FolderPanelHandlers["onOpen"];
  media?: MediaPanelHandlers;
}
export interface PreviewDragRequest {
  session: number;
  gesture: number;
  windowId: number;
  appId: number;
  pointerX: number;
  pointerY: number;
  grabX: number;
  grabY: number;
}
export interface PreviewRegion {
  windowId: number;
  appId: number;
  bounds: { x: number; y: number; w: number; h: number };
}
export type DockPanelState = Omit<DockViewState, "item"> & { item?: DockItem };
export function DockPanelView({
  state,
  handlers,
  t = (text) => text,
  nativePointer,
  title,
  item = state.item,
  onClose,
  nativeHeader = false,
  materialStatus,
}: {
  state: DockPanelState;
  handlers: DockPanelHandlers;
  t?: (text: string) => string;
  nativePointer?: DockPointer | null;
  title?: string;
  item?: DockItem | null;
  onClose?: () => void;
  nativeHeader?: boolean;
  materialStatus?: MaterialStatus | null;
}) {
  dockDragGesture = Math.max(dockDragGesture, state.dragGestureFloor ?? 0);
  const drag = useRef<DragState | null>(null);
  const [, redrawDrag] = useState(0);
  const finishDrag = (pointerId: number, cancel = true) => {
    const current = drag.current;
    if (!current || current.pointerId !== pointerId) return;
    if (current.gesture && cancel) void handlers.onCancelDrag?.(state.session, current.gesture);
    if (current.suppress) {
      current.terminal = true;
      current.dragging = false;
    } else drag.current = null;
    redrawDrag((n) => n + 1);
  };
  useEffect(() => {
    const cancel = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || !drag.current?.gesture) return;
      void handlers.onCancelDrag?.(state.session, drag.current.gesture);
      drag.current.terminal = true;
      drag.current.dragging = false;
      drag.current.suppress = true;
      redrawDrag((n) => n + 1);
    };
    window.addEventListener("keydown", cancel);
    return () => window.removeEventListener("keydown", cancel);
  }, [handlers.onCancelDrag, state.session]);
  const [hoveredWindowId, setHoveredWindowId] = useState(0);
  const lastPointerTarget = useRef(0);
  const a = state.appearance;
  const count = state.entries.length;
  const style = effectiveStyle(a.style, count, a.compactThreshold);
  const vertical = a.layoutDirection === "vertical";
  const layout = computeLayout({
    count,
    maxColumns: a.maxColumns,
    maxRows: a.maxRows,
    thumbnailMaxPx: a.thumbnailMaxPx,
    autoSize: a.autoSize,
    layoutDirection: a.layoutDirection,
  });
  const rows = Math.max(
    1,
    Math.min(a.maxRows, vertical ? count : Math.ceil(count / Math.max(1, a.maxColumns))),
  );
  const columns =
    style === "titles"
      ? 1
      : Math.max(1, Math.min(a.maxColumns, vertical ? Math.ceil(count / rows) : count));
  const cellWidth =
    style === "titles"
      ? Math.max(160, a.titleMaxWidthPx)
      : style === "appIcons"
        ? Math.max(96, a.iconSizePx + 20)
        : layout.thumbnailPx;
  const imageHeight =
    style === "titles"
      ? 0
      : style === "appIcons"
        ? a.iconSizePx
        : Math.round(layout.thumbnailPx * 0.625);
  const previewHeight = Math.max(120, Math.min(600, Math.round(layout.thumbnailPx * 1.25)));
  const cardHeight =
    style === "titles"
      ? Math.max(42, a.fontSizePx * 1.4 + 16)
      : imageHeight + (a.showTitle ? a.fontSizePx * 1.4 + 8 : 0) + 12;
  const visibleRows = style === "titles" ? Math.min(a.maxRows, Math.max(1, count)) : rows;
  const selected = state.entries.find((e) => e.windowId === state.selectedWindowId);
  const preview = a.previewSelected ? selected?.preview || selected?.thumbnail : undefined;
  const cardSpacingPx = Math.max(0, Math.min(24, state.cardSpacingPx ?? 7));
  const contentOptions = state.contentOptions?.filter(
    (kind): kind is "windows" | "media" => kind === "windows" || kind === "media",
  );
  const contentWidth =
    state.contentKind === "folder" || state.contentKind === "media"
      ? 420
      : Math.max(280, columns * (cellWidth + 12) + (columns - 1) * cardSpacingPx);
  const ref = useRef<HTMLDivElement>(null);
  const lastSize = useRef("");
  const regionState = useRef({ session: 0, revision: 0, signature: "" });
  const regionsHandler = useRef(handlers.onRegions);
  const regionsSession = useRef(state.session);
  regionsHandler.current = handlers.onRegions;
  regionsSession.current = state.session;
  const entryIdentity = state.entries.map((entry) => entry.windowId).join(",");
  useEffect(() => {
    const resolve = () => {
      if (
        (state.contentKind !== "windows" && state.contentKind !== undefined) ||
        !nativePointer ||
        nativePointer.session !== state.session
      ) {
        setHoveredWindowId(0);
        lastPointerTarget.current = 0;
        return;
      }
      let id = 0;
      if (nativePointer.inside) {
        const target = document.elementFromPoint?.(nativePointer.x, nativePointer.y);
        const article = target?.closest<HTMLElement>("article[data-window-id]");
        if (article) id = Number(article.dataset.windowId ?? 0);
      }
      setHoveredWindowId(id);
      if (id && id !== lastPointerTarget.current) handlers.onSelectWindow(state.session, id);
      lastPointerTarget.current = id;
    };
    resolve();
    const viewport = ref.current?.querySelector(".ot-dock-list-viewport");
    viewport?.addEventListener("scroll", resolve, { passive: true });
    return () => viewport?.removeEventListener("scroll", resolve);
  }, [
    nativePointer,
    state.session,
    handlers.onSelectWindow,
    entryIdentity,
    style,
    contentWidth,
    cardSpacingPx,
    cardHeight,
    visibleRows,
    state.contentKind,
  ]);
  useLayoutEffect(() => {
    if (!handlers.onRegions || (state.contentKind !== "windows" && state.contentKind !== undefined))
      return;
    const panel = ref.current?.closest<HTMLElement>(".ot-dock-panel");
    const viewport = ref.current?.querySelector<HTMLElement>(".ot-dock-list-viewport");
    if (!panel) return;
    const round = (value: number) => Math.round(value * 100) / 100;
    const report = () => {
      if (regionState.current.session !== state.session) {
        if (regionState.current.session)
          handlers.onRegions?.(regionState.current.session, ++regionState.current.revision, []);
        regionState.current = { session: state.session, revision: 0, signature: "" };
      }
      const clips = [panel.getBoundingClientRect()];
      if (viewport) clips.push(viewport.getBoundingClientRect());
      const entries = new Map(state.entries.map((entry) => [entry.windowId, entry]));
      const regions: PreviewRegion[] = [];
      for (const article of ref.current?.querySelectorAll<HTMLElement>("article[data-window-id]") ??
        []) {
        const windowId = Number(article.dataset.windowId ?? 0);
        const entry = entries.get(windowId);
        if (!entry) continue;
        let { left, top, right, bottom } = article.getBoundingClientRect();
        for (const clip of clips) {
          left = Math.max(left, clip.left);
          top = Math.max(top, clip.top);
          right = Math.min(right, clip.right);
          bottom = Math.min(bottom, clip.bottom);
        }
        if (right <= left || bottom <= top) continue;
        regions.push({
          windowId,
          appId: entry.appId,
          bounds: { x: round(left), y: round(top), w: round(right - left), h: round(bottom - top) },
        });
      }
      const signature = JSON.stringify(regions);
      if (signature === regionState.current.signature) return;
      regionState.current.signature = signature;
      const revision = ++regionState.current.revision;
      handlers.onRegions?.(state.session, revision, regions);
    };
    report();
    viewport?.addEventListener("scroll", report, { passive: true });
    if (typeof ResizeObserver === "undefined")
      return () => viewport?.removeEventListener("scroll", report);
    const observer = new ResizeObserver(report);
    observer.observe(panel);
    if (viewport) observer.observe(viewport);
    for (const article of ref.current?.querySelectorAll<HTMLElement>("article[data-window-id]") ??
      [])
      observer.observe(article);
    return () => {
      viewport?.removeEventListener("scroll", report);
      observer.disconnect();
    };
  }, [
    handlers.onRegions,
    state.session,
    entryIdentity,
    style,
    contentWidth,
    cardSpacingPx,
    cardHeight,
    visibleRows,
    state.contentKind,
  ]);
  useEffect(
    () => () => {
      if (!regionsHandler.current || regionState.current.session !== regionsSession.current) return;
      regionsHandler.current(regionsSession.current, ++regionState.current.revision, []);
    },
    [],
  );
  // Measure intrinsic content, not the viewport-capped outer panel or its scroll
  // extent. List overflow has an explicit row limit; native display clamping
  // cannot feed back into the desired content width/height.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const report = () => {
      const rect = el.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) return;
      const width = Math.ceil(rect.width + 22),
        height = Math.ceil(rect.height + 22);
      const key = `${state.session}:${width}:${height}`;
      if (lastSize.current === key) return;
      lastSize.current = key;
      handlers.onSize(state.session, width, height);
    };
    report();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(report);
    observer.observe(el);
    return () => observer.disconnect();
  }, [
    state.session,
    handlers.onSize,
    count,
    state.error,
    contentWidth,
    cardSpacingPx,
    cardHeight,
    visibleRows,
    preview,
    previewHeight,
    style,
    a.showTitle,
    a.fontSizePx,
  ]);
  return (
    <div
      className={`ot-dock-panel ot-theme-${a.theme} ${materialClass(a.blur && (!state.contentKind || state.contentKind === "windows"), materialStatus, state.session)}${state.previewDragEnabled ? " ot-dock-drag-enabled" : ""}${nativeHeader ? " ot-dock-native-header" : ""}`}
      style={
        {
          "--ot-accent": a.accentColor,
          "--ot-dock-radius": `${a.cornerRadiusPx}px`,
          "--ot-dock-font": `${a.fontSizePx}px`,
          "--ot-dock-opacity": a.backgroundOpacity,
          "--ot-dock-columns": columns,
          "--ot-dock-rows": rows,
          "--ot-dock-cell": `${cellWidth + 12}px`,
          "--ot-dock-card-height": `${cardHeight}px`,
          "--ot-dock-image-height": `${imageHeight}px`,
          "--ot-dock-preview-height": `${previewHeight}px`,
          "--ot-dock-icon": `${a.iconSizePx}px`,
          "--ot-dock-spacing": `${cardSpacingPx}px`,
          "--ot-dock-list-height": `${visibleRows * cardHeight + (visibleRows - 1) * cardSpacingPx}px`,
        } as React.CSSProperties
      }
    >
      <div
        ref={ref}
        className="ot-dock-content"
        style={{ width: contentWidth, paddingTop: nativeHeader ? 32 : undefined }}
      >
        {state.contentKind !== "media" ? (
          <header className={nativeHeader ? "ot-dock-native-titlebar" : undefined}>
            <span>{title ?? item?.title}</span>
            {onClose ? (
              <button
                type="button"
                className={nativeHeader ? "ot-dock-native-close" : undefined}
                aria-label={t("Close preview")}
                onClick={onClose}
              >
                ×
              </button>
            ) : null}
          </header>
        ) : null}
        {(contentOptions?.length ?? 0) > 1 ? (
          <div className="ot-dock-content-selector" role="group" aria-label={t("Preview content")}>
            {contentOptions?.map((kind) => (
              <button
                type="button"
                key={kind}
                aria-pressed={state.contentKind === kind}
                onClick={() => handlers.onSelectContent?.(state.session, state.revision ?? 0, kind)}
              >
                {t(kind === "windows" ? "Windows" : "Media")}
              </button>
            ))}
          </div>
        ) : null}
        {state.contentKind === "media" && state.media && handlers.media ? (
          <MediaPanel state={state.media} handlers={handlers.media} t={t} />
        ) : state.contentKind === "folder" && state.folder ? (
          <FolderPanel
            key={`${state.session}:${state.folder.folderIdentity}`}
            session={state.session}
            revision={state.revision ?? 0}
            folder={state.folder}
            t={t}
            handlers={{
              onSort: handlers.onFolderSort ?? (() => {}),
              onRequestAccess: handlers.onRequestFolderAccess ?? (() => {}),
              onCancelAccess: handlers.onCancelFolderAccess ?? (() => {}),
              onOpen: handlers.onOpenFolderEntry ?? (() => {}),
            }}
          />
        ) : (
          <>
            {state.previewDragEnabled ? (
              <span className="ot-dock-drag-hint">{t("Drag a preview to move its window")}</span>
            ) : null}
            {state.error ? (
              <p role="alert" className="ot-dock-error">
                {state.error}
              </p>
            ) : null}
            {count ? (
              <div className="ot-dock-list-viewport">
                <div
                  className={`ot-dock-list ot-dock-style-${style}${vertical && style !== "titles" ? " ot-dock-vertical" : ""}`}
                >
                  {state.entries.map((e) => {
                    const title = truncateTitle(e.title || e.appName, a.titleTruncation);
                    const image = style === "appIcons" ? e.icon : e.thumbnail;
                    return (
                      <article
                        key={e.windowId}
                        data-window-id={e.windowId}
                        className={`${e.windowId === state.selectedWindowId ? "is-selected" : ""}${e.windowId === hoveredWindowId ? " is-hovered" : ""}`}
                        onMouseEnter={() => handlers.onSelectWindow(state.session, e.windowId)}
                      >
                        <button
                          type="button"
                          className={`ot-dock-preview${drag.current?.windowId === e.windowId && drag.current.dragging ? " is-dragging" : ""}`}
                          aria-label={`Focus ${e.title || e.appName}`}
                          draggable={false}
                          onPointerDown={(event) => {
                            if (
                              !state.previewDragEnabled ||
                              event.button !== 0 ||
                              event.isPrimary === false
                            )
                              return;
                            drag.current = {
                              pointerId: event.pointerId,
                              startX: event.clientX,
                              startY: event.clientY,
                              rect: event.currentTarget.getBoundingClientRect(),
                              windowId: e.windowId,
                              appId: e.appId,
                              gesture: 0,
                              suppress: false,
                              terminal: false,
                              dragging: false,
                            };
                            try {
                              event.currentTarget.setPointerCapture(event.pointerId);
                            } catch {
                              /* optional in WebKit */
                            }
                          }}
                          onPointerMove={(event) => {
                            const current = drag.current;
                            if (
                              !current ||
                              current.pointerId !== event.pointerId ||
                              current.gesture ||
                              current.terminal ||
                              Math.hypot(
                                event.clientX - current.startX,
                                event.clientY - current.startY,
                              ) < 6
                            )
                              return;
                            current.gesture = nextDockDragGesture();
                            current.suppress = true;
                            current.dragging = true;
                            redrawDrag((n) => n + 1);
                            const gesture = current.gesture;
                            void Promise.resolve(
                              handlers.onBeginDrag?.({
                                session: state.session,
                                gesture,
                                windowId: current.windowId,
                                appId: current.appId,
                                pointerX: event.screenX,
                                pointerY: event.screenY,
                                grabX: clamp01(
                                  (current.startX - current.rect.left) / current.rect.width,
                                ),
                                grabY: clamp01(
                                  (current.startY - current.rect.top) / current.rect.height,
                                ),
                              }),
                            )
                              .catch(() => {})
                              .finally(() => {
                                if (drag.current?.gesture === gesture) {
                                  drag.current.terminal = true;
                                  drag.current.dragging = false;
                                  drag.current.suppress = true;
                                  redrawDrag((n) => n + 1);
                                }
                              });
                          }}
                          onPointerUp={(event) => finishDrag(event.pointerId)}
                          onPointerCancel={(event) => {
                            const target = event.currentTarget;
                            const current = drag.current;
                            finishDrag(event.pointerId, false);
                            if (current?.gesture)
                              window.setTimeout(() => {
                                if (target.isConnected && drag.current === current)
                                  void handlers.onCancelDrag?.(state.session, current.gesture);
                              }, 0);
                          }}
                          onClick={(event) => {
                            if (drag.current?.suppress) {
                              event.preventDefault();
                              event.stopPropagation();
                              drag.current = null;
                              redrawDrag((n) => n + 1);
                              return;
                            }
                            drag.current = null;
                            handlers.onFocusWindow(state.session, e.windowId, e.appId);
                          }}
                        >
                          {style !== "titles" ? (
                            <span className="ot-dock-image">
                              {image ? (
                                <img src={image} alt="" />
                              ) : (
                                <span aria-hidden="true">{e.appName.trim()[0] || "?"}</span>
                              )}
                              {style === "thumbnails" && a.showAppBadge && e.icon ? (
                                <img className="ot-dock-badge" src={e.icon} alt="" />
                              ) : null}
                            </span>
                          ) : null}
                          {a.showTitle || style === "titles" ? (
                            <span className="ot-dock-title" title={e.title || e.appName}>
                              {title}
                            </span>
                          ) : null}
                          {a.showStatusIcons && (e.minimized || e.hidden || e.fullscreen) ? (
                            <span
                              className="ot-dock-status"
                              aria-label={[
                                e.minimized && "Minimized",
                                e.hidden && "Hidden",
                                e.fullscreen && "Fullscreen",
                              ]
                                .filter(Boolean)
                                .join(", ")}
                            >
                              {e.minimized ? "−" : ""}
                              {e.hidden ? "◌" : ""}
                              {e.fullscreen ? "↗" : ""}
                            </span>
                          ) : null}
                          {a.showSpaceNumbers && e.spaceId ? (
                            <span className="ot-dock-space" aria-label={`Space ${e.spaceId}`}>
                              {e.spaceId}
                            </span>
                          ) : null}
                        </button>
                        {a.showWindowControls ? (
                          <div className="ot-dock-controls">
                            <button
                              aria-label={t("Close window")}
                              onClick={() =>
                                handlers.onAction(state.session, "close", e.windowId, e.appId)
                              }
                            >
                              ×
                            </button>
                            <button
                              aria-label={t("Minimize window")}
                              onClick={() =>
                                handlers.onAction(state.session, "minimize", e.windowId, e.appId)
                              }
                            >
                              −
                            </button>
                            <button
                              aria-label={t("Fullscreen window")}
                              onClick={() =>
                                handlers.onAction(state.session, "fullscreen", e.windowId, e.appId)
                              }
                            >
                              ↗
                            </button>
                          </div>
                        ) : null}
                      </article>
                    );
                  })}
                </div>
              </div>
            ) : (
              <p className="ot-dock-empty">
                {t(
                  state.emptyReason === "filtered"
                    ? "No windows match these filters"
                    : state.emptyReason === "loading"
                      ? "Loading windows…"
                      : state.emptyReason === "notRunning"
                        ? "App is not running"
                        : state.emptyReason === "unavailable"
                          ? "Windows unavailable"
                          : "No open windows",
                )}
              </p>
            )}
            {preview ? (
              <div
                className={`ot-dock-selected-preview${a.previewFade ? " ot-dock-preview-fade" : ""}`}
                aria-label="Selected window preview"
              >
                <img key={selected?.windowId} src={preview} alt="" />
              </div>
            ) : null}
            {item && item.appId > 0 && state.emptyReason !== "notRunning" ? (
              <div className="ot-dock-app-actions">
                <button
                  onClick={() => handlers.onAction(state.session, "newWindow", 0, item.appId)}
                >
                  {t("New window")}
                </button>
                <button onClick={() => handlers.onAction(state.session, "hide", 0, item.appId)}>
                  {t("Hide app")}
                </button>
                <button onClick={() => handlers.onAction(state.session, "quit", 0, item.appId)}>
                  {t("Quit app")}
                </button>
              </div>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}

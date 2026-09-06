import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { DockPointer } from "../lib/dock-bridge";
import { computeLayout, effectiveStyle } from "../lib/layout";
import { truncateTitle } from "../lib/text";
import type { DockViewState, WindowAction } from "../lib/types";
import "./dock.css";
export interface DockPanelHandlers {
  onSelectWindow: (session: number, id: number) => void;
  onFocusWindow: (session: number, id: number, appId: number) => void;
  onAction: (session: number, kind: WindowAction, id: number, appId: number) => void;
  onSize: (session: number, width: number, height: number) => void;
}
export function DockPanelView({
  state,
  handlers,
  t = (text) => text,
  nativePointer,
}: {
  state: DockViewState;
  handlers: DockPanelHandlers;
  t?: (text: string) => string;
  nativePointer?: DockPointer | null;
}) {
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
  const contentWidth = Math.max(280, columns * (cellWidth + 12) + (columns - 1) * 7);
  const ref = useRef<HTMLDivElement>(null);
  const lastSize = useRef("");
  const entryIdentity = state.entries.map((entry) => entry.windowId).join(",");
  useEffect(() => {
    const resolve = () => {
      if (!nativePointer || nativePointer.session !== state.session) {
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
    cardHeight,
    visibleRows,
  ]);
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
      className={`ot-dock-panel ot-theme-${a.theme}${a.blur ? " ot-dock-blur" : ""}`}
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
          "--ot-dock-list-height": `${visibleRows * cardHeight + (visibleRows - 1) * 7}px`,
        } as React.CSSProperties
      }
    >
      <div ref={ref} className="ot-dock-content" style={{ width: contentWidth }}>
        <header>{state.item.title}</header>
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
                      className="ot-dock-preview"
                      aria-label={`Focus ${e.title || e.appName}`}
                      onClick={() => handlers.onFocusWindow(state.session, e.windowId, e.appId)}
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
        {state.item.appId > 0 && state.emptyReason !== "notRunning" ? (
          <div className="ot-dock-app-actions">
            <button
              onClick={() => handlers.onAction(state.session, "newWindow", 0, state.item.appId)}
            >
              {t("New window")}
            </button>
            <button onClick={() => handlers.onAction(state.session, "hide", 0, state.item.appId)}>
              {t("Hide app")}
            </button>
            <button onClick={() => handlers.onAction(state.session, "quit", 0, state.item.appId)}>
              {t("Quit app")}
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

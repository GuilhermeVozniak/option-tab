import { useEffect, useMemo, useRef, useState } from "react";
import { type KeyPayload, onSwitcherKey } from "../lib/bridge";
import { keyToAction } from "../lib/keymap";
import { computeLayout, effectiveStyle } from "../lib/layout";
import {
  type MaterialRect,
  type MaterialStatus,
  materialClass,
  useMaterialReporter,
} from "../lib/material";
import { truncateTitle } from "../lib/text";
import type { Entry, PointerAction, SwitcherState } from "../lib/types";
import { useSwitcherGestureRegions } from "../lib/useSwitcherGestureRegions";
import { StatusIcons } from "../overlay/StatusIcons";
import type { OverlayHandlers } from "../overlay/types";
import "./app-switcher.css";

export interface AppSwitcherProps {
  state: SwitcherState;
  handlers: OverlayHandlers;
  nativeKeys?: boolean;
  t?: (text: string) => string;
  material?: { status?: MaterialStatus | null; onRect?: (rect: MaterialRect) => void };
}

function runPointerAction(action: PointerAction, entry: Entry, handlers: OverlayHandlers) {
  if (action === "close") handlers.onClose(entry.windowId);
  else if (action === "minimize") handlers.onMinimize(entry.windowId);
  else if (action === "fullscreen") handlers.onFullscreen(entry.windowId);
  else if (action === "hide") handlers.onHide(entry.appId);
  else if (action === "quit") handlers.onQuit(entry.appId);
}

function AppWindowCard({
  entry,
  selected,
  handlers,
  focus,
  hover,
  showControls,
  middle,
  appearance,
  style,
  thumbnailPx,
  spaceNumber,
  activeSpaceId,
  t,
}: {
  entry: Entry;
  selected: boolean;
  handlers: OverlayHandlers;
  focus: (entry: Entry) => void;
  hover: boolean;
  showControls: boolean;
  middle: PointerAction;
  appearance: SwitcherState["appearance"];
  style: SwitcherState["appearance"]["style"];
  thumbnailPx: number;
  spaceNumber?: number;
  activeSpaceId: number;
  t: (text: string) => string;
}) {
  const title = truncateTitle(entry.title || entry.appName, appearance.titleTruncation);
  const icon = entry.icon ? (
    <img src={entry.icon} alt="" />
  ) : (
    <span>{entry.appName.trim()[0] ?? "?"}</span>
  );
  return (
    <article
      className={selected ? "is-selected" : ""}
      data-switcher-gesture-window={entry.windowId}
      data-switcher-gesture-app={entry.appId}
    >
      <button
        type="button"
        className="ot-app-preview"
        aria-label={`Focus ${entry.title || entry.appName}`}
        onMouseEnter={hover ? () => handlers.onSelectAppWindow?.(entry.windowId) : undefined}
        onAuxClick={(event) => {
          if (event.button === 1) runPointerAction(middle, entry, handlers);
        }}
        onClick={() => focus(entry)}
        style={{ width: style === "titles" ? appearance.titleMaxWidthPx : thumbnailPx }}
      >
        {style === "titles" ? (
          <span className={`ot-app-card-title ot-trunc-${appearance.titleTruncation}`}>
            {title}
          </span>
        ) : (
          <>
            {appearance.showTitle ? (
              <span
                className={`ot-app-card-title ot-trunc-${appearance.titleTruncation}`}
                style={{ maxWidth: appearance.titleMaxWidthPx }}
              >
                {title}
              </span>
            ) : null}
            <span className="ot-app-card-media">
              {style === "appIcons" ? (
                <span className="ot-app-card-fallback">{icon}</span>
              ) : entry.thumbnail ? (
                <img src={entry.thumbnail} alt="" />
              ) : (
                <span className="ot-app-card-fallback">{icon}</span>
              )}
              {style === "thumbnails" && appearance.showAppBadge && entry.thumbnail ? (
                <span className="ot-app-badge">{icon}</span>
              ) : null}
            </span>
          </>
        )}
      </button>
      {appearance.showStatusIcons ? (
        <StatusIcons
          minimized={entry.minimized}
          hidden={entry.hidden}
          fullscreen={entry.fullscreen}
          otherSpace={!!entry.spaceId && !!activeSpaceId && entry.spaceId !== activeSpaceId}
        />
      ) : null}
      {spaceNumber !== undefined ? (
        <span className="ot-space-badge" role="img" aria-label={`Space ${spaceNumber}`}>
          {spaceNumber}
        </span>
      ) : null}
      {showControls ? (
        <div className="ot-app-window-actions" data-switcher-gesture-exclude>
          <button
            className="ot-traffic ot-traffic-close"
            aria-label={t("Close window")}
            onClick={() => handlers.onClose(entry.windowId)}
          />
          <button
            className="ot-traffic ot-traffic-minimize"
            aria-label={t("Minimize window")}
            onClick={() => handlers.onMinimize(entry.windowId)}
          />
          <button
            className="ot-traffic ot-traffic-fullscreen"
            aria-label={t("Fullscreen window")}
            onClick={() => handlers.onFullscreen(entry.windowId)}
          />
        </div>
      ) : null}
    </article>
  );
}

export function AppSwitcher({
  state,
  handlers,
  nativeKeys = true,
  t = (text) => text,
  material,
}: AppSwitcherProps) {
  const apps = state.apps ?? [],
    selected = Math.max(0, Math.min(state.selected, apps.length - 1));
  const app = apps[selected];
  const appearance = state.appearance;
  const style = effectiveStyle(appearance.style, state.entries.length, appearance.compactThreshold);
  const layout = computeLayout({
    count: state.entries.length,
    maxColumns: appearance.maxColumns,
    maxRows: appearance.maxRows,
    thumbnailMaxPx: appearance.thumbnailMaxPx,
    autoSize: appearance.autoSize,
    viewportW: window.innerWidth,
    viewportH: window.innerHeight,
    showTitle: appearance.showTitle,
    previewEnabled: appearance.previewSelected,
    layoutDirection: appearance.layoutDirection,
  });
  const [shown, setShown] = useState(appearance.apparitionDelayMs <= 0);
  const [shownSession, setShownSession] = useState(state.session ?? 0);
  const presented =
    shown && (appearance.apparitionDelayMs <= 0 || shownSession === (state.session ?? 0));
  const gestureRef = useSwitcherGestureRegions(
    state.session ?? 0,
    state.revision ?? 0,
    state.entries,
    state.open && presented,
    handlers.onGestureRegions,
  );
  const panelRef = useMaterialReporter(
    state.session ?? 0,
    state.revision ?? 0,
    material?.onRect,
    shown,
  );
  const [closing, setClosing] = useState(false);
  const wasOpen = useRef(false);
  useEffect(() => {
    if (!state.open) return;
    setShownSession(state.session ?? 0);
    if (appearance.apparitionDelayMs <= 0) {
      setShown(true);
      return;
    }
    setShown(false);
    const timer = window.setTimeout(() => setShown(true), appearance.apparitionDelayMs);
    return () => window.clearTimeout(timer);
  }, [state.open, appearance.apparitionDelayMs, state.session]);
  useEffect(() => {
    if (state.open) {
      wasOpen.current = true;
      setClosing(false);
      return;
    }
    if (!wasOpen.current) return;
    wasOpen.current = false;
    if (!appearance.fadeOutAnimation || !shown) return;
    setClosing(true);
    const timer = window.setTimeout(() => setClosing(false), 180);
    return () => window.clearTimeout(timer);
  }, [state.open, appearance.fadeOutAnimation, shown]);
  const spaceOrdinals = useMemo(() => {
    const ids = [
      ...new Set(state.entries.map((entry) => entry.spaceId).filter((id): id is number => !!id)),
    ].sort((a, b) => a - b);
    return new Map(ids.map((id, index) => [id, index + 1]));
  }, [state.entries]);
  useEffect(() => {
    if (!state.open) return;
    const handle = (
      key: string,
      shift = false,
      code = "",
      ctrl = false,
      alt = false,
      meta = false,
    ) => {
      const action = keyToAction(
        { key, code, shiftKey: shift, ctrlKey: ctrl, altKey: alt, metaKey: meta },
        {
          actionBindings: state.actionBindings,
          arrowKeys: state.arrowKeys,
          vimKeys: state.vimKeys,
          layoutDirection: state.appearance.layoutDirection,
        },
      );
      const bound = action.kind;
      if (
        [
          "close",
          "minimize",
          "fullscreen",
          "hide",
          "quit",
          "newWindow",
          "forceQuit",
          "closeAll",
          "minimizeAll",
        ].includes(bound) &&
        app
      ) {
        const entry = state.entries.find((e) => e.windowId === (state.selectedWindowId ?? 0));
        if (bound === "close" && entry) handlers.onClose(entry.windowId);
        else if (bound === "minimize" && entry) handlers.onMinimize(entry.windowId);
        else if (bound === "fullscreen" && entry) handlers.onFullscreen(entry.windowId);
        else if (bound === "hide") handlers.onHide(app.appId);
        else if (bound === "quit") handlers.onQuit(app.appId);
        else if (
          bound === "newWindow" ||
          bound === "forceQuit" ||
          bound === "closeAll" ||
          bound === "minimizeAll"
        )
          handlers.onAction?.(bound, entry?.windowId ?? 0, app.appId);
        return;
      }
      if (bound === "advance") {
        handlers.onAdvance();
        return;
      }
      if (bound === "reverse") {
        handlers.onReverse();
        return;
      }
      const horizontal = state.appearance.layoutDirection !== "vertical";
      const orthogonal = horizontal
        ? key === "ArrowUp" || key === "ArrowDown"
        : key === "ArrowLeft" || key === "ArrowRight";
      if (state.arrowKeys && orthogonal && state.entries.length) {
        const current = Math.max(
          0,
          state.entries.findIndex((e) => e.windowId === (state.selectedWindowId ?? 0)),
        );
        const delta = key === "ArrowDown" || key === "ArrowRight" ? 1 : -1;
        handlers.onSelectAppWindow?.(
          state.entries[(current + delta + state.entries.length) % state.entries.length].windowId,
        );
        return;
      }
      if (bound === "confirm" && app) handlers.onConfirmApp?.(app.appId);
      else if (bound === "cancel") handlers.onCancel();
      else if (bound === "searchBackspace") handlers.onSearchChange(state.search.slice(0, -1));
      else if (action.kind === "searchAppend") handlers.onSearchChange(state.search + action.char);
    };
    if (nativeKeys)
      return onSwitcherKey((p: KeyPayload) =>
        (state.session ?? 0) > 0 && p.session !== state.session
          ? undefined
          : handle(p.key, p.shift, p.code, p.ctrl, p.alt, p.meta),
      );
    const dom = (e: KeyboardEvent) => {
      handle(e.key, e.shiftKey, e.code, e.ctrlKey, e.altKey, e.metaKey);
      e.preventDefault();
    };
    window.addEventListener("keydown", dom);
    return () => window.removeEventListener("keydown", dom);
  }, [state, handlers, nativeKeys, app]);
  if (state.open ? !presented : !closing) return null;
  const focus = (entry: Entry) => {
    handlers.onSelectAppWindow?.(entry.windowId);
    handlers.onConfirmWindow(entry.windowId);
  };
  return (
    <div
      className={`ot-app-switcher ot-theme-${appearance.theme} is-${appearance.layoutDirection} style-${style}${appearance.blur ? "" : " ot-no-blur"}${closing && !state.open ? " ot-closing" : ""}`}
      style={
        {
          "--ot-accent": state.appearance.accentColor,
          "--ot-bg-opacity": String(appearance.backgroundOpacity),
          "--ot-app-icon": `${state.appearance.iconSizePx}px`,
          "--ot-app-thumb": `${layout.thumbnailPx}px`,
          "--ot-app-columns": layout.columns,
          "--ot-app-rows": Math.min(layout.rows, appearance.maxRows),
          "--ot-app-radius": `${state.appearance.cornerRadiusPx}px`,
          "--ot-app-font": `${state.appearance.fontSizePx}px`,
        } as React.CSSProperties
      }
      role="dialog"
      aria-label="Application switcher"
    >
      <section
        ref={(element) => {
          panelRef(element);
          gestureRef(element);
        }}
        className={`ot-app-panel ${materialClass(appearance.blur, material?.status, state.session)}`}
      >
        {state.search ? <div className="ot-app-search">{state.search}</div> : null}
        <div className="ot-app-rail" role="listbox" aria-label="Applications">
          {apps.map((item, index) => (
            <button
              type="button"
              role="option"
              aria-selected={index === selected}
              aria-label={`${item.appName} (${item.appId})`}
              key={item.appId}
              className={index === selected ? "is-selected" : ""}
              onMouseEnter={state.mouseHover ? () => handlers.onSelectApp?.(item.appId) : undefined}
              onClick={() => handlers.onSelectApp?.(item.appId)}
            >
              {item.icon ? (
                <img src={item.icon} alt="" />
              ) : (
                <span>{item.appName.trim()[0] ?? "?"}</span>
              )}
              <small>{item.appName}</small>
            </button>
          ))}
        </div>
        {app ? (
          <div className="ot-app-toolbar">
            <strong>{app.appName}</strong>
            <div>
              <button
                type="button"
                onClick={() => handlers.onConfirmApp?.(app.appId)}
                aria-label={`Open ${app.appName}`}
              >
                {t("Open app")}
              </button>
              <button type="button" onClick={() => handlers.onAction?.("newWindow", 0, app.appId)}>
                {t("New window")}
              </button>
              <button type="button" onClick={() => handlers.onHide(app.appId)}>
                {t("Hide app")}
              </button>
              <button type="button" onClick={() => handlers.onQuit(app.appId)}>
                {t("Quit app")}
              </button>
            </div>
          </div>
        ) : null}
        <div className="ot-app-gallery">
          {state.entries.map((entry) => (
            <AppWindowCard
              key={entry.windowId}
              entry={entry}
              selected={entry.windowId === state.selectedWindowId}
              handlers={handlers}
              focus={focus}
              hover={state.mouseHover}
              showControls={state.appearance.showWindowControls}
              middle={state.middleClickAction}
              appearance={appearance}
              style={style}
              thumbnailPx={layout.thumbnailPx}
              spaceNumber={
                appearance.showSpaceNumbers && entry.spaceId
                  ? spaceOrdinals.get(entry.spaceId)
                  : undefined
              }
              activeSpaceId={state.activeSpaceId}
              t={t}
            />
          ))}
        </div>
        {state.appearance.previewSelected &&
          (() => {
            const entry = state.entries.find((item) => item.windowId === state.selectedWindowId);
            const source = entry?.preview ?? entry?.thumbnail;
            return source ? (
              <div
                className={`ot-app-expanded-preview${state.appearance.previewFade ? " is-fading" : ""}`}
                aria-label="Selected window preview"
                style={
                  {
                    "--ot-preview-width": `${Math.round(layout.thumbnailPx * 1.6)}px`,
                  } as React.CSSProperties
                }
              >
                <img key={entry?.windowId} src={source} alt="" />
              </div>
            ) : null;
          })()}
        {!state.entries.length && app ? (
          <p className="ot-app-empty">
            {t(app.windowPresence === "unknown" ? "Windows unavailable" : "No open windows")}
          </p>
        ) : null}
      </section>
    </div>
  );
}

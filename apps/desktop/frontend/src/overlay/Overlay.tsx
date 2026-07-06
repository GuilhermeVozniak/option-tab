import { useEffect, useMemo, useRef, useState } from "react";
import { keyToAction } from "../lib/keymap";
import { computeLayout } from "../lib/layout";
import type { SwitcherState } from "../lib/types";
import { EntryItem } from "./EntryItem";
import type { OverlayHandlers } from "./types";

export type { OverlayHandlers };

interface OverlayProps {
  state: SwitcherState;
  handlers: OverlayHandlers;
}

// Overlay renders the window switcher in the configured visual style and wires
// global keyboard handling. It is a controlled component: all state comes from
// props (pushed by the Go controller) and all input flows out through handlers.
export function Overlay({ state, handlers }: OverlayProps) {
  const { open, entries, selected, search, appearance } = state;

  // Apparition delay: postpone the first paint so quick switches don't flash
  // the overlay (AltTab parity). 0 renders immediately.
  const delay = appearance.apparitionDelayMs;
  const [shown, setShown] = useState(delay <= 0);

  // The Go side resizes the overlay window to the target screen on show. Track
  // the real viewport with a ResizeObserver on the root element: it fires
  // reliably when the native window is resized programmatically (the window
  // `resize` event does not always), so the grid is laid out against the
  // actual window size on the very first show, even across screens of
  // different resolution.
  const [viewport, setViewport] = useState({ w: window.innerWidth, h: window.innerHeight });
  useEffect(() => {
    const measure = () => setViewport({ w: window.innerWidth, h: window.innerHeight });
    measure();
    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", measure);
      return () => window.removeEventListener("resize", measure);
    }
    const ro = new ResizeObserver(measure);
    ro.observe(document.documentElement);
    return () => ro.disconnect();
  }, []);
  useEffect(() => {
    if (!open) return;
    if (delay <= 0) {
      setShown(true);
      return;
    }
    setShown(false);
    const t = setTimeout(() => setShown(true), delay);
    return () => clearTimeout(t);
  }, [open, delay]);

  // Fade-out: keep the last frame mounted briefly with a closing class.
  const [closing, setClosing] = useState(false);
  const wasOpen = useRef(false);
  useEffect(() => {
    if (open) {
      wasOpen.current = true;
      setClosing(false);
      return;
    }
    if (!wasOpen.current) return;
    wasOpen.current = false;
    if (!appearance.fadeOutAnimation || !shown) return;
    setClosing(true);
    const t = setTimeout(() => setClosing(false), 180);
    return () => clearTimeout(t);
  }, [open, appearance.fadeOutAnimation, shown]);

  // Space number badges: label Spaces 1..N by their sorted ids among the
  // currently listed windows (raw CGS ids are not user-meaningful).
  const spaceOrdinals = useMemo(() => {
    const ids = [...new Set(entries.map((e) => e.spaceId).filter((v): v is number => !!v))].sort(
      (a, b) => a - b,
    );
    return new Map(ids.map((id, i) => [id, i + 1]));
  }, [entries]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      const action = keyToAction(e, { vimKeys: state.vimKeys, arrowKeys: state.arrowKeys });
      if (action.kind === "none") return;
      e.preventDefault();
      const sel = entries[selected];
      switch (action.kind) {
        case "advance":
          handlers.onAdvance();
          break;
        case "reverse":
          handlers.onReverse();
          break;
        case "confirm":
          handlers.onConfirm();
          break;
        case "cancel":
          handlers.onCancel();
          break;
        case "close":
          if (sel) handlers.onClose(sel.windowId);
          break;
        case "minimize":
          if (sel) handlers.onMinimize(sel.windowId);
          break;
        case "fullscreen":
          if (sel) handlers.onFullscreen(sel.windowId);
          break;
        case "quit":
          if (sel) handlers.onQuit(sel.appId);
          break;
        case "hide":
          if (sel) handlers.onHide(sel.appId);
          break;
        case "searchAppend":
          handlers.onSearchChange(search + action.char);
          break;
        case "searchBackspace":
          handlers.onSearchChange(search.slice(0, -1));
          break;
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, search, handlers, entries, selected, state.vimKeys]);

  if (open ? !shown : !closing) return null;

  const layout = computeLayout({
    count: entries.length,
    maxColumns: appearance.maxColumns,
    maxRows: appearance.maxRows,
    thumbnailMaxPx: appearance.thumbnailMaxPx,
    autoSize: appearance.autoSize,
    viewportW: viewport.w,
    viewportH: viewport.h,
    showTitle: appearance.showTitle,
    previewEnabled: appearance.previewSelected,
  });

  const style = state.style;
  const accent = appearance.accentColor;

  const selectedEntry = entries[selected];

  return (
    <div
      className={`ot-overlay ot-theme-${appearance.theme}${appearance.blur ? "" : " ot-no-blur"}${closing && !open ? " ot-closing" : ""}`}
      data-style={style}
      role="dialog"
      aria-label="Window switcher"
      onClick={(e) => {
        // The window now covers the whole screen; clicking the empty backdrop
        // outside the panel dismisses the switcher.
        if (e.target === e.currentTarget) handlers.onCancel();
      }}
      style={
        {
          "--ot-accent": accent,
          "--ot-bg-opacity": String(appearance.backgroundOpacity),
          "--ot-radius": `${appearance.cornerRadiusPx}px`,
          "--ot-font": `${appearance.fontSizePx}px`,
        } as React.CSSProperties
      }
    >
      <div className="ot-panel">
        {search ? <div className="ot-search">{`🔍 ${search}`}</div> : null}
        <ul
          className="ot-list"
          role="listbox"
          aria-label="Open windows"
          style={
            style === "titles"
              ? undefined
              : ({
                  gridTemplateColumns: `repeat(${layout.columns}, max-content)`,
                } as React.CSSProperties)
          }
        >
          {entries.map((entry, index) => (
            <EntryItem
              key={entry.windowId}
              entry={entry}
              index={index}
              selected={index === selected}
              style={style}
              thumbnailPx={layout.thumbnailPx}
              iconSizePx={appearance.iconSizePx}
              titleMaxWidthPx={appearance.titleMaxWidthPx}
              showTitle={appearance.showTitle}
              showControls={appearance.showWindowControls}
              showStatusIcons={appearance.showStatusIcons}
              spaceNumber={
                appearance.showSpaceNumbers && entry.spaceId && spaceOrdinals.size > 1
                  ? spaceOrdinals.get(entry.spaceId)
                  : undefined
              }
              titleTruncation={appearance.titleTruncation}
              mouseHover={state.mouseHover}
              activeSpaceId={state.activeSpaceId}
              handlers={handlers}
            />
          ))}
        </ul>
        {appearance.previewSelected && (selectedEntry?.preview || selectedEntry?.thumbnail) ? (
          <div
            className={`ot-preview${appearance.previewFade ? " ot-preview-fade" : ""}`}
            aria-label="Selected window preview"
          >
            <img
              // Keyed by source so switching windows remounts the image and
              // replays the fade-in.
              key={selectedEntry.preview ?? selectedEntry.thumbnail}
              className="ot-preview-img"
              src={selectedEntry.preview ?? selectedEntry.thumbnail}
              alt=""
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}

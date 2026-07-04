import { useEffect, useMemo, useRef, useState } from "react";
import { keyToAction } from "../lib/keymap";
import { computeLayout } from "../lib/layout";
import { truncateTitle } from "../lib/text";
import type { Entry, SwitcherState } from "../lib/types";

export interface OverlayHandlers {
  onAdvance: () => void;
  onReverse: () => void;
  onConfirm: () => void;
  onCancel: () => void;
  onSelect: (index: number) => void;
  onSearchChange: (query: string) => void;
  onClose: (windowId: number) => void;
  onMinimize: (windowId: number) => void;
  onFullscreen: (windowId: number) => void;
  onQuit: (appId: number) => void;
  onHide: (appId: number) => void;
}

interface OverlayProps {
  state: SwitcherState;
  handlers: OverlayHandlers;
}

function initial(name: string): string {
  return (name.trim()[0] ?? "?").toUpperCase();
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
      const action = keyToAction(e, { vimKeys: state.vimKeys });
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
  });

  const style = state.style;
  const accent = appearance.accentColor;

  const selectedEntry = entries[selected];

  return (
    <div
      className={`ot-overlay ot-theme-${appearance.theme}${closing && !open ? " ot-closing" : ""}`}
      data-style={style}
      role="dialog"
      aria-label="Window switcher"
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
          <div className="ot-preview" aria-label="Selected window preview">
            <img
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

interface EntryItemProps {
  entry: Entry;
  index: number;
  selected: boolean;
  style: SwitcherState["style"];
  thumbnailPx: number;
  iconSizePx: number;
  titleMaxWidthPx: number;
  showTitle: boolean;
  showControls: boolean;
  showStatusIcons: boolean;
  spaceNumber?: number;
  titleTruncation: SwitcherState["appearance"]["titleTruncation"];
  mouseHover: boolean;
  activeSpaceId: number;
  handlers: OverlayHandlers;
}

// StatusIcons renders the small AltTab-style markers for a window's state:
// minimized, its app hidden, fullscreen, or living on another Space.
function StatusIcons({
  minimized,
  hidden,
  fullscreen,
  otherSpace,
}: {
  minimized: boolean;
  hidden: boolean;
  fullscreen: boolean;
  otherSpace: boolean;
}) {
  if (!minimized && !hidden && !fullscreen && !otherSpace) return null;
  return (
    <div className="ot-status">
      {minimized ? (
        <span
          className="ot-status-icon ot-status-min"
          role="img"
          aria-label="Minimized"
          title="Minimized"
        >
          –
        </span>
      ) : null}
      {hidden ? (
        <span
          className="ot-status-icon ot-status-hidden"
          role="img"
          aria-label="Hidden app"
          title="Hidden app"
        >
          ⊘
        </span>
      ) : null}
      {fullscreen ? (
        <span
          className="ot-status-icon ot-status-fs"
          role="img"
          aria-label="Fullscreen"
          title="Fullscreen"
        >
          ⇱
        </span>
      ) : null}
      {otherSpace ? (
        <span
          className="ot-status-icon ot-status-space"
          role="img"
          aria-label="On another Space"
          title="On another Space"
        >
          ⧉
        </span>
      ) : null}
    </div>
  );
}

function EntryItem({
  entry,
  index,
  selected,
  style,
  thumbnailPx,
  iconSizePx,
  titleMaxWidthPx,
  showTitle,
  showControls,
  showStatusIcons,
  spaceNumber,
  titleTruncation,
  mouseHover,
  activeSpaceId,
  handlers,
}: EntryItemProps) {
  const iconPx = style === "appIcons" ? Math.max(iconSizePx, 48) : iconSizePx;
  const otherSpace = !!entry.spaceId && !!activeSpaceId && entry.spaceId !== activeSpaceId;
  const glyph = entry.icon ? (
    <img className="ot-icon-img" src={entry.icon} alt="" />
  ) : (
    initial(entry.appName)
  );
  const titleText = truncateTitle(entry.title || entry.appName, titleTruncation);
  const maxWidth =
    style === "titles" ? undefined : style === "thumbnails" ? thumbnailPx : titleMaxWidthPx;

  return (
    <li
      role="option"
      aria-selected={selected}
      className={`ot-entry ot-entry-${style}${selected ? " ot-selected" : ""}`}
      onMouseEnter={mouseHover ? () => handlers.onSelect(index) : undefined}
      onClick={() => handlers.onConfirm()}
      style={{ maxWidth }}
    >
      {style === "thumbnails" ? (
        <>
          {showTitle ? (
            <div className="ot-titlebar" style={{ maxWidth: thumbnailPx }}>
              <span className="ot-titlebar-icon">{glyph}</span>
              <span className={`ot-titlebar-text ot-trunc-${titleTruncation}`}>{titleText}</span>
            </div>
          ) : null}
          <div
            className="ot-thumb"
            style={{ width: thumbnailPx, height: Math.round(thumbnailPx * 0.62) }}
          >
            {entry.thumbnail ? (
              <img className="ot-thumb-img" src={entry.thumbnail} alt="" />
            ) : (
              <span
                className="ot-thumb-fallback"
                style={{
                  width: Math.round(iconSizePx * 1.6),
                  height: Math.round(iconSizePx * 1.6),
                }}
              >
                {glyph}
              </span>
            )}
          </div>
        </>
      ) : (
        <>
          <span className="ot-icon" style={{ width: iconPx, height: iconPx }} aria-hidden="true">
            {glyph}
          </span>
          {showTitle ? (
            <div className="ot-meta">
              <span className="ot-app">{entry.appName}</span>
              {style === "titles" ? (
                <span className={`ot-title ot-trunc-${titleTruncation}`}>
                  {truncateTitle(entry.title, titleTruncation)}
                </span>
              ) : null}
            </div>
          ) : null}
        </>
      )}

      {showStatusIcons ? (
        <StatusIcons
          minimized={entry.minimized}
          hidden={entry.hidden}
          fullscreen={entry.fullscreen}
          otherSpace={otherSpace}
        />
      ) : null}

      {spaceNumber !== undefined ? (
        <span
          className="ot-space-badge"
          role="img"
          aria-label={`Space ${spaceNumber}`}
          title={`Space ${spaceNumber}`}
        >
          {spaceNumber}
        </span>
      ) : null}

      {showControls ? (
        <div className="ot-controls" onClick={(e) => e.stopPropagation()}>
          <button
            type="button"
            aria-label="Close window"
            className="ot-ctl ot-ctl-close"
            onClick={() => handlers.onClose(entry.windowId)}
          >
            ✕
          </button>
          <button
            type="button"
            aria-label="Minimize window"
            className="ot-ctl ot-ctl-min"
            onClick={() => handlers.onMinimize(entry.windowId)}
          >
            –
          </button>
          <button
            type="button"
            aria-label="Fullscreen window"
            className="ot-ctl ot-ctl-fs"
            onClick={() => handlers.onFullscreen(entry.windowId)}
          >
            ⇱
          </button>
        </div>
      ) : null}
    </li>
  );
}

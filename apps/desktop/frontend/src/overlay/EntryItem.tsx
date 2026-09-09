import { useRef } from "react";
import { truncateTitle } from "../lib/text";
import type { Entry, PointerAction, SwitcherState } from "../lib/types";
import { StatusIcons } from "./StatusIcons";
import type { OverlayHandlers } from "./types";

function initial(name: string): string {
  return (name.trim()[0] ?? "?").toUpperCase();
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
  showAppBadge: boolean;
  showControls: boolean;
  showStatusIcons: boolean;
  spaceNumber?: number;
  titleTruncation: SwitcherState["appearance"]["titleTruncation"];
  mouseHover: boolean;
  activeSpaceId: number;
  handlers: OverlayHandlers;
  middleClickAction: PointerAction;
  swipeUpAction: PointerAction;
  swipeDownAction: PointerAction;
}

// EntryItem renders one window in the active visual style: a titled thumbnail
// cell, a large app icon, or a compact title row — plus status markers, the
// Space badge, and the hover window controls.
export function EntryItem({
  entry,
  index,
  selected,
  style,
  thumbnailPx,
  iconSizePx,
  titleMaxWidthPx,
  showTitle,
  showAppBadge,
  showControls,
  showStatusIcons,
  spaceNumber,
  titleTruncation,
  mouseHover,
  activeSpaceId,
  handlers,
  middleClickAction,
  swipeUpAction,
  swipeDownAction,
}: EntryItemProps) {
  const gesture = useRef<{ id: number; y: number; fired: boolean } | null>(null);
  const suppressClick = useRef(false);
  const wheelGesture = useRef<{ total: number; fired: boolean; timer?: number }>({
    total: 0,
    fired: false,
  });
  const runAction = (action: PointerAction) => {
    if (action === "close") handlers.onClose(entry.windowId);
    else if (action === "minimize") handlers.onMinimize(entry.windowId);
    else if (action === "fullscreen") handlers.onFullscreen(entry.windowId);
    else if (action === "hide") handlers.onHide(entry.appId);
    else if (action === "quit") handlers.onQuit(entry.appId);
  };
  const iconPx = style === "appIcons" ? Math.max(iconSizePx, 48) : iconSizePx;
  const otherSpace = !!entry.spaceId && !!activeSpaceId && entry.spaceId !== activeSpaceId;
  const glyph = entry.icon ? (
    <img className="ot-icon-img" src={entry.icon} alt="" />
  ) : (
    initial(entry.appName)
  );
  const titleText = truncateTitle(entry.title || entry.appName, titleTruncation);
  // Thumbnails cells size naturally from the frame inside them; capping the
  // cell at thumbnailPx would squeeze the frame while the image keeps the
  // full width and overflows it horizontally.
  const maxWidth = style === "appIcons" ? titleMaxWidthPx : undefined;

  return (
    <li
      role="option"
      aria-selected={selected}
      className={`ot-entry ot-entry-${style}${selected ? " ot-selected" : ""}`}
      onMouseEnter={mouseHover ? () => handlers.onSelect(index) : undefined}
      onClick={(e) => {
        if (suppressClick.current) {
          suppressClick.current = false;
          e.preventDefault();
          return;
        }
        handlers.onConfirmWindow(entry.windowId);
      }}
      onMouseDown={(e) => {
        if (e.button === 1) {
          e.preventDefault();
          runAction(middleClickAction);
        }
      }}
      onPointerDown={(e) => {
        if (e.button === 0 && e.isPrimary)
          gesture.current = { id: e.pointerId, y: e.clientY, fired: false };
      }}
      onPointerMove={(e) => {
        const g = gesture.current;
        if (!g || g.id !== e.pointerId || g.fired || Math.abs(e.clientY - g.y) < 48) return;
        g.fired = true;
        suppressClick.current = true;
        runAction(e.clientY < g.y ? swipeUpAction : swipeDownAction);
      }}
      onPointerUp={() => {
        gesture.current = null;
        window.setTimeout(() => {
          suppressClick.current = false;
        }, 0);
      }}
      onPointerCancel={() => {
        gesture.current = null;
        suppressClick.current = false;
      }}
      onWheel={(e) => {
        const wheel = wheelGesture.current;
        if (wheel.timer !== undefined) window.clearTimeout(wheel.timer);
        wheel.timer = window.setTimeout(() => {
          wheelGesture.current = { total: 0, fired: false };
        }, 250);
        if (wheel.fired) return;
        if (wheel.total !== 0 && Math.sign(wheel.total) !== Math.sign(e.deltaY)) wheel.total = 0;
        wheel.total += e.deltaY;
        if (Math.abs(wheel.total) < 48) return;
        wheel.fired = true;
        runAction(wheel.total < 0 ? swipeUpAction : swipeDownAction);
      }}
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
              <>
                <img className="ot-thumb-img" src={entry.thumbnail} alt="" />
                {showAppBadge ? (
                  <span className="ot-thumb-icon ot-thumb-icon-badge">{glyph}</span>
                ) : null}
              </>
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
          <button
            type="button"
            aria-label="Hide app"
            title="Hide app"
            className="ot-ctl ot-ctl-hide"
            onClick={() => handlers.onHide(entry.appId)}
          >
            ⊘
          </button>
          <button
            type="button"
            aria-label="Quit app"
            title="Quit app"
            className="ot-ctl ot-ctl-quit"
            onClick={() => handlers.onQuit(entry.appId)}
          >
            ⏻
          </button>
        </div>
      ) : null}
    </li>
  );
}

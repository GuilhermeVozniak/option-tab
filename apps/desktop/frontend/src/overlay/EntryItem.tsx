import { truncateTitle } from "../lib/text";
import type { Entry, SwitcherState } from "../lib/types";
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
  showControls: boolean;
  showStatusIcons: boolean;
  spaceNumber?: number;
  titleTruncation: SwitcherState["appearance"]["titleTruncation"];
  mouseHover: boolean;
  activeSpaceId: number;
  handlers: OverlayHandlers;
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

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
  t?: (text: string) => string;
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
  t = (text) => text,
}: EntryItemProps) {
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
    <Card appearance="unstyled" asChild>
      <li
        data-switcher-gesture-window={entry.windowId}
        data-switcher-gesture-app={entry.appId}
        role="option"
        aria-selected={selected}
        className={`ot-entry ot-entry-${style}${selected ? " ot-selected" : ""}`}
        onMouseEnter={mouseHover ? () => handlers.onSelect(index) : undefined}
        onClick={() => handlers.onConfirmWindow(entry.windowId)}
        onMouseDown={(e) => {
          if (e.button === 1) {
            e.preventDefault();
            runAction(middleClickAction);
          }
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
                    <Badge variant="unstyled" className="ot-thumb-icon ot-thumb-icon-badge">
                      {glyph}
                    </Badge>
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
            t={t}
          />
        ) : null}

        {spaceNumber !== undefined ? (
          <Badge
            variant="unstyled"
            className="ot-space-badge"
            role="img"
            aria-label={t("Space {number}").replace("{number}", String(spaceNumber))}
            title={t("Space {number}").replace("{number}", String(spaceNumber))}
          >
            {spaceNumber}
          </Badge>
        ) : null}

        {showControls ? (
          <div
            data-switcher-gesture-exclude
            className="ot-controls"
            onClick={(e) => e.stopPropagation()}
          >
            <Button
              variant="unstyled"
              type="button"
              aria-label={t("Close window")}
              className="ot-ctl ot-ctl-close"
              onClick={() => handlers.onClose(entry.windowId)}
            >
              ✕
            </Button>
            <Button
              variant="unstyled"
              type="button"
              aria-label={t(entry.minimized ? "Restore window" : "Minimize window")}
              className="ot-ctl ot-ctl-min"
              onClick={() => handlers.onMinimize(entry.windowId)}
            >
              –
            </Button>
            <Button
              variant="unstyled"
              type="button"
              aria-label={t("Fullscreen window")}
              className="ot-ctl ot-ctl-fs"
              onClick={() => handlers.onFullscreen(entry.windowId)}
            >
              ⇱
            </Button>
            <Button
              variant="unstyled"
              type="button"
              aria-label={t("Hide app")}
              title={t("Hide app")}
              className="ot-ctl ot-ctl-hide"
              onClick={() => handlers.onHide(entry.appId)}
            >
              ⊘
            </Button>
            <Button
              variant="unstyled"
              type="button"
              aria-label={t("Quit app")}
              title={t("Quit app")}
              className="ot-ctl ot-ctl-quit"
              onClick={() => handlers.onQuit(entry.appId)}
            >
              ⏻
            </Button>
          </div>
        ) : null}
      </li>
    </Card>
  );
}

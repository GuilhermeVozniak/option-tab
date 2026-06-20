import { useEffect } from "react";
import { keyToAction } from "../lib/keymap";
import { computeLayout } from "../lib/layout";
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

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      const action = keyToAction(e);
      if (action.kind === "none") return;
      e.preventDefault();
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
  }, [open, search, handlers]);

  if (!open) return null;

  const layout = computeLayout({
    count: entries.length,
    maxColumns: appearance.maxColumns,
    maxRows: appearance.maxRows,
    thumbnailMaxPx: appearance.thumbnailMaxPx,
    autoSize: appearance.autoSize,
  });

  const style = state.style;
  const accent = appearance.accentColor;

  return (
    <div
      className={`ot-overlay ot-theme-${appearance.theme}`}
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
              handlers={handlers}
            />
          ))}
        </ul>
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
  handlers: OverlayHandlers;
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
  handlers,
}: EntryItemProps) {
  const iconPx = style === "appIcons" ? Math.max(iconSizePx, 48) : iconSizePx;
  return (
    <li
      role="option"
      aria-selected={selected}
      className={`ot-entry ot-entry-${style}${selected ? " ot-selected" : ""}`}
      onMouseEnter={() => handlers.onSelect(index)}
      onClick={() => handlers.onConfirm()}
      style={{ maxWidth: style === "titles" ? undefined : `${titleMaxWidthPx}px` }}
    >
      {style === "thumbnails" ? (
        <div
          className="ot-thumb"
          style={{ width: thumbnailPx, height: Math.round(thumbnailPx * 0.62) }}
        >
          <span className="ot-thumb-icon" style={{ width: iconSizePx, height: iconSizePx }}>
            {initial(entry.appName)}
          </span>
          {entry.minimized ? (
            <span className="ot-badge-min" role="img" aria-label="Minimized" />
          ) : null}
        </div>
      ) : (
        <span className="ot-icon" style={{ width: iconPx, height: iconPx }} aria-hidden="true">
          {initial(entry.appName)}
        </span>
      )}

      {showTitle ? (
        <div className="ot-meta">
          <span className="ot-app">{entry.appName}</span>
          {style !== "appIcons" ? <span className="ot-title">{entry.title}</span> : null}
        </div>
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
            aria-label="Quit app"
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

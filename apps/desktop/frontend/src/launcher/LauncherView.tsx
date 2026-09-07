import type { LauncherPresentation, LauncherWidgetNode } from "../lib/types";
import type { LauncherWidgetState, WidgetActions, WidgetLocalized } from "../lib/widget-types";
import { WidgetView } from "../widgets/WidgetView";
import { type LauncherItemCommand, LauncherItemStrip } from "./LauncherItemStrip";
import "./launcher.css";

function WidgetNode({ node }: { node: LauncherWidgetNode }) {
  if (node.kind === "text") return <span className="ot-launcher-widget-text">{node.text}</span>;
  return (
    <div className="ot-launcher-widget-row">
      {node.text ? <span>{node.text}</span> : null}
      {node.children?.map((child, index) => (
        <WidgetNode key={`${child.kind}-${index}`} node={child} />
      ))}
    </div>
  );
}

export function LauncherView({
  presentation,
  onActivate,
  onRelaunch,
  widgetState,
  widgetActions,
  onSelectWidget,
  t = (text) => text,
  language = "en",
}: {
  presentation: LauncherPresentation;
  onActivate: (
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ) => void;
  onRelaunch?: LauncherItemCommand;
  widgetState?: LauncherWidgetState | null;
  widgetActions?: WidgetActions;
  onSelectWidget?: (stackID: string, instanceID: string) => void;
  t?: (text: string) => string;
  language?: string;
}) {
  if (!presentation?.visible) return null;
  return (
    <main
      className={`ot-launcher-shell edge-${presentation.edge} theme-${presentation.appearance.theme} material-${presentation.appearance.material}`}
      style={
        {
          "--ot-launcher-icon": `${presentation.iconPx}px`,
          "--ot-launcher-tint": presentation.appearance.tint,
          "--ot-launcher-opacity": presentation.appearance.opacity,
          "--ot-launcher-border-opacity": presentation.appearance.borderOpacity,
          "--ot-launcher-radius": `${presentation.appearance.cornerRadiusPx}px`,
          "--ot-launcher-gap": `${presentation.appearance.itemSpacingPx}px`,
        } as React.CSSProperties
      }
      aria-label="Option Tab launcher"
    >
      <LauncherItemStrip
        key={`${presentation.epoch}:${presentation.session}:${presentation.displayUUID}`}
        presentation={presentation}
        onActivate={onActivate}
        onRelaunch={onRelaunch}
        t={t}
      />
      {widgetState === undefined
        ? presentation.widgets.map((widget) => (
            <aside className="ot-launcher-widget" key={widget.id} aria-label={widget.packageID}>
              {widget.status === "ready" ? <WidgetNode node={widget.root} /> : null}
            </aside>
          ))
        : null}
      {widgetState?.visible && widgetActions ? (
        <div className="ot-launcher-widgets" aria-label={t("Launcher widgets")}>
          {widgetState.slots.map((slot) => (
            <aside
              className="ot-launcher-widget"
              key={slot.id}
              aria-label={localName(slot.name, language)}
            >
              {slot.members.length > 1 ? (
                <div className="ot-launcher-widget-choices" aria-label={t("Widget stack")}>
                  {slot.members.map((member) => (
                    <button
                      type="button"
                      className={member.id === slot.selectedID ? "is-selected" : ""}
                      key={member.id}
                      onClick={() => onSelectWidget?.(slot.stackID ?? "", member.id)}
                    >
                      {localName(member.name, language)}
                    </button>
                  ))}
                </div>
              ) : null}
              {slot.state ? (
                <WidgetView state={slot.state} actions={widgetActions} t={t} />
              ) : (
                <span className="ot-widget-status">{t(statusLabel(slot.status))}</span>
              )}
            </aside>
          ))}
        </div>
      ) : null}
    </main>
  );
}

function localName(name: WidgetLocalized, language: string): string {
  return name[language] || name.en || Object.values(name)[0] || "Widget";
}

function statusLabel(status: string): string {
  if (status === "preparing") return "Loading widget…";
  if (status === "grantRequired") return "Permission required";
  return "Widget unavailable";
}

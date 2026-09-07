import type { LauncherPresentation, LauncherWidgetNode } from "../lib/types";
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
}: {
  presentation: LauncherPresentation;
  onActivate: (
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ) => void;
}) {
  if (!presentation?.visible) return null;
  return (
    <main
      className="ot-launcher-shell"
      style={{ "--ot-launcher-icon": `${presentation.iconPx}px` } as React.CSSProperties}
      aria-label="Option Tab launcher"
    >
      <ul className="ot-launcher-strip" aria-label="Running applications">
        {presentation.items.map((item) => (
          <li key={item.id}>
            <button
              type="button"
              aria-label={item.name}
              className="ot-launcher-app"
              onClick={() =>
                onActivate(
                  presentation.epoch,
                  presentation.displayUUID,
                  presentation.session,
                  presentation.revision,
                  item.id,
                )
              }
            >
              {item.icon ? (
                <img alt="" draggable={false} src={item.icon} />
              ) : (
                <span>{item.name[0]}</span>
              )}
              <small>{item.name}</small>
            </button>
          </li>
        ))}
      </ul>
      {presentation.widgets.map((widget) => (
        <aside className="ot-launcher-widget" key={widget.id} aria-label={widget.packageID}>
          {widget.status === "ready" ? <WidgetNode node={widget.root} /> : null}
        </aside>
      ))}
    </main>
  );
}

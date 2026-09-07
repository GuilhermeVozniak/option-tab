import { useEffect, useRef, useState } from "react";
import type { LauncherPresentation, LauncherPresentationItem } from "../lib/types";

import { useMagnification } from "./useMagnification";

export type LauncherItemCommand = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  itemID: string,
) => void;

export function LauncherItemStrip({
  presentation,
  onActivate,
  onRelaunch,
  onShowPanel,
  t,
}: {
  presentation: LauncherPresentation;
  onActivate: LauncherItemCommand;
  onRelaunch?: LauncherItemCommand;
  onShowPanel?: LauncherItemCommand;
  t: (text: string) => string;
}) {
  const [group, setGroup] = useState("");
  const [contextItem, setContextItem] = useState("");
  const itemsKey = JSON.stringify(presentation.items.map(itemIdentity));
  const [admittedItems, setAdmittedItems] = useState(itemsKey);
  useEffect(() => {
    setGroup("");
    setContextItem("");
    setAdmittedItems(itemsKey);
  }, [itemsKey]);
  const strip = useRef<HTMLUListElement>(null);
  const m = presentation.magnification;
  const magnified = !!m?.enabled && m.scale > 1;
  useMagnification(strip, {
    enabled: magnified,
    scale: m?.scale ?? 1,
    reach: m?.reach ?? 0,
    iconPx: presentation.iconPx,
    vertical: presentation.edge === "left" || presentation.edge === "right",
    owner: `${presentation.epoch}:${presentation.displayUUID}:${presentation.session}:${presentation.profileID}`,
    itemsKey: `${itemsKey}:${group}`,
  });
  const currentItems = admittedItems === itemsKey;
  const invoke = (command: LauncherItemCommand, id: string) => {
    setContextItem("");
    command(
      presentation.epoch,
      presentation.displayUUID,
      presentation.session,
      presentation.revision,
      id,
    );
  };
  const renderItem = (item: LauncherPresentationItem, member = false) => {
    const kind = item.kind || "app";
    if (kind === "spacer" || kind === "separator") {
      return <li key={item.id} className={`ot-launcher-${kind}`} aria-hidden="true" />;
    }
    const isGroup = kind === "group" && !member;
    const ready = !item.status || item.status === "ready";
    const canRelaunch =
      kind === "app" && ready && item.running && !!item.referenceRevision && !!onRelaunch;
    const status = ready
      ? item.running
        ? t("Running")
        : t(kindLabel(kind))
      : t(referenceStatus(item.status));
    const canShowPanel = !!onShowPanel && (kind === "folder" || (kind === "app" && !!item.running));
    const showContext = currentItems && contextItem === item.id && (canRelaunch || canShowPanel);
    return (
      <li className={`ot-launcher-item kind-${kind}`} key={item.id}>
        <button
          type="button"
          aria-label={item.name}
          className="ot-launcher-app"
          disabled={!ready && !isGroup}
          aria-description={status}
          title={`${item.name} · ${status}`}
          aria-expanded={
            isGroup
              ? currentItems && group === item.id
              : canRelaunch || canShowPanel
                ? !!showContext
                : undefined
          }
          onClick={() =>
            isGroup
              ? setGroup(currentItems && group === item.id ? "" : item.id)
              : kind === "folder" && onShowPanel
                ? invoke(onShowPanel, item.id)
                : invoke(onActivate, item.id)
          }
          onContextMenu={(event) => {
            if (canRelaunch || canShowPanel) {
              event.preventDefault();
              setContextItem(item.id);
            }
          }}
          onKeyDown={(event) => {
            if (
              (canRelaunch || canShowPanel) &&
              ((event.shiftKey && event.key === "F10") || event.key === "ContextMenu")
            ) {
              event.preventDefault();
              setContextItem(item.id);
            }
            if (event.key === "Escape") {
              setContextItem("");
              setGroup("");
            }
          }}
        >
          <span className="ot-launcher-visual">
            {item.icon.startsWith("data:image/png;base64,") ? (
              <img alt="" draggable={false} src={item.icon} />
            ) : (
              <span className="ot-launcher-icon-fallback" aria-hidden="true">
                {isGroup
                  ? "▦"
                  : kind === "folder"
                    ? "▱"
                    : kind === "link"
                      ? "↗"
                      : item.name.slice(0, 1)}
              </span>
            )}
            <small className={presentation.appearance.showLabels ? "" : "ot-launcher-label-hidden"}>
              {item.name}
            </small>
            {item.running ? <i className="ot-launcher-running" aria-hidden="true" /> : null}
            {!ready ? (
              <i className="ot-launcher-item-warning" aria-hidden="true">
                !
              </i>
            ) : null}
          </span>
        </button>
        {showContext ? (
          <div
            className="ot-launcher-item-actions"
            aria-label={`${t("Application actions")}: ${item.name}`}
          >
            {canShowPanel && onShowPanel ? (
              <button type="button" onClick={() => invoke(onShowPanel, item.id)}>
                {t(kind === "folder" ? "Show folder contents" : "Show all windows")}
              </button>
            ) : null}
            {kind === "folder" && ready ? (
              <button type="button" onClick={() => invoke(onActivate, item.id)}>
                {t("Open folder")}
              </button>
            ) : null}
            {canRelaunch && onRelaunch ? (
              <button type="button" onClick={() => invoke(onRelaunch, item.id)}>
                {t("Relaunch")}
              </button>
            ) : null}
            <button
              type="button"
              aria-label={t("Close application actions")}
              onClick={() => setContextItem("")}
            >
              ×
            </button>
          </div>
        ) : null}
        {isGroup && currentItems && group === item.id ? (
          <ul className="ot-launcher-group-members" aria-label={item.name}>
            {(item.members ?? []).map((entry) => renderItem(entry, true))}
          </ul>
        ) : null}
      </li>
    );
  };
  return (
    <ul
      ref={strip}
      className={`ot-launcher-strip${magnified ? " is-magnified" : ""}`}
      style={
        magnified
          ? ({
              "--ot-mag-primary": `${Math.max(0, m!.primaryInset - 4)}px`,
              "--ot-mag-cross": `${Math.max(0, m!.crossInset - 4)}px`,
            } as React.CSSProperties)
          : undefined
      }
      aria-label={t("Launcher items")}
    >
      {presentation.items.map((item) => renderItem(item))}
    </ul>
  );
}

function kindLabel(kind: string) {
  if (kind === "folder") return "Open folder";
  if (kind === "file") return "Open file";
  if (kind === "link") return "Open link";
  if (kind === "group") return "Application group";
  return "Open application";
}
function referenceStatus(status?: string) {
  if (status === "moved") return "Moved — relink in Settings";
  if (status === "missing") return "Missing — relink in Settings";
  if (status === "accessRequired" || status === "needsSelection") return "Select again in Settings";
  if (status === "changed") return "Changed — relink in Settings";
  if (status === "preparing") return "Loading…";
  return "Item unavailable";
}

function itemIdentity(item: LauncherPresentationItem): unknown {
  return [
    item.id,
    item.kind,
    item.name,
    item.status,
    item.running,
    item.referenceRevision,
    item.members?.map(itemIdentity),
  ];
}

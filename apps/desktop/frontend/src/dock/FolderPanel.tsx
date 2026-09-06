import { useEffect, useRef, useState } from "react";
import type { DockFolderState } from "../lib/types";

export interface FolderPanelHandlers {
  onSort: (
    session: number,
    revision: number,
    field: DockFolderState["sort"]["field"],
    direction: DockFolderState["sort"]["direction"],
    foldersFirst: boolean,
  ) => Promise<void> | void;
  onRequestAccess: (session: number, revision: number) => Promise<void> | void;
  onCancelAccess: (session: number, revision: number) => Promise<void> | void;
  onOpen: (session: number, revision: number, itemID: string) => Promise<void> | void;
}

const statusCopy = (status: DockFolderState["status"]) => {
  switch (status) {
    case "permissionRequired":
      return ["Folder access is required", "Allow access", "access"] as const;
    case "revoked":
      return ["Folder access expired", "Allow access", "access"] as const;
    case "missing":
      return ["Folder is no longer available", "Retry", "refresh"] as const;
    case "unavailable":
      return ["Folder contents are unavailable", "Retry", "refresh"] as const;
    default:
      return null;
  }
};

const formatSize = (bytes: number) => {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};
const kindLabel = (kind: string) =>
  kind === "folder"
    ? "Folder"
    : kind === "symlink"
      ? "Symbolic link"
      : kind === "file"
        ? "File"
        : "Item";

export function FolderPanel({
  session,
  revision,
  folder,
  handlers,
  t = (text) => text,
}: {
  session: number;
  revision: number;
  folder: DockFolderState;
  handlers: FolderPanelHandlers;
  t?: (text: string) => string;
}) {
  type Scope = { session: number; revision: number; identity: string };
  const [pendingAccess, setPendingAccess] = useState<Scope | null>(null);
  const [actionError, setActionError] = useState("");
  const currentScope = useRef<Scope>({ session, revision, identity: folder.folderIdentity });
  currentScope.current = { session, revision, identity: folder.folderIdentity };
  const sameScope = (left: Scope, right: Scope) =>
    left.session === right.session &&
    left.revision === right.revision &&
    left.identity === right.identity;
  useEffect(() => setActionError(""), [session, revision, folder.folderIdentity]);
  useEffect(
    () =>
      setPendingAccess((current) =>
        current && (current.session !== session || current.identity !== folder.folderIdentity)
          ? null
          : current,
      ),
    [session, folder.folderIdentity],
  );
  const run = async (action: () => Promise<void> | void, scope = currentScope.current) => {
    setActionError("");
    try {
      await action();
    } catch (error) {
      if (sameScope(scope, currentScope.current))
        setActionError(error instanceof Error ? error.message : String(error));
    }
  };
  const requestAccess = async () => {
    const scope = { session, revision, identity: folder.folderIdentity };
    setPendingAccess(scope);
    await run(() => handlers.onRequestAccess(scope.session, scope.revision), scope);
    setPendingAccess((current) => (current && sameScope(current, scope) ? null : current));
  };
  const guidance = statusCopy(folder.status);
  return (
    <section className="ot-folder-panel" aria-label={t("Folder contents")}>
      {folder.status === "ready" || folder.status === "partial" ? (
        <div className="ot-folder-toolbar">
          <label>
            <span>{t("Sort by")}</span>
            <select
              aria-label="Sort folder contents by"
              value={folder.sort.field}
              onChange={(event) =>
                void run(() =>
                  handlers.onSort(
                    session,
                    revision,
                    event.target.value as DockFolderState["sort"]["field"],
                    folder.sort.direction,
                    folder.sort.foldersFirst,
                  ),
                )
              }
            >
              <option value="name">{t("Name")}</option>
              <option value="modified">{t("Date modified")}</option>
              <option value="size">{t("Size")}</option>
              <option value="kind">{t("Kind")}</option>
            </select>
          </label>
          <button
            type="button"
            aria-label={t(folder.sort.direction === "asc" ? "Ascending" : "Descending")}
            title={t(folder.sort.direction === "asc" ? "Ascending" : "Descending")}
            onClick={() =>
              void run(() =>
                handlers.onSort(
                  session,
                  revision,
                  folder.sort.field,
                  folder.sort.direction === "asc" ? "desc" : "asc",
                  folder.sort.foldersFirst,
                ),
              )
            }
          >
            {folder.sort.direction === "asc" ? "↑" : "↓"}
          </button>
          <label className="ot-folder-first">
            <input
              type="checkbox"
              aria-label="Folders first"
              checked={folder.sort.foldersFirst}
              onChange={(event) =>
                void run(() =>
                  handlers.onSort(
                    session,
                    revision,
                    folder.sort.field,
                    folder.sort.direction,
                    event.target.checked,
                  ),
                )
              }
            />
            <span>{t("Folders first")}</span>
          </label>
        </div>
      ) : null}
      {actionError ? (
        <p role="alert" className="ot-dock-error">
          {actionError}
        </p>
      ) : null}
      {folder.partial || folder.status === "partial" ? (
        <p className="ot-folder-notice">{t("Some folder items could not be shown.")}</p>
      ) : null}
      {guidance ? (
        <div className="ot-folder-guidance">
          <p>{t(guidance[0])}</p>
          {folder.reason ? <p className="ot-folder-reason">{folder.reason}</p> : null}
          {pendingAccess ? (
            <button
              type="button"
              onClick={() =>
                void handlers.onCancelAccess(pendingAccess.session, pendingAccess.revision)
              }
            >
              {t("Cancel access request")}
            </button>
          ) : (
            <button
              type="button"
              onClick={() =>
                void (guidance[2] === "access"
                  ? requestAccess()
                  : run(() =>
                      handlers.onSort(
                        session,
                        revision,
                        folder.sort.field,
                        folder.sort.direction,
                        folder.sort.foldersFirst,
                      ),
                    ))
              }
            >
              {t(guidance[1])}
            </button>
          )}
        </div>
      ) : folder.status === "loading" ? (
        <p className="ot-dock-empty" role="status">
          {t("Loading folder…")}
        </p>
      ) : folder.entries.length ? (
        <ul className="ot-folder-list">
          {folder.entries.map((entry) => (
            <li key={entry.id}>
              <button
                type="button"
                className={`ot-folder-entry${entry.hidden ? " is-hidden" : ""}`}
                aria-label={`${t("Open")} ${entry.name}`}
                onClick={() => void run(() => handlers.onOpen(session, revision, entry.id))}
              >
                <span className="ot-folder-icon" aria-hidden="true">
                  {entry.kind === "folder" ? "▸" : "·"}
                </span>
                <span className="ot-folder-name" title={entry.name}>
                  {entry.name}
                </span>
                <span className="ot-folder-kind">{t(kindLabel(entry.kind))}</span>
                <span className="ot-folder-size">
                  {entry.kind === "folder" ? "—" : formatSize(entry.size)}
                </span>
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="ot-dock-empty">{t("This folder is empty")}</p>
      )}
    </section>
  );
}

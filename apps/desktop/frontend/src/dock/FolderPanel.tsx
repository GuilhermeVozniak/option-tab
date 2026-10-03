import { useEffect, useRef, useState } from "react";
import {
  type ActionErrorReason,
  formatActionError,
  normalizeActionError,
} from "../lib/action-feedback";
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

const folderReasonKey = (reason: unknown): string | undefined => {
  const message = String(reason).toLowerCase();
  if (message.includes("folder access cancelled") || message.includes("context canceled"))
    return "Folder access was cancelled.";
  if (message.includes("selected folder does not match"))
    return "Choose the same folder shown in the Dock.";
  if (message.includes("folder access request is already active"))
    return "Another folder access request is in progress.";
  if (message.includes("bookmark") && /stale|revoked|refresh failed/.test(message))
    return "Folder access expired";
  if (/folder.*(replaced|identity changed|no longer canonical)/.test(message))
    return "Folder is no longer available";
  return undefined;
};

const folderActionReason = (error: unknown, fallback: string): ActionErrorReason => {
  const key = folderReasonKey(error);
  if (key) return { key };
  const reason = normalizeActionError(error);
  return reason.key === "The window action could not be completed. Try again."
    ? { key: fallback }
    : reason;
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
  view = "list",
  accessLabel,
  t = (text) => text,
}: {
  session: number;
  revision: number;
  folder: DockFolderState;
  handlers: FolderPanelHandlers;
  view?: "list" | "grid";
  accessLabel?: string;
  t?: (text: string) => string;
}) {
  type Scope = { session: number; revision: number; identity: string };
  const [pendingAccess, setPendingAccess] = useState<Scope | null>(null);
  const [actionError, setActionError] = useState<ActionErrorReason | null>(null);
  const currentScope = useRef<Scope>({ session, revision, identity: folder.folderIdentity });
  currentScope.current = { session, revision, identity: folder.folderIdentity };
  const sameScope = (left: Scope, right: Scope) =>
    left.session === right.session &&
    left.revision === right.revision &&
    left.identity === right.identity;
  useEffect(() => setActionError(null), [session, revision, folder.folderIdentity]);
  useEffect(
    () =>
      setPendingAccess((current) =>
        current && (current.session !== session || current.identity !== folder.folderIdentity)
          ? null
          : current,
      ),
    [session, folder.folderIdentity],
  );
  const run = async (
    action: () => Promise<void> | void,
    fallback = "The folder action could not be completed. Try again.",
    scope = currentScope.current,
  ) => {
    setActionError(null);
    try {
      await action();
    } catch (error) {
      if (sameScope(scope, currentScope.current))
        setActionError(folderActionReason(error, fallback));
    }
  };
  const requestAccess = async () => {
    const scope = { session, revision, identity: folder.folderIdentity };
    setPendingAccess(scope);
    await run(
      () => handlers.onRequestAccess(scope.session, scope.revision),
      "Folder access is required",
      scope,
    );
    setPendingAccess((current) => (current && sameScope(current, scope) ? null : current));
  };
  const guidance = statusCopy(folder.status);
  // The status supplies localized fallback guidance for unknown native diagnostics.
  const reasonKey = folderReasonKey(folder.reason);
  return (
    <section className="ot-folder-panel" aria-label={t("Folder contents")}>
      {folder.status === "ready" || folder.status === "partial" ? (
        <div className="ot-folder-toolbar">
          <label>
            <span>{t("Sort by")}</span>
            <select
              aria-label={t("Sort folder contents by")}
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
              aria-label={t("Folders first")}
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
          {formatActionError(actionError, t)}
        </p>
      ) : null}
      {folder.partial || folder.status === "partial" ? (
        <p className="ot-folder-notice">{t("Some folder items could not be shown.")}</p>
      ) : null}
      {guidance ? (
        <div className="ot-folder-guidance">
          <p>{t(guidance[0])}</p>
          {reasonKey && reasonKey !== guidance[0] ? (
            <p className="ot-folder-reason">{t(reasonKey)}</p>
          ) : null}
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
              {t(guidance[2] === "access" && accessLabel ? accessLabel : guidance[1])}
            </button>
          )}
        </div>
      ) : folder.status === "loading" ? (
        <p className="ot-dock-empty" role="status">
          {t("Loading folder…")}
        </p>
      ) : folder.entries.length ? (
        <ul className={`ot-folder-list${view === "grid" ? " is-grid" : ""}`}>
          {folder.entries.map((entry) => (
            <li key={entry.id}>
              <button
                type="button"
                className={`ot-folder-entry${entry.hidden ? " is-hidden" : ""}`}
                aria-label={`${t("Open")} ${entry.name}`}
                onClick={() =>
                  void run(
                    () => handlers.onOpen(session, revision, entry.id),
                    "The item could not be opened.",
                  )
                }
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

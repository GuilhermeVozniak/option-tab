import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { Translate } from "../lib/i18n";
import { launcherItemSettings } from "../lib/launcher-items-bridge";
import type {
  LauncherItem,
  LauncherItemIcon,
  LauncherItemSettings,
  LauncherItemStatus,
  LauncherReferenceView,
} from "../lib/types";
import "./editor-settings.css";

export interface LauncherItemSettingsActions {
  load(profileID: string): Promise<LauncherItemSettings>;
  save(profileID: string, revision: string, items: LauncherItem[]): Promise<LauncherItemSettings>;
  chooseReference(kind: "app" | "folder" | "file"): Promise<LauncherReferenceView>;
  relinkReference(id: string): Promise<LauncherReferenceView>;
  cancelSelection(): Promise<unknown> | unknown;
  chooseIcon(): Promise<LauncherItemIcon>;
  removeReference(id: string): Promise<unknown> | unknown;
  removeIcon(id: string): Promise<unknown> | unknown;
  getIcon?(id: string): Promise<LauncherItemIcon>;
  status?(): Promise<LauncherItemStatus>;
  subscribe?(handler: (status: LauncherItemStatus) => void): () => void;
}
const actionable = (x: LauncherItem) => !["spacer", "separator"].includes(x.kind);
function itemError(message: string, loading = false): string {
  const code = message.replace(/^launcher items?:\s*/, "");
  if (code === "busy") return "The launcher is busy. Try again.";
  if (code === "retired" || code === "staleRevision") return "The launcher changed. Try again.";
  if (code === "profileMissing") return "Save the profile before editing its items.";
  if (code === "invalidIcon") return "Choose a valid PNG icon.";
  if (code === "tooLarge") return "The selected file is too large.";
  if (code === "catalogFull") return "The local item library is full.";
  if (code === "cleanupFailed")
    return "Items were saved, but unused local data could not be removed.";
  if (code === "referenced") return "This local item is still in use.";
  if (code === "invalidKind" || code === "invalidResult" || code === "invalidArgument")
    return "The selected item is invalid. Select it again.";
  if (
    [
      "unknownReference",
      "unknownResource",
      "unknownIcon",
      "accessRequired",
      "needsSelection",
    ].includes(code)
  )
    return "Select again in Settings";
  if (["changed", "moved", "missing"].includes(code)) return referenceStatus(code);
  if (code === "cancelled" || code === "context canceled") return "The item change was cancelled.";
  if (loading || code === "unavailable") return "Launcher item settings are unavailable.";
  return "The launcher item change could not be completed. Try again.";
}
function referenceStatus(state: string): string {
  if (state === "moved") return "Moved — relink in Settings";
  if (state === "missing") return "Missing — relink in Settings";
  if (state === "accessRequired" || state === "needsSelection") return "Select again in Settings";
  if (state === "changed") return "Changed — relink in Settings";
  if (state === "preparing") return "Loading…";
  return "Item unavailable";
}
function newID(kind: string) {
  return `${kind}-${crypto.randomUUID().replaceAll("-", "").slice(0, 24)}`;
}
export function LauncherItems({
  profileID,
  actions = launcherItemSettings,
  status: initialStatus,
  refreshKey,
  t,
  onSaved,
}: {
  profileID: string;
  actions?: LauncherItemSettingsActions;
  status?: LauncherItemStatus;
  refreshKey?: number;
  t: Translate;
  onSaved?: (items: LauncherItem[]) => void;
}) {
  const [status, setStatus] = useState(initialStatus);
  const [snapshot, setSnapshot] = useState<LauncherItemSettings | null>(null),
    [items, setItems] = useState<LauncherItem[]>([]),
    [error, setError] = useState(""),
    [working, setWorking] = useState(false),
    [linkLabel, setLinkLabel] = useState(""),
    [linkURL, setLinkURL] = useState(""),
    [groupName, setGroupName] = useState(""),
    [members, setMembers] = useState<string[]>([]),
    [icons, setIcons] = useState<Record<string, LauncherItemIcon>>({});
  const loadEpoch = useRef(0);
  const loadRequest = useRef(0);
  const operationEpoch = useRef(0);
  const transientIcons = useRef(new Set<string>());
  const snapshotLoaded = useRef(false);
  const wasBusy = useRef(initialStatus?.busy ?? false);
  useEffect(() => {
    if (!actions.status || !actions.subscribe) return;
    let active = true;
    const off = actions.subscribe((next) => setStatus(next));
    void actions
      .status()
      .then((next) => {
        if (active) setStatus(next);
      })
      .catch(() => {});
    return () => {
      active = false;
      off();
    };
  }, [actions]);
  useEffect(() => {
    loadEpoch.current++;
    operationEpoch.current++;
    snapshotLoaded.current = false;
    setSnapshot(null);
    setItems([]);
    setError("");
    transientIcons.current.clear();
    return () => {
      loadEpoch.current++;
      operationEpoch.current++;
    };
  }, [profileID, actions]);
  useEffect(() => {
    const completed = wasBusy.current && status?.busy === false;
    wasBusy.current = status?.busy ?? false;
    // The hidden settings webview can mount before preferences admit reads.
    // Retry its first snapshot when admission or a saved profile returns,
    // without resetting a draft.
    if (status?.available === false || (snapshotLoaded.current && !completed)) return;
    const epoch = loadEpoch.current;
    const request = ++loadRequest.current;
    void actions
      .load(profileID)
      .then((next) => {
        if (request !== loadRequest.current || epoch !== loadEpoch.current) return;
        if (!snapshotLoaded.current) {
          snapshotLoaded.current = true;
          setSnapshot(next);
          setItems(next.items ?? []);
          setError("");
          for (const id of new Set(
            (next.items ?? []).flatMap((item) => (item.iconID ? [item.iconID] : [])),
          )) {
            void actions
              .getIcon?.(id)
              .then((icon) => {
                if (epoch === loadEpoch.current) setIcons((old) => ({ ...old, [icon.id]: icon }));
              })
              .catch(() => {});
          }
        } else {
          // A completed chooser refreshes private metadata, retaining the draft
          // and its base revision until the user explicitly saves it.
          setSnapshot((old) =>
            old ? { ...old, references: next.references, iconIDs: next.iconIDs } : next,
          );
        }
      })
      .catch((e) => {
        if (
          request === loadRequest.current &&
          epoch === loadEpoch.current &&
          !snapshotLoaded.current
        )
          setError(itemError(String(e instanceof Error ? e.message : e), true));
      });
  }, [actions, profileID, status?.available, status?.busy, refreshKey]);
  const refs = useMemo(
    () => new Map((snapshot?.references ?? []).map((r) => [r.id, r])),
    [snapshot],
  );
  const usedRefs = new Set(items.flatMap((x) => (x.referenceID ? [x.referenceID] : [])));
  const run = async <T,>(
    fn: () => Promise<T>,
    done: (v: T) => void,
    kind: "selection" | "update" = "update",
  ) => {
    const epoch = operationEpoch.current;
    setWorking(true);
    setError("");
    try {
      const value = await fn();
      if (epoch === operationEpoch.current) done(value);
    } catch (e) {
      const message = String(e instanceof Error ? e.message : e);
      // Native chooser Cancel is returned through Wails as Go context.Canceled.
      // It leaves the draft unchanged; cancelled writes still need an error.
      if (
        epoch === operationEpoch.current &&
        !(kind === "selection" && message === "context canceled")
      )
        setError(itemError(message));
    } finally {
      if (epoch === operationEpoch.current) setWorking(false);
    }
  };
  const full = items.length >= 16;
  const blocked = !snapshot || working || status?.busy || status?.available === false || full;
  const removeItem = (id: string) =>
    setItems((old) =>
      old
        .filter((item) => item.id !== id)
        .map((item) =>
          item.kind === "group"
            ? { ...item, members: item.members?.filter((member) => member !== id) }
            : item,
        )
        .filter((item) => item.kind !== "group" || (item.members?.length ?? 0) > 0),
    );
  const addRef = (kind: "app" | "folder" | "file") =>
    void run(
      () => actions.chooseReference(kind),
      (r) => {
        setSnapshot((old) =>
          old ? { ...old, references: [...old.references.filter((x) => x.id !== r.id), r] } : old,
        );
        setItems((old) => [
          ...old,
          {
            id: newID(kind),
            kind,
            label: r.label,
            referenceID: r.id,
            ...(kind === "folder" ? { folderView: "list" as const } : {}),
          },
        ]);
      },
      "selection",
    );
  const move = (index: number, by: number) => {
    const next = [...items],
      to = index + by;
    if (to < 0 || to >= next.length) return;
    [next[index], next[to]] = [next[to], next[index]];
    setItems(next);
  };
  const dropBefore = (sourceID: string, targetID: string) => {
    if (sourceID === targetID) return;
    setItems((old) => {
      const from = old.findIndex((item) => item.id === sourceID);
      const target = old.findIndex((item) => item.id === targetID);
      if (from < 0 || target < 0) return old;
      const next = [...old];
      const [item] = next.splice(from, 1);
      next.splice(from < target ? target - 1 : target, 0, item);
      return next;
    });
  };
  return (
    <section
      className="ot-settings-editor ot-launcher-items-editor"
      aria-label={t("Launcher items")}
    >
      <header className="ot-editor-heading">
        <h3>{t("Launcher items")}</h3>
        <p>{t("Pins and groups are stored per profile. Choosing a file never opens it.")}</p>
      </header>
      {error ? <p role="alert">{t(error)}</p> : null}
      {status?.available === false ? <p>{t("Launcher item selection is unavailable.")}</p> : null}
      <div className="ot-editor-actions">
        <Button type="button" onClick={() => addRef("app")} disabled={blocked}>
          {t("Add application")}
        </Button>
        <Button type="button" onClick={() => addRef("folder")} disabled={blocked}>
          {t("Add folder")}
        </Button>
        <Button type="button" onClick={() => addRef("file")} disabled={blocked}>
          {t("Add file")}
        </Button>
        <Button
          type="button"
          disabled={!snapshot || full}
          onClick={() =>
            setItems((x) => [...x, { id: newID("spacer"), kind: "spacer", label: "" }])
          }
        >
          {t("Add spacer")}
        </Button>
        <Button
          type="button"
          disabled={!snapshot || full}
          onClick={() =>
            setItems((x) => [...x, { id: newID("separator"), kind: "separator", label: "" }])
          }
        >
          {t("Add separator")}
        </Button>
        {status?.busy ? (
          <Button type="button" onClick={() => actions.cancelSelection()}>
            {t("Cancel selection")}
          </Button>
        ) : null}
      </div>
      <div className="ot-editor-draft ot-editor-link-draft">
        <label className="ot-editor-field">
          <span>{t("Link label")}</span>
          <Input
            aria-label={t("Link label")}
            value={linkLabel}
            maxLength={80}
            onChange={(e) => setLinkLabel(e.target.value)}
          />
        </label>
        <label className="ot-editor-field">
          <span>{t("Web address")}</span>
          <Input
            aria-label={t("Web address")}
            value={linkURL}
            onChange={(e) => setLinkURL(e.target.value)}
          />
        </label>
        <Button
          type="button"
          disabled={!snapshot || !linkLabel || !linkURL || full}
          onClick={() => {
            setItems((x) => [
              ...x,
              { id: newID("link"), kind: "link", label: linkLabel, url: linkURL },
            ]);
            setLinkLabel("");
            setLinkURL("");
          }}
        >
          {t("Add link")}
        </Button>
      </div>
      {!snapshot ? (
        <p className="ot-editor-empty">{t("Loading launcher items…")}</p>
      ) : items.length === 0 ? (
        <p className="ot-editor-empty">{t("No launcher items yet.")}</p>
      ) : (
        <div className="ot-editor-item-list">
          {items.map((x, index) => {
            const ref = x.referenceID ? refs.get(x.referenceID) : undefined,
              icon = x.iconID ? icons[x.iconID] : undefined;
            return (
              <article
                data-testid="launcher-item"
                key={x.id}
                className="ot-editor-item"
                draggable
                onDragStart={(event) => {
                  event.dataTransfer.effectAllowed = "move";
                  event.dataTransfer.setData("application/x-optiontab-launcher-item", x.id);
                }}
                onDragOver={(event) => {
                  if (event.dataTransfer.types.includes("application/x-optiontab-launcher-item"))
                    event.preventDefault();
                }}
                onDrop={(event) => {
                  const source = event.dataTransfer.getData(
                    "application/x-optiontab-launcher-item",
                  );
                  if (source) {
                    event.preventDefault();
                    dropBefore(source, x.id);
                  }
                }}
              >
                <div className="ot-editor-item-heading">
                  {icon ? (
                    <img src={icon.dataURL} alt={t("Custom icon")} width={32} height={32} />
                  ) : null}
                  <strong>{x.label || t(x.kind)}</strong>
                  <span className="ot-editor-kind">{t(x.kind)}</span>
                  {ref && ref.state !== "ready" ? (
                    <span className="ot-editor-item-status">{t(referenceStatus(ref.state))}</span>
                  ) : null}
                </div>
                <div className="ot-editor-actions">
                  <Button
                    size="icon"
                    aria-label={t("Move up")}
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                  >
                    ↑
                  </Button>
                  <Button
                    size="icon"
                    aria-label={t("Move down")}
                    disabled={index === items.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    ↓
                  </Button>
                  <Button variant="ghost" onClick={() => removeItem(x.id)}>
                    {t("Remove item")}
                  </Button>
                  {x.referenceID && (!ref || ref.state !== "ready") ? (
                    <Button
                      onClick={() =>
                        void (ref?.state === "needsSelection" || !ref
                          ? run(
                              () => actions.chooseReference(x.kind as "app" | "folder" | "file"),
                              (r) => {
                                setSnapshot((old) =>
                                  old
                                    ? {
                                        ...old,
                                        references: [
                                          ...old.references.filter((value) => value.id !== r.id),
                                          r,
                                        ],
                                      }
                                    : old,
                                );
                                setItems((old) =>
                                  old.map((item) =>
                                    item.id === x.id
                                      ? { ...item, referenceID: r.id, label: r.label }
                                      : item,
                                  ),
                                );
                              },
                              "selection",
                            )
                          : run(
                              () => actions.relinkReference(ref.id),
                              (r) =>
                                setSnapshot((old) =>
                                  old
                                    ? {
                                        ...old,
                                        references: old.references.map((v) =>
                                          v.id === r.id ? r : v,
                                        ),
                                      }
                                    : old,
                                ),
                              "selection",
                            ))
                      }
                    >
                      {ref?.state === "needsSelection" || !ref ? t("Select again") : t("Relink")}
                    </Button>
                  ) : null}
                  {actionable(x) ? (
                    x.iconID ? (
                      <Button
                        onClick={() => {
                          const id = x.iconID!;
                          setItems((old) =>
                            old.map((i) => (i.id === x.id ? { ...i, iconID: undefined } : i)),
                          );
                          if (transientIcons.current.delete(id)) {
                            void Promise.resolve(actions.removeIcon(id)).catch(() => {});
                          }
                        }}
                      >
                        {t("Remove custom icon")}
                      </Button>
                    ) : (
                      <Button
                        onClick={() =>
                          void run(
                            () => actions.chooseIcon(),
                            (v) => {
                              transientIcons.current.add(v.id);
                              setIcons((old) => ({ ...old, [v.id]: v }));
                              setItems((old) =>
                                old.map((i) => (i.id === x.id ? { ...i, iconID: v.id } : i)),
                              );
                            },
                            "selection",
                          )
                        }
                      >
                        {t("Choose custom icon")}
                      </Button>
                    )
                  ) : null}
                  {x.kind === "folder" ? (
                    <Select
                      aria-label={t("Folder view")}
                      value={x.folderView}
                      onChange={(e) =>
                        setItems((old) =>
                          old.map((i) =>
                            i.id === x.id
                              ? { ...i, folderView: e.target.value as "list" | "grid" }
                              : i,
                          ),
                        )
                      }
                    >
                      <option value="list">{t("List")}</option>
                      <option value="grid">{t("Grid")}</option>
                    </Select>
                  ) : null}
                </div>
              </article>
            );
          })}
        </div>
      )}
      <fieldset className="ot-editor-draft ot-editor-group-draft">
        <legend>{t("New application group")}</legend>
        <label className="ot-editor-field">
          <span>{t("Group name")}</span>
          <Input
            aria-label={t("Group name")}
            value={groupName}
            maxLength={80}
            onChange={(e) => setGroupName(e.target.value)}
          />
        </label>
        {items
          .filter(
            (x) =>
              x.kind === "app" &&
              !items.some((g) => g.kind === "group" && g.members?.includes(x.id)),
          )
          .map((x) => (
            <label key={x.id} className="ot-editor-check-row">
              <Checkbox
                aria-label={x.label}
                checked={members.includes(x.id)}
                onChange={(e) =>
                  setMembers((old) =>
                    e.target.checked ? [...old, x.id] : old.filter((id) => id !== x.id),
                  )
                }
              />
              {x.label}
            </label>
          ))}
        <Button
          disabled={!groupName || members.length === 0 || full}
          onClick={() => {
            setItems((old) => [
              ...old,
              { id: newID("group"), kind: "group", label: groupName, members },
            ]);
            setGroupName("");
            setMembers([]);
          }}
        >
          {t("Create group")}
        </Button>
      </fieldset>
      {(snapshot?.references ?? [])
        .filter((r) => !usedRefs.has(r.id))
        .map((r) => (
          <div key={r.id} className="ot-editor-cleanup-row">
            <span>{r.label}</span>
            <Button
              onClick={() =>
                void run(
                  () => Promise.resolve(actions.removeReference(r.id)),
                  () =>
                    setSnapshot((old) =>
                      old
                        ? {
                            ...old,
                            references: old.references.filter((value) => value.id !== r.id),
                          }
                        : old,
                    ),
                )
              }
            >
              {t("Remove unused reference")}
            </Button>
          </div>
        ))}
      {(snapshot?.iconIDs ?? [])
        .filter((id) => !items.some((item) => item.iconID === id))
        .map((id) => (
          <div key={id} className="ot-editor-cleanup-row">
            <span>{t("Unused custom icon")}</span>
            <Button
              onClick={() =>
                void run(
                  () => Promise.resolve(actions.removeIcon(id)),
                  () =>
                    setSnapshot((old) =>
                      old ? { ...old, iconIDs: old.iconIDs?.filter((value) => value !== id) } : old,
                    ),
                )
              }
            >
              {t("Remove unused icon")}
            </Button>
          </div>
        ))}
      <Button
        variant="default"
        className="ot-editor-save"
        disabled={!snapshot || working}
        onClick={() =>
          snapshot &&
          void run(
            () => actions.save(profileID, snapshot.revision, items),
            (next) => {
              setSnapshot(next);
              setItems(next.items);
              onSaved?.(next.items);
            },
          )
        }
      >
        {t("Save launcher items")}
      </Button>
    </section>
  );
}

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
function newID(kind: string) {
  return `${kind}-${crypto.randomUUID().replaceAll("-", "").slice(0, 24)}`;
}
export function LauncherItems({
  profileID,
  actions = launcherItemSettings,
  status: initialStatus,
  t,
  onSaved,
}: {
  profileID: string;
  actions?: LauncherItemSettingsActions;
  status?: LauncherItemStatus;
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
  const operationEpoch = useRef(0);
  const transientIcons = useRef(new Set<string>());
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
    const epoch = ++loadEpoch.current;
    operationEpoch.current++;
    setSnapshot(null);
    setItems([]);
    setError("");
    transientIcons.current.clear();
    void actions
      .load(profileID)
      .then((next) => {
        if (epoch === loadEpoch.current) {
          setSnapshot(next);
          setItems(next.items ?? []);
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
        }
      })
      .catch((e) => {
        if (epoch === loadEpoch.current) setError(String(e instanceof Error ? e.message : e));
      });
    return () => {
      loadEpoch.current++;
      operationEpoch.current++;
    };
  }, [profileID, actions]);
  useEffect(() => {
    const completed = wasBusy.current && status?.busy === false;
    wasBusy.current = status?.busy ?? false;
    if (!completed) return;
    const epoch = loadEpoch.current;
    void actions
      .load(profileID)
      .then((next) => {
        if (epoch !== loadEpoch.current) return;
        // A background chooser may change private references and icons. Refresh
        // that metadata without replacing the user's unsaved item draft.
        setSnapshot((old) =>
          old ? { ...old, references: next.references, iconIDs: next.iconIDs } : next,
        );
      })
      .catch(() => {});
  }, [actions, profileID, status?.busy]);
  const refs = useMemo(
    () => new Map((snapshot?.references ?? []).map((r) => [r.id, r])),
    [snapshot],
  );
  const usedRefs = new Set(items.flatMap((x) => (x.referenceID ? [x.referenceID] : [])));
  const run = async <T,>(fn: () => Promise<T>, done: (v: T) => void) => {
    const epoch = operationEpoch.current;
    setWorking(true);
    setError("");
    try {
      const value = await fn();
      if (epoch === operationEpoch.current) done(value);
    } catch (e) {
      if (epoch === operationEpoch.current) setError(String(e instanceof Error ? e.message : e));
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
    <section className="space-y-2" aria-label={t("Launcher items")}>
      <h3>{t("Launcher items")}</h3>
      <p>{t("Pins and groups are stored per profile. Choosing a file never opens it.")}</p>
      {error ? <p role="alert">{error}</p> : null}
      {status?.available === false ? <p>{t("Launcher item selection is unavailable.")}</p> : null}
      <div className="flex flex-wrap gap-2">
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
          disabled={full}
          onClick={() =>
            setItems((x) => [...x, { id: newID("spacer"), kind: "spacer", label: "" }])
          }
        >
          {t("Add spacer")}
        </Button>
        <Button
          type="button"
          disabled={full}
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
      <div className="grid grid-cols-2 gap-2">
        <Input
          aria-label={t("Link label")}
          value={linkLabel}
          maxLength={80}
          onChange={(e) => setLinkLabel(e.target.value)}
        />
        <Input
          aria-label={t("Web address")}
          value={linkURL}
          onChange={(e) => setLinkURL(e.target.value)}
        />
        <Button
          type="button"
          disabled={!linkLabel || !linkURL || full}
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
        <p>{t("Loading launcher items…")}</p>
      ) : items.length === 0 ? (
        <p>{t("No launcher items yet.")}</p>
      ) : (
        <div>
          {items.map((x, index) => {
            const ref = x.referenceID ? refs.get(x.referenceID) : undefined,
              icon = x.iconID ? icons[x.iconID] : undefined;
            return (
              <article
                data-testid="launcher-item"
                key={x.id}
                className="rounded-md border p-2"
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
                <div className="flex items-center gap-2">
                  {icon ? (
                    <img src={icon.dataURL} alt={t("Custom icon")} width={32} height={32} />
                  ) : null}
                  <strong>{x.label || t(x.kind)}</strong>
                  <span>{t(x.kind)}</span>
                  {ref && ref.state !== "ready" ? <span>{t(ref.state)}</span> : null}
                </div>
                <div className="flex flex-wrap gap-1">
                  <Button
                    aria-label={t("Move up")}
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                  >
                    ↑
                  </Button>
                  <Button
                    aria-label={t("Move down")}
                    disabled={index === items.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    ↓
                  </Button>
                  <Button onClick={() => removeItem(x.id)}>{t("Remove item")}</Button>
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
      <fieldset>
        <legend>{t("New application group")}</legend>
        <Input
          aria-label={t("Group name")}
          value={groupName}
          maxLength={80}
          onChange={(e) => setGroupName(e.target.value)}
        />
        {items
          .filter(
            (x) =>
              x.kind === "app" &&
              !items.some((g) => g.kind === "group" && g.members?.includes(x.id)),
          )
          .map((x) => (
            <label key={x.id}>
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
          <div key={r.id}>
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
          <div key={id}>
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

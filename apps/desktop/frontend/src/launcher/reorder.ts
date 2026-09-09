import type { LauncherPresentationItem } from "../lib/types";
export interface LauncherItemMutation {
  kind: "moveBefore" | "moveAfter" | "addToGroup" | "removeFromGroup";
  itemID: string;
  targetID: string;
}
function records(items: LauncherPresentationItem[]) {
  return items.flatMap((item) => [
    { item, owner: "" },
    ...(item.members ?? []).map((member) => ({ item: member, owner: item.id })),
  ]);
}
export function reorderable(item: LauncherPresentationItem) {
  return (
    item.id.startsWith("pin:") &&
    ["app", "file", "folder", "link", "group"].includes(item.kind ?? "")
  );
}
export function reorderMutation(
  items: LauncherPresentationItem[],
  source: string,
  target: string,
  zone: "before" | "after" | "group",
): LauncherItemMutation | null {
  const all = records(items),
    from = all.find((x) => x.item.id === source),
    to = all.find((x) => x.item.id === target);
  if (
    !from ||
    !to ||
    !reorderable(from.item) ||
    !to.item.id.startsWith("pin:") ||
    source === target
  )
    return null;
  if (zone === "group") {
    if (
      from.item.kind !== "app" ||
      to.owner !== "" ||
      !["app", "group"].includes(to.item.kind ?? "") ||
      from.owner === target
    )
      return null;
    return { kind: "addToGroup", itemID: source, targetID: target };
  }
  return from.owner === to.owner
    ? { kind: zone === "before" ? "moveBefore" : "moveAfter", itemID: source, targetID: target }
    : null;
}
export function reorderChoices(items: LauncherPresentationItem[], source: string) {
  const all = records(items),
    from = all.find((x) => x.item.id === source);
  const result: Array<{ label: string; targetName: string; mutation: LauncherItemMutation }> = [];
  if (!from || !reorderable(from.item)) return result;
  const peers = all.filter((x) => x.owner === from.owner && x.item.id.startsWith("pin:")),
    index = peers.findIndex((x) => x.item.id === source);
  for (const [offset, zone, label] of [
    [-1, "before", "Move earlier"],
    [1, "after", "Move later"],
  ] as const) {
    const target = peers[index + offset];
    if (target) {
      const mutation = reorderMutation(items, source, target.item.id, zone);
      if (mutation) result.push({ label, targetName: "", mutation });
    }
  }
  for (const target of items) {
    const mutation = reorderMutation(items, source, target.id, "group");
    if (mutation)
      result.push({
        label: target.kind === "group" ? "Add to group" : "Group with",
        targetName: target.name,
        mutation,
      });
  }
  if (from.owner)
    result.push({
      label: "Remove from group",
      targetName: "",
      mutation: { kind: "removeFromGroup", itemID: source, targetID: from.owner },
    });
  return result;
}

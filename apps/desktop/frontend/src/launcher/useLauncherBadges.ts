import { useEffect, useRef, useState } from "react";
import type {
  LauncherBadgeEntry,
  LauncherBadgeState,
  LauncherBadgeTransport,
} from "../lib/launcher-badge-types";
import type { LauncherPresentation, LauncherPresentationItem } from "../lib/types";

const empty: ReadonlyMap<string, LauncherBadgeEntry> = new Map();

function itemKey(item: LauncherPresentationItem): unknown {
  return [
    item.id,
    item.kind,
    item.status,
    item.running,
    item.referenceRevision,
    item.members?.map(itemKey),
  ];
}

function parentKey(parent: LauncherPresentation | null): string {
  return parent?.visible
    ? JSON.stringify([
        parent.epoch,
        parent.displayUUID,
        parent.session,
        parent.profileID,
        parent.items.map(itemKey),
      ])
    : "";
}

function positive(value: number): boolean {
  return Number.isSafeInteger(value) && value > 0;
}

// Clock-only parent revisions preserve observations; item/reference changes
// start a fresh admission floor before another backend value may be displayed.
export function useLauncherBadges(
  parent: LauncherPresentation | null,
  transport?: LauncherBadgeTransport,
): ReadonlyMap<string, LauncherBadgeEntry> {
  const key = parentKey(parent);
  const parentRef = useRef(parent);
  parentRef.current = parent;
  const high = useRef({ key: "", floor: 0, owner: 0, sequence: 0, retired: false });
  const [cached, setCached] = useState<{ key: string; state: LauncherBadgeState } | null>(null);
  useEffect(() => {
    if (!key || !transport || !parentRef.current) return;
    if (high.current.key !== key) {
      high.current = {
        key,
        floor: parentRef.current.revision,
        owner: 0,
        sequence: 0,
        retired: false,
      };
    }
    let active = true;
    const accept = (next: LauncherBadgeState) => {
      const current = parentRef.current;
      const mark = high.current;
      if (
        !active ||
        !current ||
        parentKey(current) !== key ||
        mark.key !== key ||
        !next ||
        next.epoch !== current.epoch ||
        next.displayUUID !== current.displayUUID ||
        next.session !== current.session ||
        !positive(next.presentationRevision) ||
        next.presentationRevision < mark.floor ||
        !positive(next.owner) ||
        !positive(next.sequence) ||
        next.owner < mark.owner ||
        (next.owner === mark.owner &&
          (next.sequence <= mark.sequence || (mark.retired && next.visible))) ||
        typeof next.visible !== "boolean" ||
        !Array.isArray(next.entries) ||
        next.entries.length > 144
      )
        return;
      mark.owner = next.owner;
      mark.sequence = next.sequence;
      mark.retired = !next.visible;
      setCached({ key, state: { ...next, entries: next.entries.map((entry) => ({ ...entry })) } });
    };
    const off = transport.subscribe(accept);
    void transport
      .get(parentRef.current.session)
      .then(accept)
      .catch(() => {
        // No source means no badge; retain no invented zero or fallback count.
      });
    return () => {
      active = false;
      off();
    };
  }, [key, transport]);

  if (
    !parent?.visible ||
    !key ||
    cached?.key !== key ||
    !cached.state.visible ||
    cached.state.presentationRevision > parent.revision
  )
    return empty;
  const appIDs = new Set<string>();
  const collect = (items: LauncherPresentationItem[]) => {
    for (const item of items) {
      if (item.kind === "group") collect(item.members ?? []);
      else if ((!item.kind || item.kind === "app") && (!item.status || item.status === "ready"))
        appIDs.add(item.id);
    }
  };
  collect(parent.items);
  const result = new Map<string, LauncherBadgeEntry>();
  const seen = new Set<string>();
  for (const entry of cached.state.entries) {
    if (!entry || !appIDs.has(entry.itemID)) continue;
    if (seen.has(entry.itemID)) {
      result.delete(entry.itemID);
      continue;
    }
    seen.add(entry.itemID);
    if (entry.state !== "known") continue;
    if (
      entry.kind === "indicator" ||
      (entry.kind === "count" &&
        Number.isSafeInteger(entry.count) &&
        entry.count! >= 0 &&
        entry.count! <= 999999)
    ) {
      result.set(entry.itemID, entry);
    }
  }
  return result;
}

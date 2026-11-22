// Space badges number the Spaces 1..N by sorted id among the listed windows:
// raw CGS Space ids are not user-meaningful. When every listed window is on
// the same Space the number says nothing, so no badge is shown at all.
export function spaceBadgeNumbers(entries: readonly { spaceId?: number }[]): Map<number, number> {
  const ids = [
    ...new Set(entries.map((entry) => entry.spaceId).filter((id): id is number => !!id)),
  ].sort((a, b) => a - b);
  return ids.length > 1 ? new Map(ids.map((id, index) => [id, index + 1])) : new Map();
}

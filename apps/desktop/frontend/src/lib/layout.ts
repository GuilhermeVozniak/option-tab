// Pure layout math for the switcher grid: how many columns/rows to use and how
// large thumbnails should be, including AltTab-style auto-sizing where the
// thumbnail shrinks as the number of windows grows.

export interface LayoutInput {
  count: number;
  maxColumns: number;
  maxRows: number;
  thumbnailMaxPx: number;
  autoSize: boolean;
}

export interface Layout {
  columns: number;
  rows: number;
  thumbnailPx: number;
}

const MIN_THUMBNAIL_PX = 96;

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

// computeLayout derives grid dimensions and thumbnail size for a window count.
export function computeLayout(input: LayoutInput): Layout {
  const { count, maxColumns, thumbnailMaxPx, autoSize } = input;
  const columns = Math.max(1, Math.min(maxColumns, count || 1));
  const rows = count <= 0 ? 0 : Math.ceil(count / columns);

  let thumbnailPx = thumbnailMaxPx;
  if (autoSize && count > maxColumns) {
    // Scale down proportionally once windows exceed one row's worth, with a
    // readable floor so thumbnails never become unusably small.
    const scaled = Math.round((thumbnailMaxPx * maxColumns) / count);
    thumbnailPx = clamp(scaled, MIN_THUMBNAIL_PX, thumbnailMaxPx);
  }

  return { columns, rows, thumbnailPx };
}

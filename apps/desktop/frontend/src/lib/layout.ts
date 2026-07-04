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

// SIZE_PRESET_PX maps the coarse Small/Medium/Large size preset (AltTab
// parity) to the pixel knobs the preferences UI writes into Appearance.
export const SIZE_PRESET_PX = {
  small: { thumbnail: 200, icon: 48 },
  medium: { thumbnail: 280, icon: 72 },
  large: { thumbnail: 360, icon: 96 },
} as const;

const MIN_THUMBNAIL_PX = 96;

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

// computeLayout derives grid dimensions and thumbnail size for a window count.
export function computeLayout(input: LayoutInput): Layout {
  const { count, maxColumns, maxRows, thumbnailMaxPx, autoSize } = input;
  const columns = Math.max(1, Math.min(maxColumns, count || 1));
  const rows = count <= 0 ? 0 : Math.ceil(count / columns);

  // AltTab-style sizing: let the grid grow rows at full size and only scale
  // thumbnails down once the window count exceeds the grid's capacity, with a
  // readable floor so they never become unusably small.
  let thumbnailPx = thumbnailMaxPx;
  const capacity = maxColumns * Math.max(1, maxRows);
  if (autoSize && count > capacity) {
    const scaled = Math.round((thumbnailMaxPx * capacity) / count);
    thumbnailPx = clamp(scaled, MIN_THUMBNAIL_PX, thumbnailMaxPx);
  }

  return { columns, rows, thumbnailPx };
}

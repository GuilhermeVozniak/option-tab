// Pure layout math for the switcher grid: how many columns/rows to use and how
// large thumbnails should be, including AltTab-style auto-sizing where the
// thumbnail shrinks as the number of windows grows.

export interface LayoutInput {
  count: number;
  maxColumns: number;
  maxRows: number;
  thumbnailMaxPx: number;
  autoSize: boolean;
  /** Overlay viewport (window.innerWidth/Height). 0/undefined = unconstrained. */
  viewportW?: number;
  viewportH?: number;
  showTitle?: boolean;
  previewEnabled?: boolean;
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

// Pixel metrics mirroring styles.css so the grid can be sized to fit inside
// the panel without scrollbars: .ot-panel max-width/height fractions and
// padding (18px × 2), per-cell chrome (entry padding/border + grid gap),
// titlebar row height, preview reserve, and the thumbnail aspect ratio.
const PANEL_W_FRAC = 0.92;
const PANEL_H_FRAC = 0.88;
const PANEL_PADDING = 36;
const CELL_CHROME_W = 22;
const CELL_CHROME_H = 22;
const TITLEBAR_H = 22;
const PREVIEW_H_FRAC = 0.34;
const PREVIEW_GAP = 12;
const THUMB_ASPECT = 0.62;

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

// computeLayout derives grid dimensions and thumbnail size for a window count.
export function computeLayout(input: LayoutInput): Layout {
  const { count, maxColumns, maxRows, thumbnailMaxPx, autoSize } = input;
  const { viewportW = 0, viewportH = 0, showTitle = false, previewEnabled = false } = input;
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

  // Fit-to-viewport: shrink thumbnails until the grid fits inside the panel's
  // max-width/height so the panel never needs scrollbars.
  if (viewportW > 0) {
    const availW = viewportW * PANEL_W_FRAC - PANEL_PADDING;
    const fit = Math.floor(availW / columns - CELL_CHROME_W);
    thumbnailPx = clamp(Math.min(thumbnailPx, fit), MIN_THUMBNAIL_PX, thumbnailMaxPx);
  }
  if (viewportH > 0 && rows > 0) {
    let availH = viewportH * PANEL_H_FRAC - PANEL_PADDING;
    if (previewEnabled) availH -= viewportH * PREVIEW_H_FRAC + PREVIEW_GAP;
    const cellH = availH / rows - CELL_CHROME_H - (showTitle ? TITLEBAR_H : 0);
    const fit = Math.floor(cellH / THUMB_ASPECT);
    thumbnailPx = clamp(Math.min(thumbnailPx, fit), MIN_THUMBNAIL_PX, thumbnailMaxPx);
  }

  return { columns, rows, thumbnailPx };
}

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
  layoutDirection?: "horizontal" | "vertical";
  /** Height above the grid, including the toolbar's bottom margin. */
  toolbarHeightPx?: number;
  /** Additional cell height reserved for metadata below the preview. */
  entryFooterHeightPx?: number;
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

export function effectiveStyle(
  style: "thumbnails" | "appIcons" | "titles",
  count: number,
  compactThreshold: number,
) {
  return compactThreshold > 0 && count >= compactThreshold ? "titles" : style;
}

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

// gridShape is the number of columns and rows the entries occupy. A horizontal
// grid fills rows first, a vertical grid fills columns first.
export function gridShape(
  count: number,
  maxColumns: number,
  maxRows: number,
  vertical: boolean,
): { columns: number; rows: number } {
  const rows =
    count <= 0
      ? 0
      : vertical
        ? Math.min(maxRows, count)
        : Math.ceil(count / Math.max(1, Math.min(maxColumns, count || 1)));
  const columns = vertical
    ? Math.max(1, Math.ceil(count / Math.max(rows, 1)))
    : Math.max(1, Math.min(maxColumns, count || 1));
  return { columns, rows };
}

// crossAxisIndex returns the entry one line away from index across the fill
// direction: a row down or up in a horizontal grid, a column right or left in
// a vertical one. lineLength is the number of entries in a full line. A
// shorter last line is entered at its last entry. null means there is no line
// in that direction.
export function crossAxisIndex(
  index: number,
  count: number,
  lineLength: number,
  delta: 1 | -1,
): number | null {
  if (lineLength <= 0 || index < 0 || index >= count) return null;
  const target = index + delta * lineLength;
  if (target < 0) return null;
  if (target < count) return target;
  const lastLine = Math.floor((count - 1) / lineLength);
  return Math.floor(index / lineLength) < lastLine ? count - 1 : null;
}

// computeLayout derives grid dimensions and thumbnail size for a window count.
export function computeLayout(input: LayoutInput): Layout {
  const { count, maxColumns, maxRows, thumbnailMaxPx, autoSize } = input;
  const { viewportW = 0, viewportH = 0, showTitle = false, previewEnabled = false } = input;
  const { columns, rows } = gridShape(
    count,
    maxColumns,
    maxRows,
    input.layoutDirection === "vertical",
  );

  // AltTab-style sizing: let the grid grow rows at full size and only scale
  // thumbnails down once the window count exceeds the grid's capacity, with a
  // readable floor so they never become unusably small.
  let thumbnailPx = thumbnailMaxPx;
  // The readability floor only limits shrinking. A configured size below it
  // is its own floor; otherwise the clamp would enlarge it back to 96.
  const minPx = Math.min(MIN_THUMBNAIL_PX, thumbnailMaxPx);
  const capacity = maxColumns * Math.max(1, maxRows);
  if (autoSize && count > capacity) {
    const scaled = Math.round((thumbnailMaxPx * capacity) / count);
    thumbnailPx = clamp(scaled, minPx, thumbnailMaxPx);
  }

  // Fit-to-viewport: shrink thumbnails until the grid fits inside the panel's
  // max-width/height so the panel never needs scrollbars.
  if (viewportW > 0) {
    const availW = viewportW * PANEL_W_FRAC - PANEL_PADDING;
    const fit = Math.floor(availW / columns - CELL_CHROME_W);
    thumbnailPx = clamp(Math.min(thumbnailPx, fit), minPx, thumbnailMaxPx);
  }
  if (viewportH > 0 && rows > 0) {
    let availH = viewportH * PANEL_H_FRAC - PANEL_PADDING - (input.toolbarHeightPx ?? 0);
    if (previewEnabled) availH -= viewportH * PREVIEW_H_FRAC + PREVIEW_GAP;
    const cellH =
      availH / rows -
      CELL_CHROME_H -
      (showTitle ? TITLEBAR_H : 0) -
      (input.entryFooterHeightPx ?? 0);
    const fit = Math.floor(cellH / THUMB_ASPECT);
    thumbnailPx = clamp(Math.min(thumbnailPx, fit), minPx, thumbnailMaxPx);
  }

  return { columns, rows, thumbnailPx };
}

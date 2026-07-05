import { describe, expect, it } from "vitest";
import { computeLayout } from "./layout";

describe("computeLayout", () => {
  it("uses up to maxColumns columns", () => {
    expect(
      computeLayout({ count: 10, maxColumns: 6, maxRows: 4, thumbnailMaxPx: 256, autoSize: false })
        .columns,
    ).toBe(6);
    expect(
      computeLayout({ count: 3, maxColumns: 6, maxRows: 4, thumbnailMaxPx: 256, autoSize: false })
        .columns,
    ).toBe(3);
  });

  it("computes rows from count and columns", () => {
    const l = computeLayout({
      count: 7,
      maxColumns: 3,
      maxRows: 10,
      thumbnailMaxPx: 256,
      autoSize: false,
    });
    expect(l.columns).toBe(3);
    expect(l.rows).toBe(3); // ceil(7/3)
  });

  it("handles zero windows without dividing by zero", () => {
    const l = computeLayout({
      count: 0,
      maxColumns: 6,
      maxRows: 4,
      thumbnailMaxPx: 256,
      autoSize: false,
    });
    expect(l.columns).toBeGreaterThanOrEqual(1);
    expect(l.rows).toBe(0);
  });

  it("keeps thumbnails at max size when autoSize is off", () => {
    const l = computeLayout({
      count: 50,
      maxColumns: 6,
      maxRows: 4,
      thumbnailMaxPx: 256,
      autoSize: false,
    });
    expect(l.thumbnailPx).toBe(256);
  });

  it("shrinks thumbnails to fit the viewport width (no horizontal scroll)", () => {
    const l = computeLayout({
      count: 5,
      maxColumns: 5,
      maxRows: 4,
      thumbnailMaxPx: 280,
      autoSize: false,
      viewportW: 1280,
      viewportH: 820,
    });
    // 5 × 280px would be wider than 92% of a 1280px window; it must shrink.
    expect(l.thumbnailPx).toBeLessThan(280);
    expect(l.columns * (l.thumbnailPx + 22)).toBeLessThanOrEqual(1280 * 0.92 - 36);
  });

  it("reserves space for the selected-window preview when fitting height", () => {
    const base = {
      count: 4,
      maxColumns: 2,
      maxRows: 4,
      thumbnailMaxPx: 280,
      autoSize: false,
      viewportW: 1280,
      viewportH: 600,
      showTitle: true,
    };
    const withPreview = computeLayout({ ...base, previewEnabled: true });
    const withoutPreview = computeLayout({ ...base, previewEnabled: false });
    expect(withPreview.thumbnailPx).toBeLessThan(withoutPreview.thumbnailPx);
  });

  it("never fits below the readable floor", () => {
    const l = computeLayout({
      count: 8,
      maxColumns: 8,
      maxRows: 1,
      thumbnailMaxPx: 360,
      autoSize: false,
      viewportW: 500,
      viewportH: 400,
    });
    expect(l.thumbnailPx).toBe(96);
  });

  it("keeps legacy behavior when no viewport is given", () => {
    const l = computeLayout({
      count: 5,
      maxColumns: 5,
      maxRows: 4,
      thumbnailMaxPx: 280,
      autoSize: false,
    });
    expect(l.thumbnailPx).toBe(280);
  });

  it("shrinks thumbnails as count grows when autoSize is on", () => {
    const few = computeLayout({
      count: 2,
      maxColumns: 6,
      maxRows: 4,
      thumbnailMaxPx: 256,
      autoSize: true,
    });
    const many = computeLayout({
      count: 40,
      maxColumns: 6,
      maxRows: 4,
      thumbnailMaxPx: 256,
      autoSize: true,
    });
    expect(few.thumbnailPx).toBe(256);
    expect(many.thumbnailPx).toBeLessThan(256);
    expect(many.thumbnailPx).toBeGreaterThanOrEqual(96); // floor
  });
});

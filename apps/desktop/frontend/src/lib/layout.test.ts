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

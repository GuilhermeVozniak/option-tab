import { describe, expect, it } from "vitest";
import { spaceBadgeNumbers } from "./spaces";

describe("spaceBadgeNumbers", () => {
  it("numbers Spaces by sorted id, never by the raw id", () => {
    const numbers = spaceBadgeNumbers([{ spaceId: 187 }, { spaceId: 3 }, { spaceId: 187 }, {}]);
    expect([...numbers]).toEqual([
      [3, 1],
      [187, 2],
    ]);
  });

  it("gives no badge when every window is on one Space", () => {
    expect(spaceBadgeNumbers([{ spaceId: 42 }, { spaceId: 42 }]).size).toBe(0);
    expect(spaceBadgeNumbers([{ spaceId: 0 }, {}]).size).toBe(0);
    expect(spaceBadgeNumbers([]).size).toBe(0);
  });
});

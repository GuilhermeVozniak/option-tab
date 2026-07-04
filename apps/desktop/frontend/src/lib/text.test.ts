import { describe, expect, it } from "vitest";
import { truncateTitle } from "./text";

describe("truncateTitle", () => {
  const long = `${"a".repeat(40)}MID${"b".repeat(40)}`;

  it("leaves short titles alone in every mode", () => {
    for (const mode of ["end", "middle", "start"] as const) {
      expect(truncateTitle("short", mode)).toBe("short");
    }
  });

  it("leaves end mode to CSS (returns the title unchanged)", () => {
    expect(truncateTitle(long, "end")).toBe(long);
  });

  it("elides the middle keeping both ends", () => {
    const got = truncateTitle(long, "middle", 21);
    expect(got).toHaveLength(21);
    expect(got.startsWith("aaa")).toBe(true);
    expect(got.endsWith("bbb")).toBe(true);
    expect(got).toContain("…");
  });

  it("elides the start keeping the tail", () => {
    const got = truncateTitle(long, "start", 11);
    expect(got).toHaveLength(11);
    expect(got.startsWith("…")).toBe(true);
    expect(got.endsWith("b".repeat(10))).toBe(true);
  });
});

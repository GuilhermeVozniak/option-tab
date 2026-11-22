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

  it("defaults maxChars to 60 when omitted", () => {
    const got = truncateTitle("c".repeat(80), "middle");
    expect(got).toHaveLength(60);
    expect(got).toContain("…");
  });

  it("elides the start keeping the tail", () => {
    const got = truncateTitle(long, "start", 11);
    expect(got).toHaveLength(11);
    expect(got.startsWith("…")).toBe(true);
    expect(got.endsWith("b".repeat(10))).toBe(true);
  });

  // A cut that lands inside an astral character must not leave half of it.
  const loneSurrogate = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/;

  it("never splits a surrogate pair when eliding the start", () => {
    // 5 UTF-16 units from the end would start on the low half of the emoji.
    const got = truncateTitle(`${"a".repeat(20)}😀bbbb`, "start", 6);
    expect(got).toBe("…😀bbbb");
    expect(got).not.toMatch(loneSurrogate);
  });

  it("never splits a surrogate pair when eliding the middle", () => {
    // Odd offsets put both the head and the tail cut inside an emoji.
    const title = `a${"😀".repeat(20)}b`;
    const got = truncateTitle(title, "middle", 8);
    expect(got).toBe("a😀😀😀…😀😀b");
    expect(got).not.toMatch(loneSurrogate);
  });

  it("measures length in code points, not UTF-16 units", () => {
    const emoji = "😀".repeat(8); // 8 characters, 16 UTF-16 units
    for (const mode of ["middle", "start"] as const) {
      expect(truncateTitle(emoji, mode, 10)).toBe(emoji);
    }
    expect(Array.from(truncateTitle("😀".repeat(30), "middle", 11))).toHaveLength(11);
    expect(Array.from(truncateTitle("😀".repeat(30), "start", 11))).toHaveLength(11);
  });
});

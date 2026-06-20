import { describe, expect, it } from "vitest";
import { sanitizeName } from "./name";

describe("sanitizeName", () => {
  it("trims surrounding whitespace", () => {
    expect(sanitizeName("  Gui  ")).toBe("Gui");
  });
  it("collapses inner whitespace runs", () => {
    expect(sanitizeName("Ana   Maria")).toBe("Ana Maria");
  });
});

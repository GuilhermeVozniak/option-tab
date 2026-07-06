import { describe, expect, it } from "vitest";
import { chordFromEvent, keyTokenFromCode } from "./chord";

function ev(
  code: string,
  mods: Partial<Record<"ctrlKey" | "altKey" | "shiftKey" | "metaKey", boolean>> = {},
) {
  return { code, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods };
}

describe("keyTokenFromCode", () => {
  it("maps letters, digits, and named keys", () => {
    expect(keyTokenFromCode("KeyA")).toBe("a");
    expect(keyTokenFromCode("Digit7")).toBe("7");
    expect(keyTokenFromCode("Tab")).toBe("tab");
    expect(keyTokenFromCode("Backquote")).toBe("grave");
    expect(keyTokenFromCode("Enter")).toBe("return");
  });

  it("maps space, escape, and arrow keys", () => {
    expect(keyTokenFromCode("Space")).toBe("space");
    expect(keyTokenFromCode("Escape")).toBe("escape");
    expect(keyTokenFromCode("ArrowLeft")).toBe("left");
    expect(keyTokenFromCode("ArrowRight")).toBe("right");
    expect(keyTokenFromCode("ArrowUp")).toBe("up");
    expect(keyTokenFromCode("ArrowDown")).toBe("down");
  });

  it("rejects modifiers and unknown keys", () => {
    expect(keyTokenFromCode("AltLeft")).toBeNull();
    expect(keyTokenFromCode("MetaRight")).toBeNull();
    expect(keyTokenFromCode("F5")).toBeNull();
  });
});

describe("chordFromEvent", () => {
  it("builds modifier+key chords in canonical order", () => {
    expect(chordFromEvent(ev("Tab", { altKey: true }))).toBe("option+tab");
    expect(chordFromEvent(ev("Tab", { metaKey: true, shiftKey: true }))).toBe("shift+command+tab");
    expect(chordFromEvent(ev("Backquote", { ctrlKey: true, altKey: true }))).toBe(
      "control+option+grave",
    );
  });

  it("returns null without a modifier or for modifier-only presses", () => {
    expect(chordFromEvent(ev("Tab"))).toBeNull();
    expect(chordFromEvent(ev("AltLeft", { altKey: true }))).toBeNull();
  });
});

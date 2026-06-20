import { describe, expect, it } from "vitest";
import { keyToAction } from "./keymap";

const ev = (
  key: string,
  mods: Partial<{ shiftKey: boolean; ctrlKey: boolean; metaKey: boolean; altKey: boolean }> = {},
) => ({
  key,
  shiftKey: false,
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  ...mods,
});

describe("keyToAction", () => {
  it("maps Tab to advance and Shift+Tab to reverse", () => {
    expect(keyToAction(ev("Tab"))).toEqual({ kind: "advance" });
    expect(keyToAction(ev("Tab", { shiftKey: true }))).toEqual({ kind: "reverse" });
  });

  it("maps arrows", () => {
    expect(keyToAction(ev("ArrowRight"))).toEqual({ kind: "advance" });
    expect(keyToAction(ev("ArrowDown"))).toEqual({ kind: "advance" });
    expect(keyToAction(ev("ArrowLeft"))).toEqual({ kind: "reverse" });
    expect(keyToAction(ev("ArrowUp"))).toEqual({ kind: "reverse" });
  });

  it("maps Escape to cancel and Enter to confirm", () => {
    expect(keyToAction(ev("Escape"))).toEqual({ kind: "cancel" });
    expect(keyToAction(ev("Enter"))).toEqual({ kind: "confirm" });
  });

  it("maps Backspace to searchBackspace", () => {
    expect(keyToAction(ev("Backspace"))).toEqual({ kind: "searchBackspace" });
  });

  it("maps a printable character to searchAppend", () => {
    expect(keyToAction(ev("a"))).toEqual({ kind: "searchAppend", char: "a" });
    expect(keyToAction(ev("Z"))).toEqual({ kind: "searchAppend", char: "Z" });
    expect(keyToAction(ev(" "))).toEqual({ kind: "searchAppend", char: " " });
  });

  it("ignores printable characters when a command/control modifier is held", () => {
    expect(keyToAction(ev("a", { metaKey: true }))).toEqual({ kind: "none" });
    expect(keyToAction(ev("a", { ctrlKey: true }))).toEqual({ kind: "none" });
  });

  it("returns none for unhandled keys", () => {
    expect(keyToAction(ev("F5"))).toEqual({ kind: "none" });
    expect(keyToAction(ev("Shift"))).toEqual({ kind: "none" });
  });
});

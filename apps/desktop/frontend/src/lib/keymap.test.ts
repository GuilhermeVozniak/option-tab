import { describe, expect, it } from "vitest";
import { keyToAction } from "./keymap";

const ev = (
  key: string,
  mods: Partial<{
    code: string;
    shiftKey: boolean;
    ctrlKey: boolean;
    metaKey: boolean;
    altKey: boolean;
  }> = {},
) => ({
  key,
  code: "",
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

  it("ignores arrows when arrowKeys is disabled", () => {
    const opts = { arrowKeys: false };
    expect(keyToAction(ev("ArrowRight"), opts)).toEqual({ kind: "none" });
    expect(keyToAction(ev("ArrowLeft"), opts)).toEqual({ kind: "none" });
    // Tab is unaffected by the arrow-keys toggle.
    expect(keyToAction(ev("Tab"), opts)).toEqual({ kind: "advance" });
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

  it("maps held-modifier + physical key to window actions (Option char tolerant)", () => {
    // Option+W yields key "∑" but code "KeyW"; must still close.
    expect(keyToAction(ev("∑", { code: "KeyW", altKey: true }))).toEqual({ kind: "close" });
    expect(keyToAction(ev("m", { code: "KeyM", metaKey: true }))).toEqual({ kind: "minimize" });
    expect(keyToAction(ev("q", { code: "KeyQ", metaKey: true }))).toEqual({ kind: "quit" });
    expect(keyToAction(ev("h", { code: "KeyH", altKey: true }))).toEqual({ kind: "hide" });
    expect(keyToAction(ev("f", { code: "KeyF", altKey: true }))).toEqual({ kind: "fullscreen" });
  });

  it("does not trigger window actions without a modifier (letters type into search)", () => {
    expect(keyToAction(ev("w", { code: "KeyW" }))).toEqual({ kind: "searchAppend", char: "w" });
    expect(keyToAction(ev("q", { code: "KeyQ" }))).toEqual({ kind: "searchAppend", char: "q" });
  });

  it("maps vim keys to navigation only when enabled", () => {
    const opts = { vimKeys: true };
    expect(keyToAction(ev("h", { code: "KeyH" }), opts)).toEqual({ kind: "reverse" });
    expect(keyToAction(ev("k", { code: "KeyK" }), opts)).toEqual({ kind: "reverse" });
    expect(keyToAction(ev("l", { code: "KeyL" }), opts)).toEqual({ kind: "advance" });
    expect(keyToAction(ev("j", { code: "KeyJ" }), opts)).toEqual({ kind: "advance" });
    // Disabled by default: those letters type into search.
    expect(keyToAction(ev("j", { code: "KeyJ" }))).toEqual({ kind: "searchAppend", char: "j" });
  });

  it("never routes vim keys while a modifier is held (window actions win)", () => {
    const opts = { vimKeys: true };
    // Option+H yields key "˙" but code "KeyH"; with vimKeys enabled the held
    // modifier must resolve to the window action, never vim navigation.
    expect(keyToAction(ev("˙", { code: "KeyH", altKey: true }), opts)).toEqual({ kind: "hide" });
    // KeyJ has no window action; with a modifier held it must not advance —
    // the Option-produced character falls through to type-to-search.
    expect(keyToAction(ev("∆", { code: "KeyJ", altKey: true }), opts)).toEqual({
      kind: "searchAppend",
      char: "∆",
    });
  });

  it("appends Option-produced characters to search when the code is not an action key", () => {
    // Option+A yields "å"; KeyA is not a window-action code, so it types.
    expect(keyToAction(ev("å", { code: "KeyA", altKey: true }))).toEqual({
      kind: "searchAppend",
      char: "å",
    });
  });
});

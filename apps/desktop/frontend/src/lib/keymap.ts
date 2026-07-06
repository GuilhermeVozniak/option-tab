// Pure mapping from a keyboard event to a switcher action. Kept free of React
// and DOM so it is unit-testable in isolation.

export type ActionKind =
  | "advance"
  | "reverse"
  | "confirm"
  | "cancel"
  | "close"
  | "minimize"
  | "quit"
  | "hide"
  | "fullscreen"
  | "searchBackspace";

export type Action =
  | { kind: ActionKind }
  | { kind: "searchAppend"; char: string }
  | { kind: "none" };

export interface KeyEventLike {
  key: string;
  code: string;
  shiftKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}

export interface KeymapOptions {
  // vimKeys enables h/j/k/l navigation (opt-in, AltTab "Vim keys").
  vimKeys?: boolean;
  // arrowKeys enables arrow-key navigation (on by default).
  arrowKeys?: boolean;
}

// ACTION_BY_CODE maps a physical key to a window action. Matched on e.code (not
// e.key) so an Option-mangled character (e.g. Option+W yields "∑") still resolves
// — mirroring AltTab's "hold the switcher modifier, tap W/M/Q/H/F". The keys also
// line up with macOS conventions (⌘W close, ⌘M minimize, ⌘Q quit, ⌘H hide).
const ACTION_BY_CODE: Record<string, ActionKind> = {
  KeyW: "close",
  KeyM: "minimize",
  KeyQ: "quit",
  KeyH: "hide",
  KeyF: "fullscreen",
};

// keyToAction translates a key event into the switcher action it should trigger.
export function keyToAction(e: KeyEventLike, opts: KeymapOptions = {}): Action {
  switch (e.key) {
    case "Tab":
      return e.shiftKey ? { kind: "reverse" } : { kind: "advance" };
    case "ArrowRight":
    case "ArrowDown":
      return opts.arrowKeys === false ? { kind: "none" } : { kind: "advance" };
    case "ArrowLeft":
    case "ArrowUp":
      return opts.arrowKeys === false ? { kind: "none" } : { kind: "reverse" };
    case "Escape":
      return { kind: "cancel" };
    case "Enter":
      return { kind: "confirm" };
    case "Backspace":
      return { kind: "searchBackspace" };
  }

  const hasMod = e.altKey || e.metaKey || e.ctrlKey;

  // Window actions: a held modifier + a mapped physical key. Requiring a
  // modifier keeps bare letters free for type-to-search.
  if (hasMod) {
    const action = ACTION_BY_CODE[e.code];
    if (action) return { kind: action };
  }

  // Vim navigation (opt-in), only without a modifier so it never shadows actions.
  if (opts.vimKeys && !hasMod) {
    if (e.code === "KeyH" || e.code === "KeyK") return { kind: "reverse" };
    if (e.code === "KeyL" || e.code === "KeyJ") return { kind: "advance" };
  }

  // A single printable character (no command/control modifier) types into the
  // search box. Option/Shift are allowed because they produce characters.
  if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
    return { kind: "searchAppend", char: e.key };
  }
  return { kind: "none" };
}

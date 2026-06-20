// Pure mapping from a keyboard event to a switcher action. Kept free of React
// and DOM so it is unit-testable in isolation.

export type Action =
  | { kind: "advance" }
  | { kind: "reverse" }
  | { kind: "confirm" }
  | { kind: "cancel" }
  | { kind: "searchAppend"; char: string }
  | { kind: "searchBackspace" }
  | { kind: "none" };

export interface KeyEventLike {
  key: string;
  shiftKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}

// keyToAction translates a key event into the switcher action it should trigger.
export function keyToAction(e: KeyEventLike): Action {
  switch (e.key) {
    case "Tab":
      return e.shiftKey ? { kind: "reverse" } : { kind: "advance" };
    case "ArrowRight":
    case "ArrowDown":
      return { kind: "advance" };
    case "ArrowLeft":
    case "ArrowUp":
      return { kind: "reverse" };
    case "Escape":
      return { kind: "cancel" };
    case "Enter":
      return { kind: "confirm" };
    case "Backspace":
      return { kind: "searchBackspace" };
  }

  // A single printable character (with no command/control modifier) types into
  // the search box. Option/Shift are allowed because they produce characters.
  if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
    return { kind: "searchAppend", char: e.key };
  }
  return { kind: "none" };
}

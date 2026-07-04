// Pure keyboard-chord building for the shortcut recorder. Mirrors the grammar
// of the Go hotkey parser (internal/hotkey): modifiers in canonical order
// (control, option, shift, command) joined with "+", ending in one key token.

export interface ChordKeyEvent {
  code: string;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
}

const CODE_KEYS: Record<string, string> = {
  Tab: "tab",
  Space: "space",
  Escape: "escape",
  Enter: "return",
  Backquote: "grave",
  ArrowLeft: "left",
  ArrowRight: "right",
  ArrowUp: "up",
  ArrowDown: "down",
};

// keyTokenFromCode maps a KeyboardEvent.code to a hotkey key token, or null
// for keys a chord cannot bind to (including bare modifiers).
export function keyTokenFromCode(code: string): string | null {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3).toLowerCase();
  if (/^Digit[0-9]$/.test(code)) return code.slice(5);
  return CODE_KEYS[code] ?? null;
}

// chordFromEvent builds a chord string from a keydown, or null when the event
// is not a bindable chord (no modifier held, or a modifier-only keypress).
export function chordFromEvent(e: ChordKeyEvent): string | null {
  const mods: string[] = [];
  if (e.ctrlKey) mods.push("control");
  if (e.altKey) mods.push("option");
  if (e.shiftKey) mods.push("shift");
  if (e.metaKey) mods.push("command");
  const key = keyTokenFromCode(e.code);
  if (!key || mods.length === 0) return null;
  return [...mods, key].join("+");
}

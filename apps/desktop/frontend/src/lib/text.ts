// Text helpers for the overlay.
import type { TruncationMode } from "./types";

// truncateTitle elides a long title at the configured position (AltTab's
// "window title truncation" preference). "end" is left to CSS text-overflow so
// it stays pixel-accurate; middle/start are character-based. Characters are
// code points, so a cut never leaves half of a surrogate pair (an emoji).
export function truncateTitle(title: string, mode: TruncationMode, maxChars = 60): string {
  if (mode === "end" || title.length <= maxChars) return title;
  const chars = Array.from(title);
  if (chars.length <= maxChars) return title;
  const ellipsis = "…";
  if (mode === "start") {
    return ellipsis + chars.slice(chars.length - (maxChars - 1)).join("");
  }
  // middle
  const head = Math.ceil((maxChars - 1) / 2);
  const tail = maxChars - 1 - head;
  return chars.slice(0, head).join("") + ellipsis + chars.slice(chars.length - tail).join("");
}

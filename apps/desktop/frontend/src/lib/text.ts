// Text helpers for the overlay.
import type { TruncationMode } from "./types";

// truncateTitle elides a long title at the configured position (AltTab's
// "window title truncation" preference). "end" is left to CSS text-overflow so
// it stays pixel-accurate; middle/start are character-based.
export function truncateTitle(title: string, mode: TruncationMode, maxChars = 60): string {
  if (mode === "end" || title.length <= maxChars) return title;
  const ellipsis = "…";
  if (mode === "start") {
    return ellipsis + title.slice(title.length - (maxChars - 1));
  }
  // middle
  const head = Math.ceil((maxChars - 1) / 2);
  const tail = maxChars - 1 - head;
  return title.slice(0, head) + ellipsis + title.slice(title.length - tail);
}

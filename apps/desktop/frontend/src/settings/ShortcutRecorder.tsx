import { useRef, useState } from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { system } from "../lib/bridge";
import { chordFromEvent } from "../lib/chord";

interface ShortcutRecorderProps {
  "aria-label": string;
  value: string;
  placeholder: string;
  onChordChange: (chord: string) => void;
}

// ShortcutRecorder captures the chord the user actually presses (AltTab-style)
// instead of asking them to type its name: focus the field, press e.g.
// ⌥⇥, and the canonical chord string ("option+tab") is stored.
//
// Recording goes through the native event tap (system.captureShortcut), which
// sees chords the DOM never receives: Command+Tab (taken by macOS) and the
// switcher's own registered chord (taken by our tap). DOM key events remain as
// the fallback when the native binding is unavailable (browser/dev/tests).
export function ShortcutRecorder({
  value,
  placeholder,
  onChordChange,
  ...aria
}: ShortcutRecorderProps) {
  const [recording, setRecording] = useState(false);
  const armed = useRef(false);

  const startNativeCapture = async () => {
    setRecording(true);
    if (armed.current) return;
    armed.current = true;
    const chord = await system.captureShortcut();
    armed.current = false;
    if (chord === null) return; // no native capture: DOM fallback below records
    if (chord) onChordChange(chord);
    setRecording(false);
  };

  return (
    <Input
      {...aria}
      type="text"
      readOnly
      value={value}
      placeholder={placeholder}
      className={cn(
        "w-44 cursor-pointer text-center font-medium",
        recording && "border-primary/70 ring-2 ring-ring/40",
      )}
      onFocus={() => void startNativeCapture()}
      onBlur={() => {
        setRecording(false);
        if (armed.current) void system.cancelShortcutCapture();
      }}
      onKeyDown={(e) => {
        e.preventDefault();
        const chord = chordFromEvent(e);
        if (chord) onChordChange(chord);
      }}
    />
  );
}

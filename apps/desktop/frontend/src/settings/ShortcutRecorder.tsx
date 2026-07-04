import { useState } from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
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
export function ShortcutRecorder({
  value,
  placeholder,
  onChordChange,
  ...aria
}: ShortcutRecorderProps) {
  const [recording, setRecording] = useState(false);
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
      onFocus={() => setRecording(true)}
      onBlur={() => setRecording(false)}
      onKeyDown={(e) => {
        e.preventDefault();
        const chord = chordFromEvent(e);
        if (chord) onChordChange(chord);
      }}
    />
  );
}

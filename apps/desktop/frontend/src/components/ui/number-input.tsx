import type * as React from "react";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Input } from "./input";

type NumberDraftOptions = {
  value: number;
  onCommit: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
};

type NumberInputProps = Omit<
  React.ComponentProps<"input">,
  "type" | "value" | "defaultValue" | "onChange" | "min" | "max" | "step"
> &
  NumberDraftOptions;

// How long typing has to pause before the draft is saved without a blur.
const IDLE_COMMIT_MS = 600;

// Editing state for a native number input that edits a local draft. Values are
// saved and echoed back in canonical form on every change, so committing each
// keystroke would rewrite the field under the caret (and an in-between state
// such as "" or "-" would be saved as 0). Typed text commits on blur or Enter,
// clamped to min/max; the native stepper commits immediately. A pause in
// typing saves the clamped value too, but leaves the text alone: the switcher
// hotkey never takes focus from Preferences, so a blur may not come.
function useNumberDraft({ value, onCommit, min, max, step = 1 }: NumberDraftOptions) {
  const [draft, setDraft] = useState<string | null>(null);
  // True while a text-editing key is held, so a keystroke is never mistaken for
  // a stepper change even if the engine omits InputEvent.inputType.
  const typingKey = useRef(false);
  const idle = useRef<ReturnType<typeof setTimeout> | null>(null);
  const latest = useRef({ value, onCommit, min, max, step });
  useLayoutEffect(() => {
    latest.current = { value, onCommit, min, max, step };
  });

  const cancelIdle = useCallback(() => {
    if (idle.current !== null) clearTimeout(idle.current);
    idle.current = null;
  }, []);
  // Reads the latest props: it also runs from a timer and a window listener.
  const save = useCallback((text: string) => {
    const current = latest.current;
    const parsed = text.trim() === "" ? Number.NaN : Number(text);
    if (!Number.isFinite(parsed)) return;
    let next = Number.isInteger(current.step) ? Math.round(parsed) : parsed;
    if (current.min !== undefined) next = Math.max(current.min, next);
    if (current.max !== undefined) next = Math.min(current.max, next);
    if (next !== current.value) current.onCommit(next);
  }, []);
  const commit = useCallback(
    (text: string) => {
      cancelIdle();
      setDraft(null);
      save(text);
    },
    [cancelIdle, save],
  );

  useEffect(() => cancelIdle, [cancelIdle]);
  // Hiding or deactivating the window does not blur the field in every engine,
  // and a draft left behind would look like a saved value on the next open.
  useEffect(() => {
    if (draft === null) return;
    const flush = () => commit(draft);
    window.addEventListener("blur", flush);
    window.addEventListener("pagehide", flush);
    return () => {
      window.removeEventListener("blur", flush);
      window.removeEventListener("pagehide", flush);
    };
  }, [draft, commit]);

  return {
    value: draft ?? String(value),
    onChange: (event: React.ChangeEvent<HTMLInputElement>) => {
      const text = event.target.value;
      // Stepper arrows and Arrow keys dispatch an input event with no
      // inputType; typing, deleting and pasting always carry one.
      const stepped = !(event.nativeEvent as InputEvent).inputType && !typingKey.current;
      if (stepped) {
        commit(text);
        return;
      }
      setDraft(text);
      cancelIdle();
      idle.current = setTimeout(() => {
        idle.current = null;
        save(text);
      }, IDLE_COMMIT_MS);
    },
    onBlur: () => {
      typingKey.current = false;
      if (draft !== null) commit(draft);
    },
    onKeyDown: (event: React.KeyboardEvent<HTMLInputElement>) => {
      typingKey.current = event.key !== "ArrowUp" && event.key !== "ArrowDown";
      if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
      if (event.key === "Enter") {
        event.preventDefault();
        if (draft !== null) commit(draft);
      } else if (event.key === "Escape" && draft !== null) {
        event.preventDefault();
        cancelIdle();
        setDraft(null);
      }
    },
    onKeyUp: () => {
      typingKey.current = false;
    },
  };
}

function NumberInput({
  value,
  onCommit,
  min,
  max,
  step = 1,
  onBlur,
  onKeyDown,
  onKeyUp,
  ...props
}: NumberInputProps) {
  const field = useNumberDraft({ value, onCommit, min, max, step });
  return (
    <Input
      {...props}
      type="number"
      min={min}
      max={max}
      step={step}
      value={field.value}
      onChange={field.onChange}
      onBlur={(event) => {
        field.onBlur();
        onBlur?.(event);
      }}
      onKeyDown={(event) => {
        onKeyDown?.(event);
        field.onKeyDown(event);
      }}
      onKeyUp={(event) => {
        field.onKeyUp();
        onKeyUp?.(event);
      }}
    />
  );
}

export { NumberInput, useNumberDraft };

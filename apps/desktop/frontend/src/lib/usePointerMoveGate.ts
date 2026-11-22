import { useCallback, useEffect, useRef } from "react";

export interface PointerPosition {
  screenX: number;
  screenY: number;
}

// usePointerMoveGate tells a pointer that really moved from the mouse events
// a browser sends when content appears under a pointer that is resting. The
// switcher opens in the middle of the screen, often right under the pointer;
// selecting the card that happens to be there would switch to a window the
// user never pointed at.
//
// The returned function reports whether an event is a move away from the
// last position seen. The first event after resetKey changes only records
// where the pointer is.
export function usePointerMoveGate(resetKey: unknown): (event: PointerPosition) => boolean {
  const last = useRef<PointerPosition | null>(null);
  useEffect(() => {
    last.current = null;
  }, [resetKey]);
  return useCallback((event: PointerPosition) => {
    const previous = last.current;
    last.current = { screenX: event.screenX, screenY: event.screenY };
    return (
      previous !== null &&
      (previous.screenX !== event.screenX || previous.screenY !== event.screenY)
    );
  }, []);
}

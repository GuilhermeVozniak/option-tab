import { useCallback, useLayoutEffect, useRef } from "react";
import type { SwitcherState } from "./types";

// Accumulate keys immediately; older backend echoes must not replace pending
// text. The session and sequence also let Go reject reordered/retired RPCs.
export function useSwitcherSearch(
  state: SwitcherState,
  send: (query: string, session?: number, sequence?: number) => void,
) {
  const input = useRef({
    session: state.session ?? 0,
    open: state.open,
    query: state.search,
    sequence: state.searchSequence ?? 0,
  });
  useLayoutEffect(() => {
    const session = state.session ?? 0;
    const sequence = state.searchSequence ?? 0;
    if (input.current.session !== session || input.current.open !== state.open) {
      input.current = { session, open: state.open, query: state.search, sequence };
    } else if (sequence >= input.current.sequence) {
      input.current.query = state.search;
      input.current.sequence = sequence;
    }
  }, [state.session, state.open, state.search, state.searchSequence]);
  return useCallback(
    (text: string | null) => {
      const current = input.current;
      if (!current.open) return;
      current.query =
        text === null ? Array.from(current.query).slice(0, -1).join("") : current.query + text;
      current.sequence++;
      if (current.session > 0) send(current.query, current.session, current.sequence);
      else send(current.query);
    },
    [send],
  );
}

import { useLayoutEffect, useRef } from "react";

// Native key events change selection without focusing a DOM element, so the
// browser will not scroll it for us. Nearest keeps already-visible cards still.
export function useScrollSelection<T extends HTMLElement>(
  selection: string,
  active: boolean,
  selector = '[aria-selected="true"]',
) {
  const container = useRef<T>(null);
  useLayoutEffect(() => {
    if (!active) return;
    container.current?.querySelector<HTMLElement>(selector)?.scrollIntoView?.({
      block: "nearest",
      inline: "nearest",
      behavior: "instant",
    });
  }, [selection, active, selector]);
  return container;
}

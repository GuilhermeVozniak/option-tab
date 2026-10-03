import { useEffect, useRef } from "react";

export interface MaterialStatus {
  session: number;
  revision: number;
  state: "system" | "solid" | "unavailable";
  reason?: string;
}

export interface MaterialRect {
  session: number;
  stateRevision: number;
  sequence: number;
  x: number;
  y: number;
  width: number;
  height: number;
}

export function materialClass(
  blur: boolean,
  status?: MaterialStatus | null,
  session?: number,
): string {
  return blur && status?.state === "system" && (!session || status.session === session)
    ? "ot-native-material"
    : "ot-solid-material";
}

export function admitMaterialStatus(
  current: MaterialStatus | null,
  next: MaterialStatus,
  session: number,
): MaterialStatus | null {
  if (
    next.session !== session ||
    next.revision <= 0 ||
    (current?.session === session && next.revision <= current.revision)
  )
    return current;
  return next;
}

export function useMaterialReporter(
  session: number,
  stateRevision: number,
  report?: (rect: MaterialRect) => void,
  active = true,
) {
  const node = useRef<HTMLElement | null>(null);
  const sequence = useRef(0);
  const last = useRef("");
  const pending = useRef<number | null>(null);
  const lastSentAt = useRef(0);
  const reporter = useRef(report);
  reporter.current = report;
  useEffect(() => {
    sequence.current = 0;
    last.current = "";
  }, [session]);
  useEffect(() => {
    const element = node.current;
    if (!active || !element || !reporter.current || session <= 0 || stateRevision <= 0) return;
    let mounted = true;
    let timer: number | undefined;
    const send = () => {
      pending.current = null;
      if (!mounted || !node.current || !reporter.current) return;
      const rect = node.current.getBoundingClientRect();
      if (
        ![rect.x, rect.y, rect.width, rect.height].every(Number.isFinite) ||
        rect.width <= 0 ||
        rect.height <= 0
      )
        return;
      const signature = [stateRevision, rect.x, rect.y, rect.width, rect.height].join(":");
      if (signature === last.current) return;
      last.current = signature;
      lastSentAt.current = performance.now();
      reporter.current({
        session,
        stateRevision,
        sequence: ++sequence.current,
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height,
      });
    };
    const schedule = () => {
      if (pending.current !== null || timer !== undefined) return;
      const wait = Math.max(0, 34 - (performance.now() - lastSentAt.current));
      timer = window.setTimeout(() => {
        timer = undefined;
        pending.current = window.requestAnimationFrame(send);
      }, wait);
    };
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(schedule);
    observer?.observe(element);
    element.addEventListener("transitionend", schedule);
    element.addEventListener("animationend", schedule);
    window.addEventListener("resize", schedule);
    schedule();
    return () => {
      mounted = false;
      observer?.disconnect();
      element.removeEventListener("transitionend", schedule);
      element.removeEventListener("animationend", schedule);
      window.removeEventListener("resize", schedule);
      if (timer !== undefined) window.clearTimeout(timer);
      if (pending.current !== null) window.cancelAnimationFrame(pending.current);
      pending.current = null;
    };
  }, [active, session, stateRevision]);
  return (element: HTMLElement | null) => {
    node.current = element;
  };
}

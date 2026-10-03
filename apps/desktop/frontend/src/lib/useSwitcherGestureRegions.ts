import { useCallback, useLayoutEffect, useRef } from "react";
import type { Entry } from "./types";

export interface SwitcherGestureRegion {
  windowId: number;
  appId: number;
  bounds: { x: number; y: number; w: number; h: number };
}
export type SwitcherGestureReporter = (
  session: number,
  stateRevision: number,
  regions: SwitcherGestureRegion[],
) => void;
export interface SwitcherGesturePublication {
  session: number;
  stateRevision: number;
  sequence: number;
  regions: SwitcherGestureRegion[];
}

// Owned by the route, so switching renderers cannot reset a live session's
// sequence. Replies never publish geometry: only the current DOM can do that.
export function createSwitcherGestureReporter(
  send: (publication: SwitcherGesturePublication) => void,
): SwitcherGestureReporter {
  let currentSession = 0;
  let sequence = 0;
  return (session, stateRevision, regions) => {
    if (session <= 0 || stateRevision <= 0 || session < currentSession) return;
    if (session !== currentSession) {
      currentSession = session;
      sequence = 0;
    }
    send({ session, stateRevision, sequence: ++sequence, regions });
  };
}

type Box = { left: number; top: number; right: number; bottom: number };
function intersect(a: Box, b: Box): Box | null {
  const box = {
    left: Math.max(a.left, b.left),
    top: Math.max(a.top, b.top),
    right: Math.min(a.right, b.right),
    bottom: Math.min(a.bottom, b.bottom),
  };
  return Object.values(box).every(Number.isFinite) && box.right > box.left && box.bottom > box.top
    ? box
    : null;
}
function subtract(box: Box, control: Box): Box[] {
  const cut = intersect(box, control);
  if (!cut) return [box];
  return [
    { ...box, bottom: cut.top },
    { ...box, top: cut.bottom },
    { left: box.left, right: cut.left, top: cut.top, bottom: cut.bottom },
    { left: cut.right, right: box.right, top: cut.top, bottom: cut.bottom },
  ].filter((part) => part.right > part.left && part.bottom > part.top);
}

// client dimensions exclude borders and traditional scrollbars. Bounding
// rectangles alone include both, even when a card continues behind them.
function clientBox(element: HTMLElement): Box {
  const bounds = element.getBoundingClientRect();
  const left = bounds.left + element.clientLeft;
  const top = bounds.top + element.clientTop;
  return { left, top, right: left + element.clientWidth, bottom: top + element.clientHeight };
}

function collectRegions(
  panel: HTMLElement,
  entries: ReadonlyArray<Entry>,
): SwitcherGestureRegion[] {
  // The window switcher enters with a translated/scaled fade. Taking one
  // rectangle during that animation would give native input stale hit areas.
  // End/cancel notifications republish the final settled presentation.
  if (
    panel
      .getAnimations?.()
      .some((animation) => animation.playState === "running" || animation.playState === "paused")
  )
    return [];
  const eligible = new Map(entries.map((entry) => [entry.windowId, entry.appId]));
  const viewport = { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight };
  const panelClip = intersect(clientBox(panel), viewport);
  if (!panelClip) return [];
  const regions: SwitcherGestureRegion[] = [];
  for (const card of panel.querySelectorAll<HTMLElement>("[data-switcher-gesture-window]")) {
    const windowId = Number(card.dataset.switcherGestureWindow);
    const appId = Number(card.dataset.switcherGestureApp);
    if (windowId <= 0 || appId <= 0 || eligible.get(windowId) !== appId) continue;
    let box = intersect(card.getBoundingClientRect(), panelClip);
    for (
      let ancestor: HTMLElement | null = card;
      box && ancestor && ancestor !== panel;
      ancestor = ancestor.parentElement
    ) {
      const style = getComputedStyle(ancestor);
      if (
        style.display === "none" ||
        style.visibility === "hidden" ||
        style.visibility === "collapse"
      ) {
        box = null;
        break;
      }
      if (ancestor === card) continue;
      const clipX = /auto|scroll|hidden|clip/.test(style.overflowX || style.overflow);
      const clipY = /auto|scroll|hidden|clip/.test(style.overflowY || style.overflow);
      if (clipX || clipY) {
        const clip = clientBox(ancestor);
        box = intersect(box, {
          left: clipX ? clip.left : box.left,
          right: clipX ? clip.right : box.right,
          top: clipY ? clip.top : box.top,
          bottom: clipY ? clip.bottom : box.bottom,
        });
      }
    }
    if (!box) continue;
    let parts = [box];
    for (const control of card.querySelectorAll<HTMLElement>("[data-switcher-gesture-exclude]")) {
      const bounds = control.getBoundingClientRect();
      if (bounds.width <= 0 || bounds.height <= 0) continue;
      // Browser hit testing rounds fractional edges to device pixels. Leave a
      // CSS pixel around controls so the native owner cannot steal their edge.
      const excluded = {
        left: bounds.left - 1,
        top: bounds.top - 1,
        right: bounds.right + 1,
        bottom: bounds.bottom + 1,
      };
      parts = parts.flatMap((part) => subtract(part, excluded));
    }
    // Reserve whole cards; truncation must never reintroduce an excluded hit area.
    if (regions.length + parts.length > 256) break;
    for (const part of parts)
      regions.push({
        windowId,
        appId,
        bounds: { x: part.left, y: part.top, w: part.right - part.left, h: part.bottom - part.top },
      });
  }
  return regions;
}

// Geometry only. Native AppKit owns and qualifies the input stream; DOM wheel
// and drag events deliberately have no gesture recognizer.
export function useSwitcherGestureRegions(
  session: number,
  stateRevision: number,
  entries: ReadonlyArray<Entry>,
  active: boolean,
  report?: SwitcherGestureReporter,
) {
  const node = useRef<HTMLElement | null>(null);
  const last = useRef<{
    session: number;
    revision: number;
    signature: string;
    report: SwitcherGestureReporter;
  } | null>(null);
  const ref = useCallback((element: HTMLElement | null) => {
    node.current = element;
  }, []);
  // Run after every committed layout, including image arrival, style changes,
  // displayed gallery replacement and selection changes without a new revision.
  useLayoutEffect(() => {
    if (!report || session <= 0 || stateRevision <= 0) return;
    const panel = node.current;
    let mounted = true;
    const publish = () => {
      if (!mounted) return;
      const regions = active && panel ? collectRegions(panel, entries) : [];
      const signature = JSON.stringify(regions);
      if (
        last.current?.session === session &&
        last.current.revision === stateRevision &&
        last.current.signature === signature &&
        last.current.report === report
      )
        return;
      if (last.current && last.current.session !== session) {
        last.current.report(last.current.session, last.current.revision, []);
      }
      last.current = { session, revision: stateRevision, signature, report };
      report(session, stateRevision, regions);
    };
    publish();
    if (!active || !panel) return;
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(publish);
    observer?.observe(panel);
    for (const element of panel.querySelectorAll<HTMLElement>(
      "[data-switcher-gesture-window], [data-switcher-gesture-exclude]",
    )) {
      observer?.observe(element);
      if (element.parentElement) observer?.observe(element.parentElement);
    }
    panel.addEventListener("scroll", publish, true);
    panel.addEventListener("transitionend", publish);
    panel.addEventListener("animationstart", publish);
    panel.addEventListener("animationend", publish);
    panel.addEventListener("animationcancel", publish);
    window.addEventListener("resize", publish);
    return () => {
      mounted = false;
      observer?.disconnect();
      panel.removeEventListener("scroll", publish, true);
      panel.removeEventListener("transitionend", publish);
      panel.removeEventListener("animationstart", publish);
      panel.removeEventListener("animationend", publish);
      panel.removeEventListener("animationcancel", publish);
      window.removeEventListener("resize", publish);
    };
  });
  useLayoutEffect(
    () => () => {
      if (last.current) {
        last.current.report(last.current.session, last.current.revision, []);
        last.current = null;
      }
    },
    [],
  );
  return ref;
}

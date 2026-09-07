import { type RefObject, useLayoutEffect } from "react";
import {
  fitMagnification,
  magnificationTargets,
  magnificationTransforms,
  springStep,
} from "./magnification";

interface Options {
  enabled: boolean;
  scale: number;
  reach: number;
  iconPx: number;
  vertical: boolean;
  owner: string;
  itemsKey: string;
}
export function useMagnification(
  ref: RefObject<HTMLUListElement | null>,
  { enabled, scale, reach, iconPx, vertical, owner, itemsKey }: Options,
) {
  useLayoutEffect(() => {
    const node = ref.current;
    if (!node) return;
    const buttons = Array.from(node.querySelectorAll<HTMLButtonElement>(".ot-launcher-app"));
    const visuals = buttons.map((b) => b.querySelector<HTMLElement>(".ot-launcher-visual"));
    let states = buttons.map(() => ({ scale: 1, velocity: 0 })),
      targets = buttons.map(() => 1);
    let raf = 0,
      previous: number | null = null,
      pointer: number | null = null;
    const media = window.matchMedia?.("(prefers-reduced-motion: reduce)");
    const active = () =>
      enabled && scale > 1 && !media?.matches && document.visibilityState === "visible";
    let boxes: Array<{ start: number; end: number }> = [];
    let viewport = { start: 0, end: 0 };
    const draw = () => {
      const transforms = fitMagnification(
        magnificationTransforms(
          states.map((s) => s.scale),
          iconPx,
          scale,
          reach,
        ),
        boxes,
        viewport,
      );
      visuals.forEach((visual, i) => {
        if (visual)
          visual.style.transform = `translate${vertical ? "Y" : "X"}(${transforms[i].shift}px) scale(${transforms[i].scale})`;
      });
    };
    const reset = () => {
      if (raf) cancelAnimationFrame(raf);
      raf = 0;
      previous = null;
      pointer = null;
      states = buttons.map(() => ({ scale: 1, velocity: 0 }));
      targets = buttons.map(() => 1);
      draw();
    };
    const tick = (now: number) => {
      raf = 0;
      if (!active()) {
        reset();
        return;
      }
      const elapsed = previous === null ? 0 : (now - previous) / 1000;
      previous = now;
      states = states.map((s, i) => springStep(s, targets[i], elapsed, scale));
      draw();
      if (states.some((s, i) => s.scale !== targets[i] || s.velocity !== 0))
        raf = requestAnimationFrame(tick);
      else previous = null;
    };
    const schedule = () => {
      if (active() && !raf) raf = requestAnimationFrame(tick);
    };
    const update = () => {
      if (!active()) {
        reset();
        return;
      }
      const extent = node.getBoundingClientRect();
      viewport = vertical
        ? { start: extent.top, end: extent.bottom }
        : { start: extent.left, end: extent.right };
      boxes = buttons.map((b) => {
        const r = b.getBoundingClientRect();
        return vertical ? { start: r.top, end: r.bottom } : { start: r.left, end: r.right };
      });
      const centers = boxes.map((b) => (b.start + b.end) / 2);
      targets = magnificationTargets(centers, pointer, scale, reach).map((value, i) =>
        buttons[i].disabled ? 1 : value,
      );
      schedule();
    };
    const move = (event: PointerEvent) => {
      const position = vertical ? event.clientY : event.clientX;
      if (!Number.isFinite(position)) return;
      pointer = position;
      update();
    };
    const leave = () => {
      pointer = null;
      update();
    };
    const visibility = () => {
      if (!active()) reset();
    };
    const resize =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(() => {
            if (pointer !== null) update();
          });
    resize?.observe(node);
    node.addEventListener("pointermove", move);
    node.addEventListener("pointerleave", leave);
    node.addEventListener("scroll", update, { passive: true });
    document.addEventListener("visibilitychange", visibility);
    media?.addEventListener("change", visibility);
    draw();
    return () => {
      reset();
      resize?.disconnect();
      node.removeEventListener("pointermove", move);
      node.removeEventListener("pointerleave", leave);
      node.removeEventListener("scroll", update);
      document.removeEventListener("visibilitychange", visibility);
      media?.removeEventListener("change", visibility);
    };
  }, [ref, enabled, scale, reach, iconPx, vertical, owner, itemsKey]);
}

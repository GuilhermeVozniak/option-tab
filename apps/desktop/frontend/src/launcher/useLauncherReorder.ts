import {
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import type { LauncherPresentationItem } from "../lib/types";
import { type LauncherItemMutation, reorderable, reorderMutation } from "./reorder";

interface Options {
  enabled: boolean;
  owner: string;
  items: LauncherPresentationItem[];
  vertical: boolean;
  perform: (mutation: LauncherItemMutation) => void;
}
export function useLauncherReorder(ref: RefObject<HTMLUListElement | null>, options: Options) {
  const current = useRef(options);
  current.current = options;
  const draft = useRef<{
    id: string;
    pointer: number;
    x: number;
    y: number;
    owner: string;
    dragging: boolean;
    target: LauncherItemMutation | null;
  } | null>(null);
  const suppress = useRef(false);
  const [target, setTarget] = useState<LauncherItemMutation | null>(null);
  useLayoutEffect(() => {
    const clear = () => {
      if (draft.current?.dragging) suppress.current = true;
      draft.current = null;
      setTarget(null);
    };
    clear();
    const move = (event: PointerEvent) => {
      const d = draft.current,
        o = current.current;
      if (!d || event.pointerId !== d.pointer) return;
      if (!o.enabled || o.owner !== d.owner) {
        clear();
        return;
      }
      if (!d.dragging && Math.hypot(event.clientX - d.x, event.clientY - d.y) < 6) return;
      d.dragging = true;
      event.preventDefault();
      let next: LauncherItemMutation | null = null;
      for (const node of ref.current?.querySelectorAll<HTMLElement>("[data-reorder-target]") ??
        []) {
        const r = node.getBoundingClientRect();
        if (
          event.clientX < r.left ||
          event.clientX > r.right ||
          event.clientY < r.top ||
          event.clientY > r.bottom
        )
          continue;
        const ratio = o.vertical
          ? (event.clientY - r.top) / r.height
          : (event.clientX - r.left) / r.width;
        const zone = ratio < 0.3 ? "before" : ratio > 0.7 ? "after" : "group";
        next = reorderMutation(o.items, d.id, node.dataset.reorderTarget!, zone);
        break;
      }
      d.target = next;
      setTarget(next);
    };
    const up = (event: PointerEvent) => {
      const d = draft.current;
      if (!d || event.pointerId !== d.pointer) return;
      if (d.dragging) move(event);
      const o = current.current;
      const mutation = d.dragging && d.owner === o.owner && o.enabled ? d.target : null;
      clear();
      if (mutation) o.perform(mutation);
    };
    const cancel = () => clear();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") clear();
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", key);
    return () => {
      clear();
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("keydown", key);
    };
  }, [ref, options.owner, options.enabled]);
  const start = (event: ReactPointerEvent, id: string) => {
    const o = current.current,
      all = o.items.flatMap((item) => [item, ...(item.members ?? [])]),
      item = all.find((x) => x.id === id);
    if (
      !o.enabled ||
      event.button !== 0 ||
      event.isPrimary === false ||
      !item ||
      !reorderable(item)
    )
      return;
    suppress.current = false;
    draft.current = {
      id,
      pointer: event.pointerId,
      x: event.clientX,
      y: event.clientY,
      owner: o.owner,
      dragging: false,
      target: null,
    };
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      /* optional in test DOM */
    }
  };
  const suppressClick = (event: ReactMouseEvent) => {
    if (suppress.current) {
      suppress.current = false;
      event.preventDefault();
      event.stopPropagation();
    }
  };
  return { start, target, suppressClick };
}

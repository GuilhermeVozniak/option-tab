import { type PointerEvent as ReactPointerEvent, useLayoutEffect, useRef, useState } from "react";
import type { LauncherPresentation } from "../lib/types";

export type LauncherAutoHidePhase = "begin" | "renew" | "end";
export type LauncherAutoHideHold = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  sequence: number,
  phase: LauncherAutoHidePhase,
) => Promise<void>;

export function useLauncherAutoHideHold(
  presentation: LauncherPresentation,
  menuOpen: boolean,
  send: LauncherAutoHideHold | undefined,
  cancelMenus: () => void,
  itemsKey = "",
) {
  const current = useRef({ presentation, send, cancelMenus });
  current.current = { presentation, send, cancelMenus };
  const sequence = useRef(0);
  const outbound = useRef<{
    pending: boolean;
    next: {
      send: LauncherAutoHideHold;
      args: [number, string, number, number, LauncherAutoHidePhase];
    } | null;
  }>({ pending: false, next: null });
  const pointer = useRef<number | null>(null);
  const [pressedOwner, setPressedOwner] = useState("");
  const owner = `${presentation.epoch}:${presentation.displayUUID}:${presentation.session}:${presentation.profileID}:${JSON.stringify(presentation.bounds)}:${itemsKey}`;
  const menu = useRef({ owner, open: false, admitted: "" });
  if (menu.current.owner !== owner) menu.current.admitted = "";
  if (!menuOpen) menu.current.admitted = "";
  else if (!menu.current.open) menu.current.admitted = owner;
  menu.current.owner = owner;
  menu.current.open = menuOpen;
  useLayoutEffect(() => {
    const cancel = () => {
      pointer.current = null;
      setPressedOwner("");
      current.current.cancelMenus();
    };
    const up = (event: PointerEvent) => {
      if (pointer.current !== event.pointerId) return;
      pointer.current = null;
      setPressedOwner("");
    };
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") cancel();
    };
    const visibility = () => {
      if (document.hidden) cancel();
    };
    cancel();
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("blur", cancel);
    window.addEventListener("keydown", key);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      pointer.current = null;
      setPressedOwner("");
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("blur", cancel);
      window.removeEventListener("keydown", key);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [owner]);
  const active =
    presentation.visible &&
    (pressedOwner === owner || (menuOpen && menu.current.admitted === owner));
  useLayoutEffect(() => {
    if (!active || !send) return;
    let admitted = current.current.presentation;
    const flush = () => {
      const queue = outbound.current;
      if (queue.pending || !queue.next) return;
      const task = queue.next;
      queue.next = null;
      queue.pending = true;
      const [epoch, display, session, revision, held] = task.args;
      const finished = () => {
        queue.pending = false;
        flush();
      };
      try {
        void Promise.resolve(
          task.send(epoch, display, session, revision, ++sequence.current, held),
        ).then(finished, finished);
      } catch {
        finished();
      }
    };
    const publish = (held: LauncherAutoHidePhase) => {
      const latest = current.current.presentation;
      if (
        latest.epoch === admitted.epoch &&
        latest.session === admitted.session &&
        latest.displayUUID === admitted.displayUUID
      )
        admitted = latest;
      // One in-flight request plus the latest state, even if the bridge stalls.
      // Cancellation replaces queued renewals; the native lease also expires.
      const queued = outbound.current.next;
      if (
        held === "renew" &&
        queued?.args[4] === "begin" &&
        queued.args[0] === admitted.epoch &&
        queued.args[1] === admitted.displayUUID &&
        queued.args[2] === admitted.session
      )
        held = "begin";
      outbound.current.next = {
        send,
        args: [admitted.epoch, admitted.displayUUID, admitted.session, admitted.revision, held],
      };
      flush();
    };
    publish("begin");
    const timer = window.setInterval(() => publish("renew"), 500);
    return () => {
      window.clearInterval(timer);
      publish("end");
    };
  }, [owner, active, send]);
  return {
    onPointerDownCapture: (event: ReactPointerEvent) => {
      if (event.isPrimary === false || event.button < 0 || event.button > 2) return;
      pointer.current = event.pointerId;
      setPressedOwner(owner);
    },
    onLostPointerCapture: () => {
      pointer.current = null;
      setPressedOwner("");
    },
  };
}

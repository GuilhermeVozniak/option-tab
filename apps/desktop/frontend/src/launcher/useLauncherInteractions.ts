import { useCallback, useEffect, useRef, useState } from "react";
import type { LauncherInteractionState, LauncherPresentation } from "../lib/types";

export interface LauncherInteractionTransport {
  getState(session: number): Promise<LauncherInteractionState | null>;
  subscribe(handler: (state: LauncherInteractionState) => void): () => void;
  subscribeError?(handler: (state: LauncherInteractionState) => void): () => void;
  keyboard(
    epoch: number,
    displayUUID: string,
    session: number,
    presentationRevision: number,
    admission: number,
    enabled: boolean,
  ): Promise<void>;
  letter(
    epoch: number,
    displayUUID: string,
    session: number,
    presentationRevision: number,
    admission: number,
    sequence: number,
    text: string,
    modifiers: number,
    composing: boolean,
  ): Promise<void>;
  activate(
    epoch: number,
    displayUUID: string,
    session: number,
    presentationRevision: number,
    admission: number,
    sequence: number,
  ): Promise<void>;
  reorderTarget?(
    epoch: number,
    displayUUID: string,
    session: number,
    presentationRevision: number,
    admission: number,
    sequence: number,
    itemID: string,
  ): Promise<void>;
}

const scopeKey = (p: LauncherPresentation) => `${p.epoch}:${p.displayUUID}:${p.session}`;

export function useLauncherInteractions(
  presentation: LauncherPresentation,
  transport?: LauncherInteractionTransport,
) {
  const [state, setState] = useState<LauncherInteractionState | null>(null);
  const [error, setError] = useState("");
  const stateRef = useRef<LauncherInteractionState | null>(null);
  const pending = useRef<LauncherInteractionState | null>(null);
  const terminalAdmission = useRef(0);
  const presentationRevision = useRef(presentation.revision);
  presentationRevision.current = presentation.revision;
  const actionSequence = useRef(0);
  const owner = scopeKey(presentation);

  useEffect(() => {
    stateRef.current = null;
    pending.current = null;
    terminalAdmission.current = 0;
    actionSequence.current = 0;
    setState(null);
    setError("");
    if (!transport) return;
    let active = true;
    const accept = (next: LauncherInteractionState, asError = false) => {
      if (
        !active ||
        next.admission <= terminalAdmission.current ||
        next.epoch !== presentation.epoch ||
        next.displayUUID !== presentation.displayUUID ||
        next.session !== presentation.session
      )
        return;
      const rendered = stateRef.current;
      const queued = pending.current;
      const current =
        !rendered ||
        (queued &&
          (queued.admission > rendered.admission ||
            (queued.admission === rendered.admission && queued.sequence > rendered.sequence)))
          ? queued
          : rendered;
      if (
        current &&
        (next.admission < current.admission ||
          (next.admission === current.admission && next.sequence <= current.sequence))
      )
        return;
      if (!next.visible) {
        terminalAdmission.current = next.admission;
        stateRef.current = null;
        pending.current = null;
        setState(null);
        return;
      }
      if (next.presentationRevision > presentationRevision.current) {
        pending.current = next;
        return;
      }
      pending.current = null;
      const previousAdmission = stateRef.current?.admission;
      stateRef.current = next;
      actionSequence.current =
        previousAdmission === next.admission
          ? Math.max(actionSequence.current, next.sequence)
          : next.sequence;
      setState(next);
      setError(asError || next.reason === "failed" || next.reason === "busy" ? next.reason : "");
    };
    const off = transport.subscribe((next) => accept(next));
    const offError = transport.subscribeError?.((next) => accept(next, true));
    void transport
      .getState(presentation.session)
      .then((next) => next && accept(next))
      .catch((reason) => {
        if (active) setError(String(reason).slice(0, 240));
      });
    return () => {
      active = false;
      off();
      offError?.();
    };
  }, [owner, presentation.displayUUID, presentation.epoch, presentation.session, transport]);

  useEffect(() => {
    const next = pending.current;
    if (
      !next ||
      next.presentationRevision > presentation.revision ||
      next.admission <= terminalAdmission.current
    )
      return;
    pending.current = null;
    const previousAdmission = stateRef.current?.admission;
    stateRef.current = next;
    actionSequence.current =
      previousAdmission === next.admission
        ? Math.max(actionSequence.current, next.sequence)
        : next.sequence;
    setState(next);
    setError(next.reason === "failed" || next.reason === "busy" ? next.reason : "");
  }, [presentation.revision]);

  const invoke = useCallback(
    (run: (admitted: LauncherInteractionState, sequence: number) => Promise<void>) => {
      const admitted = stateRef.current;
      if (!admitted || !transport || admitted.admission <= terminalAdmission.current) return;
      const sequence = ++actionSequence.current;
      setError("");
      void run(admitted, sequence).catch((reason) => {
        if (stateRef.current === admitted && admitted.admission > terminalAdmission.current)
          setError(String(reason).slice(0, 240));
      });
    },
    [transport],
  );
  const args = (admitted: LauncherInteractionState) =>
    [
      admitted.epoch,
      admitted.displayUUID,
      admitted.session,
      presentation.revision,
      admitted.admission,
    ] as const;
  return {
    state,
    error,
    setKeyboardMode: (enabled: boolean) =>
      invoke((admitted) => transport!.keyboard(...args(admitted), enabled)),
    commitLetter: (text: string, modifiers = 0, composing = false) =>
      invoke((admitted, sequence) =>
        transport!.letter(...args(admitted), sequence, text, modifiers, composing),
      ),
    activateSelection: () =>
      invoke((admitted, sequence) => transport!.activate(...args(admitted), sequence)),
    setReorderTarget: (itemID: string) =>
      transport?.reorderTarget
        ? invoke((admitted, sequence) =>
            transport.reorderTarget!(...args(admitted), sequence, itemID),
          )
        : undefined,
  };
}

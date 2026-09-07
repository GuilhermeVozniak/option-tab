import { useCallback, useEffect, useRef, useState } from "react";
import { DockPanelView } from "../dock/DockPanelView";
import { automationPreview, onAutomationPreviewEvents } from "../lib/automation-preview-bridge";
import type { Translate } from "../lib/i18n";
import type { AutomationPreviewState } from "../lib/types";

export function AutomationPreviewRoute({
  session,
  t = (text) => text,
}: {
  session: number;
  t?: Translate;
}) {
  const [state, setState] = useState<AutomationPreviewState | null>(null);
  const [frames, setFrames] = useState<Record<string, string>>({});
  const revision = useRef(0);
  const frameSequence = useRef(0);
  const retired = useRef(false);
  const stateRef = useRef(state);
  const accept = useCallback(
    (next: AutomationPreviewState) => {
      if (next.session !== session || retired.current || next.revision < revision.current) return;
      const allowed = new Set(next.entries.map((entry) => String(entry.windowId)));
      const snapshotFrames = Object.fromEntries(
        Object.entries(next.frames ?? {}).filter(([windowID]) => allowed.has(windowID)),
      );
      if (next.revision > revision.current) {
        frameSequence.current = next.frameSequence ?? 0;
        setFrames(snapshotFrames);
      } else if (next.frameSequence !== undefined && next.frameSequence >= frameSequence.current) {
        frameSequence.current = next.frameSequence;
        setFrames(snapshotFrames);
      } else {
        setFrames((old) =>
          Object.fromEntries(Object.entries(old).filter(([windowID]) => allowed.has(windowID))),
        );
      }
      revision.current = next.revision;
      stateRef.current = next;
      setState(next);
    },
    [session],
  );
  useEffect(() => {
    let mounted = true;
    const off = onAutomationPreviewEvents({
      update: accept,
      hide: (eventSession, eventRevision) => {
        if (eventSession !== session || eventRevision < revision.current) return;
        retired.current = true;
        revision.current = eventRevision;
        setState(null);
        setFrames({});
      },
      frames: (eventSession, eventRevision, sequence, nextFrames) => {
        if (
          eventSession !== session ||
          eventRevision !== revision.current ||
          sequence <= frameSequence.current ||
          retired.current
        )
          return;
        frameSequence.current = sequence;
        setFrames((old) => {
          const allowed = new Set(stateRef.current?.entries.map((entry) => String(entry.windowId)));
          const admitted = Object.fromEntries(
            Object.entries(nextFrames).filter(([windowID]) => allowed?.has(windowID)),
          );
          return { ...old, ...admitted };
        });
      },
    });
    void automationPreview
      .state(session)
      .then((snapshot) => {
        if (mounted && snapshot) accept(snapshot);
      })
      .catch(() => {});
    return () => {
      mounted = false;
      off();
    };
  }, [accept, session]);
  stateRef.current = state;
  const request = useCallback(async (atRevision: number, operation: () => Promise<unknown>) => {
    try {
      await operation();
    } catch (error) {
      if (!retired.current && revision.current === atRevision)
        setState((old) =>
          old?.revision === atRevision
            ? { ...old, error: error instanceof Error ? error.message : String(error) }
            : old,
        );
    }
  }, []);
  if (!state?.open) return null;
  const visible = {
    ...state,
    contentKind: "windows" as const,
    entries: state.entries.map((entry) => ({
      ...entry,
      thumbnail: frames[String(entry.windowId)] ?? entry.thumbnail,
    })),
  };
  return (
    <DockPanelView
      state={visible}
      item={null}
      title={state.title}
      onClose={() =>
        void request(state.revision, () => automationPreview.close(session, state.revision))
      }
      nativeHeader
      handlers={{
        onSelectWindow: (s, windowID) => {
          if (s === session && state.revision === revision.current)
            void request(state.revision, () =>
              automationPreview.select(s, state.revision, windowID),
            );
        },
        onFocusWindow: (s, windowID) =>
          void request(state.revision, () =>
            automationPreview.action(s, state.revision, "focus", windowID, false),
          ),
        onAction: (s, kind, windowID) => {
          if (!["close", "minimize", "fullscreen"].includes(kind)) return;
          const entry = state.entries.find((candidate) => candidate.windowId === windowID);
          void request(state.revision, () =>
            automationPreview.action(
              s,
              state.revision,
              kind,
              windowID,
              kind === "fullscreen" ? !entry?.fullscreen : false,
            ),
          );
        },
        onSize: (s, width, height) =>
          void request(state.revision, () =>
            automationPreview.size(s, state.revision, width, height),
          ),
      }}
      t={t}
    />
  );
}

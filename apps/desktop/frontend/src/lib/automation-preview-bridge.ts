import { Events } from "@wailsio/runtime";
import {
  CloseAutomationPreview,
  GetAutomationPreviewState,
  PerformAutomationPreviewAction,
  SelectAutomationPreview,
  SetAutomationPreviewSize,
} from "../../bindings/option-tab/app.js";
import type { AutomationPreviewState } from "./types";

export const automationPreview = {
  state: (session: number) =>
    GetAutomationPreviewState(session) as Promise<AutomationPreviewState | null>,
  select: (session: number, revision: number, windowID: number) =>
    SelectAutomationPreview(session, revision, windowID),
  action: (
    session: number,
    revision: number,
    kind: string,
    windowID: number,
    fullscreen: boolean,
  ) => PerformAutomationPreviewAction(session, revision, kind, windowID, fullscreen),
  size: (session: number, revision: number, width: number, height: number) =>
    SetAutomationPreviewSize(session, revision, width, height),
  close: (session: number, revision: number) => CloseAutomationPreview(session, revision),
};

export function onAutomationPreviewEvents(handlers: {
  update: (state: AutomationPreviewState) => void;
  hide: (session: number, revision: number) => void;
  frames: (
    session: number,
    revision: number,
    sequence: number,
    frames: Record<string, string>,
  ) => void;
}) {
  const off = [
    Events.On("automation-preview:update", (event) =>
      handlers.update(event.data as AutomationPreviewState),
    ),
    Events.On("automation-preview:hide", (event) => {
      const data = event.data as { session: number; revision: number };
      handlers.hide(data.session, data.revision);
    }),
    Events.On("automation-preview:frames", (event) => {
      const data = event.data as {
        session: number;
        revision: number;
        sequence: number;
        frames: Record<string, string>;
      };
      handlers.frames(data.session, data.revision, data.sequence, data.frames);
    }),
  ];
  return () => off.forEach((unsubscribe) => unsubscribe());
}

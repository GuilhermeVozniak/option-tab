import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type { WindowActionResult } from "./bridge";
import type { DockPointer, DockViewState, WindowAction } from "./types";

export type { DockPointer } from "./types";

export const dock = {
  state: async () => (await AppService.GetDockState()) as DockViewState | null,
  select: (session: number, id: number) => AppService.SelectDockWindow(session, id),
  focus: async (session: number, id: number, appId: number) =>
    (await AppService.FocusDockWindow(session, id, appId)) as WindowActionResult,
  action: (session: number, kind: WindowAction, id: number, appId: number) =>
    AppService.PerformDockAction(session, kind, id, appId) as Promise<WindowActionResult>,
  size: (session: number, w: number, h: number) => AppService.SetDockPanelSize(session, w, h),
};
export interface DockEvents {
  show: (state: DockViewState) => void;
  update: (state: DockViewState) => void;
  hide: (session: number, revision: number) => void;
  frames: (session: number, frames: Record<string, string>) => void;
  error: (session: number, revision: number, message: string) => void;
  pointer: (pointer: DockPointer) => void;
}
export function onDockEvent(h: DockEvents) {
  const readSession = (data: unknown) => Number((data as { session?: number })?.session ?? 0);
  const off = [
    Events.On("dock:show", (e) => h.show(e.data as DockViewState)),
    Events.On("dock:update", (e) => h.update(e.data as DockViewState)),
    Events.On("dock:hide", (e) => {
      const d = e.data as { session?: number; revision?: number };
      h.hide(readSession(d), Number(d?.revision ?? 0));
    }),
    Events.On("dock:frames", (e) => {
      const d = e.data as { session: number; frames: Record<string, string> };
      h.frames(d.session, d.frames);
    }),
    Events.On("dock:error", (e) => {
      const d = e.data as { session: number; message: string };
      h.error(d.session, Number((d as { revision?: number }).revision ?? 0), d.message);
    }),
    Events.On("dock:pointer", (e) => h.pointer(e.data as DockPointer)),
  ];
  return () => off.forEach((fn) => fn());
}

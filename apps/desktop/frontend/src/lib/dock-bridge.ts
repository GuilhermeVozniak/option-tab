import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type { WindowActionResult } from "./bridge";
import type { DockPointer, DockViewState, WindowAction } from "./types";

export type { DockPointer } from "./types";

export const dock = {
  state: async () => (await AppService.GetDockState()) as DockViewState | null,
  select: (session: number, id: number) => AppService.SelectDockWindow(session, id),
  selectContent: (session: number, revision: number, kind: string) =>
    AppService.SelectDockContent(session, revision, kind),
  focus: async (session: number, id: number, appId: number) =>
    (await AppService.FocusDockWindow(session, id, appId)) as WindowActionResult,
  action: (session: number, kind: WindowAction, id: number, appId: number) =>
    AppService.PerformDockAction(session, kind, id, appId) as Promise<WindowActionResult>,
  size: (session: number, w: number, h: number) => AppService.SetDockPanelSize(session, w, h),
  regions: (
    session: number,
    revision: number,
    regions: Array<{
      windowId: number;
      appId: number;
      bounds: { x: number; y: number; w: number; h: number };
    }>,
  ) => AppService.SetDockPreviewRegions(session, revision, regions),
  beginDrag: (
    session: number,
    gesture: number,
    windowId: number,
    appId: number,
    pointerX: number,
    pointerY: number,
    grabX: number,
    grabY: number,
  ) =>
    AppService.BeginDockPreviewDrag(
      session,
      gesture,
      windowId,
      appId,
      pointerX,
      pointerY,
      grabX,
      grabY,
    ),
  cancelDrag: (session: number, gesture: number) =>
    AppService.CancelDockPreviewDrag(session, gesture),
  folderSort: (
    session: number,
    revision: number,
    field: string,
    direction: string,
    foldersFirst: boolean,
  ) => AppService.SetDockFolderSort(session, revision, field, direction, foldersFirst),
  requestFolderAccess: (session: number, revision: number) =>
    AppService.RequestDockFolderAccess(session, revision),
  cancelFolderAccess: (session: number, revision: number) =>
    AppService.CancelDockFolderAccess(session, revision),
  openFolderEntry: (session: number, revision: number, itemID: string) =>
    AppService.OpenDockFolderEntry(session, revision, itemID),
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

export function onDockInputStatus(handler: (revision: number, message: string) => void) {
  return Events.On("dock:input-status", (event) => {
    const data = event.data as { revision?: number; message?: string };
    handler(Number(data.revision ?? 0), String(data.message ?? ""));
  });
}

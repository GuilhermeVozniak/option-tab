import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type { DockLockDisplay, DockMonitorLockState } from "./types";

export const dockLock = {
  state: async () => mapState(await AppService.GetDockMonitorLockState()),
  displays: async () => mapDisplays(await AppService.GetDockMonitorLockDisplays()),
  place: (session: number, revision: number, generation: number) =>
    AppService.PlaceDockOnSelectedMonitor(session, revision, generation),
  cancel: () => AppService.CancelDockPlacement(),
  onState: (handler: (state: DockMonitorLockState) => void) =>
    Events.On("dock:monitor-lock", (event) => handler(mapState(event.data))),
};

function mapDisplays(values: unknown[]): DockLockDisplay[] {
  return values.map((value: any) => ({
    uuid: value.uuid,
    id: value.id,
    name: value.name,
    bounds: {
      x: value.bounds?.x ?? value.bounds?.X ?? 0,
      y: value.bounds?.y ?? value.bounds?.Y ?? 0,
      w: value.bounds?.w ?? value.bounds?.W ?? 0,
      h: value.bounds?.h ?? value.bounds?.H ?? 0,
    },
    scale: value.scale,
    main: value.main,
    mirrored: value.mirrored,
  }));
}
function mapState(value: any): DockMonitorLockState {
  return {
    session: value.session,
    revision: value.revision,
    generation: value.generation,
    sequence: value.sequence,
    observedAtMs: value.observedAtMs,
    status: value.status,
    reason: value.reason,
    targetUUID: value.targetUUID,
    actualUUID: value.actualUUID,
    edge: value.edge,
    displays: mapDisplays(value.displays ?? []),
    placementAvailable: Boolean(value.placementAvailable),
  };
}

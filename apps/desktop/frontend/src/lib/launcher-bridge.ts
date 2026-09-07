import { Events } from "@wailsio/runtime";
import {
  ActivateLauncherItem,
  GetLauncherState,
  GetLauncherStatus,
  UseNativeDock,
} from "../../bindings/option-tab/app.js";
import type { LauncherPresentation, LauncherStatus } from "./types";

export const launcher = {
  state: (session: number) =>
    GetLauncherState(session) as unknown as Promise<LauncherPresentation | null>,
  status: () => GetLauncherStatus() as Promise<LauncherStatus>,
  activate: (
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ) => ActivateLauncherItem(epoch, displayUUID, session, revision, itemID),
  useNativeDock: () => UseNativeDock(),
};
export const onLauncherState = (handler: (state: LauncherPresentation) => void) =>
  Events.On("launcher:state", (event) => handler(event.data as LauncherPresentation));
export const onLauncherStatus = (handler: (status: LauncherStatus) => void) =>
  Events.On("launcher:status", (event) => handler(event.data as LauncherStatus));

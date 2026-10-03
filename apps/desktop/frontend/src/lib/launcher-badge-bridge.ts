import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type { LauncherBadgeState, LauncherBadgeTransport } from "./launcher-badge-types";

export const launcherBadges: LauncherBadgeTransport = {
  get: (session) => AppService.GetLauncherBadges(session) as Promise<LauncherBadgeState>,
  subscribe: (handler) =>
    Events.On("launcher:badges", (event) => handler(event.data as LauncherBadgeState)),
};

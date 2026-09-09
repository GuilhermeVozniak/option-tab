import { Events } from "@wailsio/runtime";
import {
  ActivateLauncherSelection,
  CommitLauncherLetter,
  GetLauncherInteractionCapabilities,
  GetLauncherInteractionState,
  SetLauncherKeyboardMode,
  SetLauncherReorderTarget,
} from "../../bindings/option-tab/app.js";
import type { LauncherInteractionTransport } from "../launcher/useLauncherInteractions";
import type { LauncherInteractionCapabilities, LauncherInteractionState } from "./types";

export const launcherInteractions: LauncherInteractionTransport = {
  getState: (session) =>
    GetLauncherInteractionState(session) as Promise<LauncherInteractionState | null>,
  subscribe: (handler) =>
    Events.On("launcher:interaction", (event) => handler(event.data as LauncherInteractionState)),
  subscribeError: (handler) =>
    Events.On("launcher:interaction-error", (event) =>
      handler(event.data as LauncherInteractionState),
    ),
  keyboard: (...args) => SetLauncherKeyboardMode(...args),
  letter: (...args) => CommitLauncherLetter(...args),
  activate: (...args) => ActivateLauncherSelection(...args),
  reorderTarget: (...args) => SetLauncherReorderTarget(...args),
};

export const getLauncherInteractionCapabilities = () =>
  GetLauncherInteractionCapabilities() as Promise<LauncherInteractionCapabilities>;

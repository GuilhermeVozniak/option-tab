import { SetLauncherAutoHideHold } from "../../bindings/option-tab/app.js";
import type { LauncherAutoHideHold } from "../launcher/useLauncherAutoHideHold";

export const setLauncherAutoHideHold: LauncherAutoHideHold = (...args) =>
  SetLauncherAutoHideHold(...args);

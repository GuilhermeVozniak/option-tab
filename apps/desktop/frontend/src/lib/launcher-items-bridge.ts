import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type {
  LauncherItem,
  LauncherItemIcon,
  LauncherItemSettings,
  LauncherItemStatus,
  LauncherReferenceView,
} from "./types";

export const launcherItemSettings = {
  load: (profileID: string) =>
    AppService.GetLauncherItemSettings(profileID) as Promise<LauncherItemSettings>,
  save: (profileID: string, revision: string, items: LauncherItem[]) =>
    AppService.SetLauncherItems(
      profileID,
      revision,
      items as Parameters<typeof AppService.SetLauncherItems>[2],
    ) as Promise<LauncherItemSettings>,
  chooseReference: (kind: "app" | "folder" | "file") =>
    AppService.ChooseLauncherItemReference(kind) as Promise<LauncherReferenceView>,
  relinkReference: (id: string) =>
    AppService.RelinkLauncherItemReference(id) as Promise<LauncherReferenceView>,
  cancelSelection: () => AppService.CancelLauncherItemSelection(),
  chooseIcon: () => AppService.ChooseLauncherItemIcon() as Promise<LauncherItemIcon>,
  getIcon: (id: string) => AppService.GetLauncherItemIcon(id) as Promise<LauncherItemIcon>,
  removeReference: (id: string) => AppService.RemoveUnusedLauncherReference(id),
  removeIcon: (id: string) => AppService.RemoveUnusedLauncherIcon(id),
  status: () => AppService.GetLauncherItemStatus() as Promise<LauncherItemStatus>,
  subscribe: onLauncherItemStatus,
};
export function onLauncherItemStatus(handler: (status: LauncherItemStatus) => void) {
  return Events.On("launcher:items", (event) => handler(event.data as LauncherItemStatus));
}

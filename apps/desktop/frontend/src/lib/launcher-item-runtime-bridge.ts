import { MutateLauncherItems, RelaunchLauncherItem } from "../../bindings/option-tab/app.js";

export const relaunchLauncherItem = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  itemID: string,
) => RelaunchLauncherItem(epoch, displayUUID, session, revision, itemID);

import type { LauncherItemMutation } from "../launcher/reorder";
export type LauncherMutateCommand = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  itemsRevision: string,
  mutation: LauncherItemMutation,
) => void;
export const mutateLauncherItems = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  itemsRevision: string,
  mutation: LauncherItemMutation,
) => MutateLauncherItems(epoch, displayUUID, session, revision, itemsRevision, mutation);

import { RelaunchLauncherItem } from "../../bindings/option-tab/app.js";

export const relaunchLauncherItem = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  itemID: string,
) => RelaunchLauncherItem(epoch, displayUUID, session, revision, itemID);

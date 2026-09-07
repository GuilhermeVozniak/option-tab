import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type { AutomationPreviewState, DockFolderState } from "./types";

export interface LauncherItemPanelState {
  session: number;
  revision: number;
  parentEpoch: number;
  parentSession: number;
  displayUUID: string;
  profileID: string;
  itemID: string;
  kind: "folder" | "windows";
  title: string;
  open: boolean;
  bounds: unknown;
  folder?: DockFolderState & { view: "list" | "grid" };
  windows?: AutomationPreviewState;
  error?: string;
}

export interface LauncherItemPanelTransport {
  getState(session: number): Promise<LauncherItemPanelState | null>;
  close(session: number, revision: number): Promise<void>;
  size(session: number, revision: number, width: number, height: number): Promise<void>;
  sort(
    session: number,
    revision: number,
    field: DockFolderState["sort"]["field"],
    direction: DockFolderState["sort"]["direction"],
    foldersFirst: boolean,
  ): Promise<void>;
  view(session: number, revision: number, view: "list" | "grid"): Promise<void>;
  open(session: number, revision: number, itemID: string): Promise<void>;
  selectWindow?(session: number, revision: number, windowID: number): Promise<void>;
  windowAction?(
    session: number,
    revision: number,
    kind: string,
    windowID: number,
    fullscreen: boolean,
  ): Promise<void>;
  subscribe(handlers: {
    update(state: LauncherItemPanelState): void;
    hide(value: { session: number; revision: number }): void;
    frames(value: {
      session: number;
      revision: number;
      sequence: number;
      frames: Record<string, string>;
    }): void;
  }): () => void;
}

export const showLauncherItemPanel = (
  epoch: number,
  displayUUID: string,
  session: number,
  revision: number,
  itemID: string,
) =>
  AppService.ShowLauncherItemPanel(epoch, displayUUID, session, revision, itemID).then(
    () => undefined,
  );

export const launcherItemPanel: LauncherItemPanelTransport = {
  getState: (session) =>
    AppService.GetLauncherItemPanelState(
      session,
    ) as unknown as Promise<LauncherItemPanelState | null>,
  close: AppService.CloseLauncherItemPanel,
  size: AppService.SetLauncherItemPanelSize,
  sort: AppService.SetLauncherFolderSort,
  view: AppService.SetLauncherFolderView,
  open: AppService.OpenLauncherFolderEntry,
  selectWindow: AppService.SelectLauncherWindow,
  windowAction: AppService.PerformLauncherWindowAction,
  subscribe: ({ update, hide, frames }) => {
    const offUpdate = Events.On("launcher-item:update", (event) =>
      update(event.data as LauncherItemPanelState),
    );
    const offHide = Events.On("launcher-item:hide", (event) =>
      hide(event.data as { session: number; revision: number }),
    );
    const offFrames = Events.On("launcher-item:frames", (event) =>
      frames(
        event.data as {
          session: number;
          revision: number;
          sequence: number;
          frames: Record<string, string>;
        },
      ),
    );
    return () => {
      offUpdate();
      offHide();
      offFrames();
    };
  },
};

import { Events } from "@wailsio/runtime";
import {
  CancelMediaLyricsImport,
  CloseMediaPanel,
  ConnectMediaProvider,
  GetMediaPermissions,
  GetMediaState,
  ImportMediaLyrics,
  PerformMediaAction,
  PinMediaPanel,
  ReloadMediaLyrics,
  RemoveMediaLyrics,
  SetMediaLyricsOffset,
  SetMediaPanelSize,
} from "../../bindings/option-tab/app.js";
import type { MediaProvider, MediaViewState } from "./types";

export const media = {
  state: (session: number) => GetMediaState(session) as Promise<MediaViewState | null>,
  action: (session: number, revision: number, kind: string, positionMS: number) =>
    PerformMediaAction(session, revision, kind, positionMS),
  pin: (session: number, revision: number) => PinMediaPanel(session, revision),
  close: (session: number, revision: number) => CloseMediaPanel(session, revision),
  size: (session: number, revision: number, width: number, height: number) =>
    SetMediaPanelSize(session, revision, width, height),
  importLyrics: (session: number, revision: number) => ImportMediaLyrics(session, revision),
  cancelImport: (session: number, revision: number) => CancelMediaLyricsImport(session, revision),
  reloadLyrics: (session: number, revision: number) => ReloadMediaLyrics(session, revision),
  removeLyrics: (session: number, revision: number) => RemoveMediaLyrics(session, revision),
  offset: (session: number, revision: number, offsetMS: number) =>
    SetMediaLyricsOffset(session, revision, offsetMS),
  connect: (provider: MediaProvider) =>
    ConnectMediaProvider(provider) as Promise<{ status: string; reason: string }>,
  permissions: () =>
    GetMediaPermissions() as Promise<Record<string, { status: string; reason: string }>>,
};
export function onMediaEvents(h: {
  update: (s: MediaViewState) => void;
  hide: (session: number, revision: number) => void;
  progress: (p: {
    session: number;
    revision: number;
    sequence: number;
    positionMS: number;
    activeCue: number;
  }) => void;
  permission?: (provider: MediaProvider, status: string, reason: string) => void;
}) {
  const off = [
    Events.On("media:update", (e) => h.update(e.data as MediaViewState)),
    Events.On("media:hide", (e) => {
      const d = e.data as { session: number; revision: number };
      h.hide(d.session, d.revision);
    }),
    Events.On("media:progress", (e) => h.progress(e.data as any)),
    Events.On("media:permission", (e) => {
      const d = e.data as { provider: MediaProvider; status: string; reason: string };
      h.permission?.(d.provider, d.status, d.reason);
    }),
  ];
  return () => off.forEach((f) => f());
}

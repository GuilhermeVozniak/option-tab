export interface LauncherBadgeEntry {
  itemID: string;
  state: "known" | "unsupported" | "unavailable";
  kind?: "absent" | "count" | "indicator";
  count?: number;
}

export interface LauncherBadgeState {
  epoch: number;
  displayUUID: string;
  session: number;
  presentationRevision: number;
  owner: number;
  sequence: number;
  visible: boolean;
  status: string;
  entries: LauncherBadgeEntry[];
}

export interface LauncherBadgeTransport {
  get(session: number): Promise<LauncherBadgeState>;
  subscribe(handler: (state: LauncherBadgeState) => void): () => void;
}

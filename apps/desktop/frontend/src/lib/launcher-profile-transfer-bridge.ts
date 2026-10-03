import type { JSONExportResult } from "./json-export-bridge";

export interface LauncherProfileImportReview {
  digest: string;
  revision: string;
  name: string;
  itemCount: number;
  widgetCount: number;
  notices: string[];
}

export interface LauncherProfileImportResult {
  profileID: string;
  settingsJSON: string;
}

export interface LauncherProfileTransferActions {
  exportProfile(profileID: string): Promise<JSONExportResult>;
  previewImport(document: string): Promise<LauncherProfileImportReview>;
  importProfile(
    document: string,
    digest: string,
    expectedRevision: string,
  ): Promise<LauncherProfileImportResult>;
}

export const launcherProfileTransfer: LauncherProfileTransferActions = {
  exportProfile: (profileID) => SaveLauncherProfileExport(profileID) as Promise<JSONExportResult>,
  previewImport: (document) => PreviewLauncherProfileImport(document),
  importProfile: (document, digest, expectedRevision) =>
    ImportLauncherProfile(document, digest, expectedRevision),
};

import {
  ImportLauncherProfile,
  PreviewLauncherProfileImport,
  SaveLauncherProfileExport,
} from "../../bindings/option-tab/app.js";

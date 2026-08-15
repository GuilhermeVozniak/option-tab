// Shared types and row shells for the preferences UI. Tabs receive a single
// TabContext with the settings, the translator, and the patch helpers, so
// each tab file stays focused on its own markup.
import type { Translate } from "../lib/i18n";
import type { Permissions, PermKey, Settings as SettingsModel } from "../lib/types";

// PROJECT_URL is the public home of this free clone (About-tab links).
export const PROJECT_URL = "https://github.com/GuilhermeVozniak/option-tab";

// PermissionsControl carries the live permission state plus the actions the
// Permissions section invokes. When omitted (e.g. running without Wails), the
// section is not rendered.
export interface PermissionsControl {
  state: Permissions;
  onRequest: (kind: PermKey) => void;
  onOpenSettings: (kind: PermKey) => void;
}

// AboutControl carries the live app version, link/update actions, and any
// newer release found by the background checker. When omitted (e.g. running
// without Wails), the About tab uses dev fallbacks.
export interface AboutControl {
  version: string;
  update?: { version: string; url: string };
  // progress is the self-installer's current stage while an update installs
  // (downloading/installing/restarting; "error" carries a failure message).
  progress?: { stage: string; message?: string };
  // checked is the outcome of the most recent update check, so the tab can
  // say "up to date" / "could not check" when no install banner applies.
  checked?: { latest?: string; available: boolean; error?: string };
  onOpenURL: (url: string) => void;
  onCheckUpdates: () => void;
  onInstallUpdate?: () => void;
}

// CrashControl surfaces a crash log from the previous run: report opens a
// prefilled GitHub issue, dismiss discards it. Omitted when there is nothing
// to report.
export interface CrashControl {
  summary: string;
  onReport: () => void;
  onDismiss: () => void;
}

// TabContext is everything a tab needs to read and edit settings.
export interface TabContext {
  settings: SettingsModel;
  t: Translate;
  onChange: (next: SettingsModel) => void;
  patch: (p: Partial<SettingsModel>) => void;
  patchAppearance: (p: Partial<SettingsModel["appearance"]>) => void;
  patchBehavior: (p: Partial<SettingsModel["behavior"]>) => void;
  patchFilters: (p: Partial<SettingsModel["filters"]>) => void;
  patchShortcut: (id: number, p: Partial<SettingsModel["shortcuts"][number]>) => void;
}

// Shared row shells: a control on the right, its (translated) text on the left.
export const ROW = "flex items-center justify-between gap-4 py-1 text-[13px]";
export const CHECK_LABEL = "flex cursor-pointer items-center gap-2 text-[13px]";
export const HINT = "m-0 text-xs leading-relaxed text-muted-foreground";
export const ACTIONS_ROW = "flex flex-wrap gap-2 pt-1";

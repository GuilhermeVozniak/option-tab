import { SaveSettingsExport } from "../../bindings/option-tab/app.js";
import type { Translate } from "./i18n";

export interface JSONExportResult {
  status: "saved" | "cancelled";
}

export const saveSettingsExport = () => SaveSettingsExport() as Promise<JSONExportResult>;

export function jsonExportError(cause: unknown, t: Translate): string {
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/destinationExists/.test(message))
    return t("That filename already exists. Choose a new name.");
  if (/unavailable/.test(message)) return t("Export is unavailable right now.");
  if (/busy/.test(message)) return t("Another export is already in progress.");
  return t("The file could not be exported.");
}

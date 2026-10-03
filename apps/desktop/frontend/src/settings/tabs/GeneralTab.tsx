import { type ReactNode, type RefObject, useEffect, useRef, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Switch } from "@/components/ui/switch";
import { LANGUAGES } from "../../lib/i18n";
import { type JSONExportResult, jsonExportError } from "../../lib/json-export-bridge";
import {
  type CrashPolicy,
  defaultSettings,
  type MenubarIconStyle,
  type UpdatePolicy,
} from "../../lib/types";
import { PermissionRow } from "../PermissionRow";
import {
  ACTIONS_ROW,
  CHECK_LABEL,
  type CrashControl,
  HINT,
  type PermissionsControl,
  ROW,
  type TabContext,
} from "../shared";

interface GeneralTabProps {
  ctx: TabContext;
  permissions?: PermissionsControl;
  crash?: CrashControl;
  /** Scroll target for update deep-links (menubar action, global banner). */
  updatesRef?: RefObject<HTMLDivElement | null>;
  /** Outcome of the last check ("up to date" / "could not check"). */
  updateCheckResult: ReactNode;
  checkUpdates: () => void;
  onImport?: (text: string) => Promise<void>;
  onExport?: () => Promise<JSONExportResult>;
}

export function GeneralTab({
  ctx,
  permissions,
  crash,
  updatesRef,
  updateCheckResult,
  checkUpdates,
  onImport,
  onExport,
}: GeneralTabProps) {
  const { settings, t, onChange, patchBehavior } = ctx;
  const [importError, setImportError] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  const [transferPending, setTransferPending] = useState<"export" | "import" | null>(null);
  const [exportError, setExportError] = useState("");
  const [exported, setExported] = useState(false);
  const [imported, setImported] = useState(false);
  const transferOwner = useRef(0);
  useEffect(
    () => () => {
      ++transferOwner.current;
    },
    [],
  );

  const exportSettings = async () => {
    const owner = ++transferOwner.current;
    setTransferPending("export");
    setExportError("");
    setExported(false);
    setImportError(null);
    setImported(false);
    try {
      if (!onExport) throw new Error("json export: unavailable");
      const result = await onExport();
      if (owner === transferOwner.current) setExported(result.status === "saved");
    } catch (cause) {
      if (owner === transferOwner.current) setExportError(jsonExportError(cause, t));
    } finally {
      if (owner === transferOwner.current) setTransferPending(null);
    }
  };
  const importFile = async (file: File | undefined) => {
    if (!file) return;
    const owner = ++transferOwner.current;
    setTransferPending("import");
    setImportError(null);
    setImported(false);
    setExportError("");
    setExported(false);
    try {
      if (!onImport) throw new Error("Import settings in the desktop app.");
      await onImport(await file.text());
      if (owner === transferOwner.current) setImported(true);
    } catch (error) {
      if (owner === transferOwner.current)
        setImportError(`Could not import settings: ${String(error)}`);
    } finally {
      if (owner === transferOwner.current) setTransferPending(null);
    }
  };

  return (
    <>
      {crash ? (
        <Alert appearance="unstyled" asChild>
          <div
            className="flex flex-wrap items-center gap-3 rounded-xl border border-red-400/35 bg-red-500/12 px-3.5 py-2.5 text-[13px] shadow-[inset_0_1px_0_rgba(255,255,255,0.1)] backdrop-blur-md"
            role="alert"
          >
            <span>{t("A crash from the previous session was detected.")}</span>
            <code className="max-w-full overflow-hidden text-ellipsis whitespace-nowrap text-xs text-red-200/85">
              {crash.summary}
            </code>
            <div className={ACTIONS_ROW}>
              <Button
                variant="destructive"
                size="sm"
                aria-label="Report crash"
                onClick={crash.onReport}
              >
                {t("Report crash…")}
              </Button>
              <Button
                variant="glass"
                size="sm"
                aria-label="Dismiss crash report"
                onClick={crash.onDismiss}
              >
                {t("Dismiss")}
              </Button>
            </div>
          </div>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("App behavior")}</CardTitle>
          <CardDescription>
            {t("Choose how Option Tab starts and appears in the menu bar.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Start at login")}</span>
            <Switch
              aria-label="Start at login"
              checked={settings.behavior.startAtLogin}
              onCheckedChange={(checked) => patchBehavior({ startAtLogin: checked })}
            />
          </Label>
          <fieldset className="ot-settings-menubar-options m-0 border-0 p-0">
            <legend className="mb-1 p-0 text-[13px] font-semibold">{t("Menubar icon")}</legend>
            <RadioGroup
              appearance="unstyled"
              className="contents"
              name="menubar-icon"
              aria-label={t("Menubar icon")}
              value={
                settings.behavior.showMenubarIcon ? settings.behavior.menubarIconStyle : "hidden"
              }
              onValueChange={(value) =>
                value === "hidden"
                  ? patchBehavior({ showMenubarIcon: false })
                  : patchBehavior({
                      showMenubarIcon: true,
                      menubarIconStyle: value as MenubarIconStyle,
                    })
              }
            >
              {(
                [
                  ["default", "⌥⇥ Default"],
                  ["outline", "⧉ Outline"],
                  ["dot", "● Dot"],
                ] as [MenubarIconStyle, string][]
              ).map(([value, label]) => (
                <Label appearance="unstyled" key={value} className={CHECK_LABEL}>
                  <RadioGroupItem value={value} aria-label={`Menubar icon ${value}`} />
                  {t(label)}
                </Label>
              ))}
              <Label appearance="unstyled" className={CHECK_LABEL}>
                <RadioGroupItem value="hidden" aria-label="Menubar icon hidden" />
                {t("Hidden")}
              </Label>
            </RadioGroup>
          </fieldset>

          <Label appearance="unstyled" className={ROW}>
            <span>{t("Language")}</span>
            <NativeSelect
              aria-label="Language"
              value={settings.behavior.language}
              onChange={(e) => patchBehavior({ language: e.target.value })}
            >
              {LANGUAGES.map((lang) => (
                <NativeSelectOption key={lang.value} value={lang.value}>
                  {lang.value === "" ? t("System default") : lang.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </Label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Window capture")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Capture windows in the background")}</span>
            <Switch
              aria-label="Capture windows in the background"
              checked={settings.behavior.captureInBackground}
              onCheckedChange={(checked) => patchBehavior({ captureInBackground: checked })}
            />
          </Label>
          <p className={HINT}>
            {t(
              "Keeps thumbnails fresh so the switcher opens with previews instantly. While enabled, macOS shows the screen-recording indicator.",
            )}
          </p>
        </CardContent>
      </Card>

      {permissions ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("Permissions")}</CardTitle>
            <CardDescription>
              {t("Option Tab needs these macOS permissions to work.")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <PermissionRow
              label="Accessibility"
              display={t("Accessibility")}
              hint={t(
                "Required for the global shortcut and window actions (focus, close, minimize).",
              )}
              state={permissions.state.accessibility}
              t={t}
              onRequest={() => permissions.onRequest("accessibility")}
              onOpenSettings={() => permissions.onOpenSettings("accessibility")}
            />
            <PermissionRow
              label="Screen Recording"
              display={t("Screen Recording")}
              hint={t(
                "Required for live window thumbnails; without it, app icons are shown instead.",
              )}
              state={permissions.state.screenRecording}
              t={t}
              onRequest={() => permissions.onRequest("screenRecording")}
              onOpenSettings={() => permissions.onOpenSettings("screenRecording")}
            />
          </CardContent>
        </Card>
      ) : null}

      <Card ref={updatesRef}>
        <CardHeader>
          <CardTitle>{t("Updates")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {updateCheckResult}
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="mb-1 p-0 text-[13px] font-semibold">{t("Updates policy")}</legend>
            <RadioGroup
              appearance="unstyled"
              className="contents"
              name="update-policy"
              aria-label={t("Updates policy")}
              value={settings.behavior.updatePolicy}
              onValueChange={(value) => patchBehavior({ updatePolicy: value as UpdatePolicy })}
            >
              {(
                [
                  ["check", "Check for updates periodically"],
                  ["auto", "Auto-install updates"],
                  ["off", "Don’t check for updates"],
                ] as [UpdatePolicy, string][]
              ).map(([value, label]) => (
                <Label appearance="unstyled" key={value} className={CHECK_LABEL}>
                  <RadioGroupItem value={value} aria-label={`Updates ${value}`} />
                  {t(label)}
                </Label>
              ))}
            </RadioGroup>
          </fieldset>
          <p className={HINT}>
            {t("Auto-install downloads, installs, and restarts the app when an update is found.")}
          </p>
          <div className={ACTIONS_ROW}>
            <Button aria-label="Check for updates now" onClick={checkUpdates}>
              {t("Check for updates now…")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Crash reports")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="mb-1 p-0 text-[13px] font-semibold">
              {t("Crash reports policy")}
            </legend>
            <RadioGroup
              appearance="unstyled"
              className="contents"
              name="crash-policy"
              aria-label={t("Crash reports policy")}
              value={settings.behavior.crashReports}
              onValueChange={(value) => patchBehavior({ crashReports: value as CrashPolicy })}
            >
              {(
                [
                  ["never", "Never send"],
                  ["ask", "Ask each time"],
                  ["always", "Always send"],
                ] as [CrashPolicy, string][]
              ).map(([value, label]) => (
                <Label appearance="unstyled" key={value} className={CHECK_LABEL}>
                  <RadioGroupItem value={value} aria-label={`Crash reports ${value}`} />
                  {t(label)}
                </Label>
              ))}
            </RadioGroup>
          </fieldset>
          <p className={HINT}>
            {t(
              "Crashes are captured locally; reporting opens a prefilled GitHub issue so you see exactly what is shared.",
            )}
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Backup and reset")}</CardTitle>
          <CardDescription>
            {t(
              "Export a backup or import an existing setup. Reset restores all feature settings to their defaults.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {importError ? (
            <Alert appearance="unstyled" asChild>
              <p role="alert" className="text-red-300">
                {importError}
              </p>
            </Alert>
          ) : null}
          {exported ? <p role="status">{t("Settings exported.")}</p> : null}
          {imported ? <p role="status">{t("Settings imported.")}</p> : null}
          {exportError ? (
            <Alert appearance="unstyled" asChild>
              <p role="alert">{exportError}</p>
            </Alert>
          ) : null}
          <div className={ACTIONS_ROW}>
            <Button
              aria-label="Export settings"
              disabled={transferPending !== null || !onExport}
              onClick={() => void exportSettings()}
            >
              {transferPending === "export" ? t("Exporting…") : t("Export…")}
            </Button>
            <Button
              aria-label="Import settings"
              disabled={transferPending !== null}
              onClick={() => fileInput.current?.click()}
            >
              {transferPending === "import" ? t("Importing…") : t("Import…")}
            </Button>
            <Button
              variant="destructive"
              aria-label="Reset to defaults"
              disabled={transferPending !== null}
              onClick={() => onChange(defaultSettings)}
            >
              {t("Reset to defaults")}
            </Button>
            <Input
              ref={fileInput}
              type="file"
              accept="application/json,.json"
              hidden
              onChange={(e) => {
                void importFile(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </div>
        </CardContent>
      </Card>
    </>
  );
}

import { type ReactNode, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Radio } from "@/components/ui/radio";
import { Select } from "@/components/ui/select";
import { LANGUAGES } from "../../lib/i18n";
import {
  type CrashPolicy,
  defaultSettings,
  type MenubarIconStyle,
  type Settings as SettingsModel,
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
  updateBanner: ReactNode;
  checkUpdates: () => void;
}

export function GeneralTab({
  ctx,
  permissions,
  crash,
  updateBanner,
  checkUpdates,
}: GeneralTabProps) {
  const { settings, t, onChange, patchBehavior } = ctx;
  const fileInput = useRef<HTMLInputElement>(null);

  const exportSettings = () => {
    try {
      const blob = new Blob([JSON.stringify(settings, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "option-tab-settings.json";
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // Download is unavailable (e.g. in tests); ignore.
    }
  };
  const importFile = (file: File | undefined) => {
    if (!file) return;
    file
      .text()
      .then((txt) => {
        try {
          onChange(JSON.parse(txt) as SettingsModel);
        } catch {
          // Ignore malformed files; the Go layer also validates on save.
        }
      })
      .catch(() => {});
  };

  return (
    <>
      {crash ? (
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
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("General")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <label className={ROW}>
            <span>{t("Start at login")}</span>
            <Checkbox
              aria-label="Start at login"
              checked={settings.behavior.startAtLogin}
              onChange={(e) => patchBehavior({ startAtLogin: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Capture windows in the background")}</span>
            <Checkbox
              aria-label="Capture windows in the background"
              checked={settings.behavior.captureInBackground}
              onChange={(e) => patchBehavior({ captureInBackground: e.target.checked })}
            />
          </label>
          <p className={HINT}>
            {t(
              "Keeps thumbnails fresh so the switcher opens with previews instantly. While enabled, macOS shows the screen-recording indicator.",
            )}
          </p>
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="mb-1 p-0 text-[13px] font-semibold">{t("Menubar icon")}</legend>
            {(
              [
                ["default", "⌥⇥ Default"],
                ["outline", "⧉ Outline"],
                ["dot", "● Dot"],
              ] as [MenubarIconStyle, string][]
            ).map(([value, label]) => (
              <label key={value} className={CHECK_LABEL}>
                <Radio
                  name="menubar-icon"
                  aria-label={`Menubar icon ${value}`}
                  checked={
                    settings.behavior.showMenubarIcon &&
                    settings.behavior.menubarIconStyle === value
                  }
                  onChange={() => patchBehavior({ showMenubarIcon: true, menubarIconStyle: value })}
                />
                {t(label)}
              </label>
            ))}
            <label className={CHECK_LABEL}>
              <Radio
                name="menubar-icon"
                aria-label="Menubar icon hidden"
                checked={!settings.behavior.showMenubarIcon}
                onChange={() => patchBehavior({ showMenubarIcon: false })}
              />
              {t("Hidden")}
            </label>
          </fieldset>

          <label className={ROW}>
            <span>{t("Language")}</span>
            <Select
              aria-label="Language"
              value={settings.behavior.language}
              onChange={(e) => patchBehavior({ language: e.target.value })}
            >
              {LANGUAGES.map((lang) => (
                <option key={lang.value} value={lang.value}>
                  {lang.value === "" ? t("System default") : lang.label}
                </option>
              ))}
            </Select>
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Updates")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {updateBanner}
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="mb-1 p-0 text-[13px] font-semibold">{t("Updates policy")}</legend>
            {(
              [
                ["check", "Check for updates periodically"],
                ["auto", "Auto-install updates"],
                ["off", "Don’t check for updates"],
              ] as [UpdatePolicy, string][]
            ).map(([value, label]) => (
              <label key={value} className={CHECK_LABEL}>
                <Radio
                  name="update-policy"
                  aria-label={`Updates ${value}`}
                  checked={settings.behavior.updatePolicy === value}
                  onChange={() => patchBehavior({ updatePolicy: value })}
                />
                {t(label)}
              </label>
            ))}
          </fieldset>
          <p className={HINT}>
            {t(
              "Auto-install downloads the new installer to your Downloads folder and opens it when an update is found.",
            )}
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
            {(
              [
                ["never", "Never send"],
                ["ask", "Ask each time"],
                ["always", "Always send"],
              ] as [CrashPolicy, string][]
            ).map(([value, label]) => (
              <label key={value} className={CHECK_LABEL}>
                <Radio
                  name="crash-policy"
                  aria-label={`Crash reports ${value}`}
                  checked={settings.behavior.crashReports === value}
                  onChange={() => patchBehavior({ crashReports: value })}
                />
                {t(label)}
              </label>
            ))}
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
          <CardTitle>{t("Settings file")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className={ACTIONS_ROW}>
            <Button aria-label="Export settings" onClick={exportSettings}>
              {t("Export…")}
            </Button>
            <Button aria-label="Import settings" onClick={() => fileInput.current?.click()}>
              {t("Import…")}
            </Button>
            <Button
              variant="destructive"
              aria-label="Reset to defaults"
              onClick={() => onChange(defaultSettings)}
            >
              {t("Reset to defaults")}
            </Button>
            <input
              ref={fileInput}
              type="file"
              accept="application/json,.json"
              hidden
              onChange={(e) => importFile(e.target.files?.[0])}
            />
          </div>
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
    </>
  );
}

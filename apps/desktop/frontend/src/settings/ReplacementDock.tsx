import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { Translate } from "../lib/i18n";
import type { LauncherStatus, ReplacementDockSettings } from "../lib/types";
import { HINT, ROW } from "./shared";

export function ReplacementDock({
  value,
  t,
  onChange,
  status,
  error,
  onUseNativeDock,
}: {
  value: ReplacementDockSettings;
  t: Translate;
  onChange: (value: ReplacementDockSettings) => void;
  status?: LauncherStatus;
  error?: string;
  onUseNativeDock?: () => void;
}) {
  const profile = value.profiles[0];
  const clock = profile?.widgets.find((widget) => widget.packageID === "org.optiontab.clock");
  const patchProfile = (partial: Partial<(typeof value.profiles)[number]>) =>
    onChange({
      ...value,
      profiles: value.profiles.map((p, index) => (index ? p : { ...p, ...partial })),
    });
  const patchClock = (enabled: boolean) =>
    onChange({
      ...value,
      profiles: value.profiles.map((p, index) =>
        index
          ? p
          : {
              ...p,
              widgets: p.widgets.map((w) =>
                w.id === clock?.id
                  ? {
                      ...w,
                      packageID: status?.clockPackageID ?? w.packageID,
                      digest: status?.clockDigest ?? w.digest,
                      enabled,
                      grants: enabled ? ["clock.read"] : [],
                    }
                  : w,
              ),
            },
      ),
    });
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("Replacement Dock")}</CardTitle>
        <CardDescription>
          {t("A floating launcher that coexists with the native Dock.")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <label className={ROW}>
          <span>{t("Enable replacement Dock")}</span>
          <Checkbox
            aria-label="Enable replacement Dock"
            checked={value.enabled}
            onChange={(event) => onChange({ ...value, enabled: event.target.checked })}
          />
        </label>
        <p className={HINT}>
          {t("The native Dock remains available. Use the menu command to return permanently.")}
        </p>
        <label className={ROW}>
          <span>{t("Show clock")}</span>
          <Checkbox
            aria-label="Show clock"
            checked={clock?.enabled ?? false}
            onChange={(event) => patchClock(event.target.checked)}
          />
        </label>
        <p className={HINT}>
          {t("The clock reads local time only while its explicit clock.read grant is enabled.")}
        </p>
        {profile ? (
          <div className="grid grid-cols-2 gap-2">
            <label>
              <span>{t("Icon size")}</span>
              <Input
                aria-label="Icon size"
                type="number"
                min={24}
                max={64}
                value={profile.iconPx}
                onChange={(event) => patchProfile({ iconPx: Number(event.target.value) })}
              />
            </label>
            <label>
              <span>{t("Dock thickness")}</span>
              <Input
                aria-label="Dock thickness"
                type="number"
                min={48}
                max={112}
                value={profile.thicknessPx}
                onChange={(event) => patchProfile({ thicknessPx: Number(event.target.value) })}
              />
            </label>
            <label>
              <span>{t("Screen inset")}</span>
              <Input
                aria-label="Screen inset"
                type="number"
                min={12}
                max={64}
                value={profile.insetPx}
                onChange={(event) => patchProfile({ insetPx: Number(event.target.value) })}
              />
            </label>
            <label className={ROW}>
              <span>{t("Auto-hide")}</span>
              <Checkbox
                aria-label="Auto-hide replacement Dock"
                checked={profile.autoHide}
                onChange={(event) => patchProfile({ autoHide: event.target.checked })}
              />
            </label>
          </div>
        ) : null}
        <div aria-label={t("Display bindings")}>
          {value.bindings.map((binding) => (
            <div className={ROW} key={binding.id}>
              <span>{binding.target === "main" ? t("Main display") : binding.displayUUID}</span>
              <Select
                aria-label={`${binding.id} profile`}
                value={binding.profileID}
                onChange={(event) =>
                  onChange({
                    ...value,
                    bindings: value.bindings.map((b) =>
                      b.id === binding.id ? { ...b, profileID: event.target.value } : b,
                    ),
                  })
                }
              >
                {value.profiles.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            </div>
          ))}
          {status?.displays
            .filter(
              (display) =>
                !value.bindings.some(
                  (binding) => binding.target === "display" && binding.displayUUID === display.uuid,
                ),
            )
            .map((display) => (
              <Button
                key={display.uuid}
                variant="outline"
                onClick={() =>
                  onChange({
                    ...value,
                    bindings: [
                      ...value.bindings,
                      {
                        id: `display-${value.bindings.length + 1}`,
                        target: "display",
                        displayUUID: display.uuid,
                        profileID: profile?.id ?? "default",
                      },
                    ],
                  })
                }
              >
                {t("Add {display}").replace("{display}", display.name)}
              </Button>
            ))}
        </div>
        {error || (status && status.status !== "ready" && status.status !== "disabled") ? (
          <p role="alert">
            {t("Replacement Dock unavailable")}: {error || status?.reason || status?.status}
          </p>
        ) : null}
        {onUseNativeDock ? (
          <Button variant="outline" onClick={onUseNativeDock}>
            {t("Use native Dock")}
          </Button>
        ) : null}
      </CardContent>
    </Card>
  );
}

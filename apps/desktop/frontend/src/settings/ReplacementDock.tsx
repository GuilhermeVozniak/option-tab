import { useEffect, useState } from "react";
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
  const [profileID, setProfileID] = useState(value.profiles[0]?.id ?? "");
  const [replacementID, setReplacementID] = useState("");
  useEffect(() => {
    if (!value.profiles.some((profile) => profile.id === profileID))
      setProfileID(value.profiles[0]?.id ?? "");
  }, [profileID, value.profiles]);
  useEffect(() => setReplacementID(""), [profileID]);
  const profile = value.profiles.find((profile) => profile.id === profileID) ?? value.profiles[0];
  const clock = profile?.widgets.find((widget) => widget.packageID === "org.optiontab.clock");
  const bindingLabel = (binding: ReplacementDockSettings["bindings"][number]) => {
    if (binding.target === "main") return t("Main display");
    const display = status?.displays.find((candidate) => candidate.uuid === binding.displayUUID);
    return display?.name || `${binding.displayUUID} (${t("Disconnected")})`;
  };
  const patchProfile = (partial: Partial<(typeof value.profiles)[number]>) =>
    onChange({
      ...value,
      profiles: value.profiles.map((p) => (p.id === profile?.id ? { ...p, ...partial } : p)),
    });
  const patchClock = (enabled: boolean) =>
    onChange({
      ...value,
      profiles: value.profiles.map((p) =>
        p.id === profile?.id
          ? {
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
            }
          : p,
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
        <div className="flex flex-wrap gap-2">
          <Select
            aria-label="Profile"
            value={profile?.id}
            onChange={(event) => setProfileID(event.target.value)}
          >
            {value.profiles.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
          <Button
            variant="outline"
            disabled={value.profiles.length >= 8}
            onClick={() => {
              let n = value.profiles.length + 1;
              while (value.profiles.some((p) => p.id === `profile-${n}`)) n++;
              const next = {
                ...(profile ?? value.profiles[0]),
                id: `profile-${n}`,
                name: t("New profile"),
                widgets: (profile?.widgets ?? []).map((w) => ({ ...w, grants: [...w.grants] })),
              };
              setProfileID(next.id);
              onChange({ ...value, profiles: [...value.profiles, next] });
            }}
          >
            {t("New profile")}
          </Button>
          <Button
            variant="outline"
            disabled={!profile || value.profiles.length >= 8}
            onClick={() => {
              if (!profile) return;
              let n = value.profiles.length + 1;
              while (value.profiles.some((p) => p.id === `profile-${n}`)) n++;
              const copyName = Array.from(`${profile.name} ${t("copy")}`)
                .slice(0, 80)
                .join("");
              const next = {
                ...profile,
                id: `profile-${n}`,
                name: copyName,
                widgets: profile.widgets.map((w) => ({ ...w, grants: [...w.grants] })),
              };
              setProfileID(next.id);
              onChange({ ...value, profiles: [...value.profiles, next] });
            }}
          >
            {t("Duplicate profile")}
          </Button>
          <Button
            variant="outline"
            disabled={
              value.profiles.length <= 1 ||
              !profile ||
              !value.profiles.some(
                (candidate) => candidate.id === replacementID && candidate.id !== profile.id,
              )
            }
            onClick={() => {
              if (!profile || value.profiles.length <= 1) return;
              const replacement = value.profiles.find(
                (p) => p.id === replacementID && p.id !== profile.id,
              );
              if (!replacement) return;
              setProfileID(replacement.id);
              onChange({
                ...value,
                profiles: value.profiles.filter((p) => p.id !== profile.id),
                bindings: value.bindings.map((b) =>
                  b.profileID === profile.id ? { ...b, profileID: replacement.id } : b,
                ),
              });
            }}
          >
            {t("Delete profile")}
          </Button>
          {value.profiles.length > 1 ? (
            <Select
              aria-label="Reassign deleted profile to"
              value={replacementID}
              onChange={(event) => setReplacementID(event.target.value)}
            >
              <option value="">{t("Choose replacement")}</option>
              {value.profiles
                .filter((p) => p.id !== profile?.id)
                .map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
            </Select>
          ) : null}
        </div>
        {profile ? (
          <label>
            <span>{t("Profile name")}</span>
            <Input
              aria-label="Profile name"
              maxLength={80}
              value={profile.name}
              onChange={(event) => patchProfile({ name: event.target.value })}
            />
          </label>
        ) : null}
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
        <p className={HINT}>{t("Reads local time while visible.")}</p>
        {profile ? (
          <div className="grid grid-cols-2 gap-2">
            <label>
              <span>{t("Edge")}</span>
              <Select
                aria-label="Edge"
                value={profile.edge}
                onChange={(event) =>
                  patchProfile({ edge: event.target.value as typeof profile.edge })
                }
              >
                {["bottom", "left", "right", "top"].map((v) => (
                  <option key={v} value={v}>
                    {t(v)}
                  </option>
                ))}
              </Select>
            </label>
            <label>
              <span>{t("Layout")}</span>
              <Select
                aria-label="Layout"
                value={profile.layout}
                onChange={(event) =>
                  patchProfile({ layout: event.target.value as typeof profile.layout })
                }
              >
                <option value="floating">{t("Floating")}</option>
                <option value="fullWidth">{t("Full width")}</option>
              </Select>
            </label>
            <label>
              <span>{t("Alignment")}</span>
              <Select
                aria-label="Alignment"
                value={profile.alignment}
                onChange={(event) =>
                  patchProfile({ alignment: event.target.value as typeof profile.alignment })
                }
              >
                {["start", "center", "end"].map((v) => (
                  <option key={v} value={v}>
                    {t(v)}
                  </option>
                ))}
              </Select>
            </label>
            <label>
              <span>{t("Theme")}</span>
              <Select
                aria-label="Launcher theme"
                value={profile.appearance.theme}
                onChange={(event) =>
                  patchProfile({
                    appearance: {
                      ...profile.appearance,
                      theme: event.target.value as typeof profile.appearance.theme,
                    },
                  })
                }
              >
                {["system", "light", "dark"].map((v) => (
                  <option key={v} value={v}>
                    {t(v)}
                  </option>
                ))}
              </Select>
            </label>
            <label>
              <span>{t("Material")}</span>
              <Select
                aria-label="Material"
                value={profile.appearance.material}
                onChange={(event) =>
                  patchProfile({
                    appearance: {
                      ...profile.appearance,
                      material: event.target.value as typeof profile.appearance.material,
                    },
                  })
                }
              >
                <option value="solid">{t("Solid")}</option>
                <option value="system">{t("System material")}</option>
              </Select>
            </label>
            <label>
              <span>{t("Tint")}</span>
              <Input
                aria-label="Tint"
                type="color"
                value={profile.appearance.tint}
                onChange={(event) =>
                  patchProfile({ appearance: { ...profile.appearance, tint: event.target.value } })
                }
              />
            </label>
            <label>
              <span>{t("Opacity")}</span>
              <Input
                aria-label="Launcher opacity"
                type="number"
                min={0.35}
                max={1}
                step={0.05}
                value={profile.appearance.opacity}
                onChange={(event) =>
                  patchProfile({
                    appearance: { ...profile.appearance, opacity: Number(event.target.value) },
                  })
                }
              />
            </label>
            <label>
              <span>{t("Border opacity")}</span>
              <Input
                aria-label="Border opacity"
                type="number"
                min={0}
                max={0.5}
                step={0.05}
                value={profile.appearance.borderOpacity}
                onChange={(event) =>
                  patchProfile({
                    appearance: {
                      ...profile.appearance,
                      borderOpacity: Number(event.target.value),
                    },
                  })
                }
              />
            </label>
            <label>
              <span>{t("Corner radius")}</span>
              <Input
                aria-label="Corner radius"
                type="number"
                min={0}
                max={28}
                value={profile.appearance.cornerRadiusPx}
                onChange={(event) =>
                  patchProfile({
                    appearance: {
                      ...profile.appearance,
                      cornerRadiusPx: Number(event.target.value),
                    },
                  })
                }
              />
            </label>
            <label>
              <span>{t("Item spacing")}</span>
              <Input
                aria-label="Item spacing"
                type="number"
                min={2}
                max={20}
                value={profile.appearance.itemSpacingPx}
                onChange={(event) =>
                  patchProfile({
                    appearance: {
                      ...profile.appearance,
                      itemSpacingPx: Number(event.target.value),
                    },
                  })
                }
              />
            </label>
            <label className={ROW}>
              <span>{t("Show labels")}</span>
              <Checkbox
                aria-label="Show launcher labels"
                checked={profile.appearance.showLabels}
                onChange={(event) =>
                  patchProfile({
                    appearance: { ...profile.appearance, showLabels: event.target.checked },
                  })
                }
              />
            </label>
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
              <span>{t("Maximum length")}</span>
              <Input
                aria-label="Maximum length"
                type="number"
                min={0.25}
                max={0.9}
                step={0.05}
                value={profile.maxLengthFraction}
                onChange={(event) =>
                  patchProfile({ maxLengthFraction: Number(event.target.value) })
                }
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
              <span>{bindingLabel(binding)}</span>
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
              <Button
                aria-label={`${t("Remove assignment")} ${binding.id}`}
                variant="outline"
                onClick={() =>
                  onChange({
                    ...value,
                    bindings: value.bindings.filter((b) => b.id !== binding.id),
                  })
                }
              >
                {t("Remove")}
              </Button>
            </div>
          ))}
          {status?.displays
            .filter(
              (display) =>
                !value.bindings.some((binding) =>
                  display.main
                    ? binding.target === "main" ||
                      (binding.target === "display" && binding.displayUUID === display.uuid)
                    : binding.target === "display" && binding.displayUUID === display.uuid,
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
                        target: display.main ? "main" : "display",
                        displayUUID: display.main ? "" : display.uuid,
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

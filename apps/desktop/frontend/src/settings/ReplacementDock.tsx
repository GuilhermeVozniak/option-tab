import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { resolveLang, type Translate } from "../lib/i18n";
import type { LauncherProfileTransferActions } from "../lib/launcher-profile-transfer-bridge";
import type {
  LauncherAppChoice,
  LauncherProfileRule,
  LauncherStatus,
  ReplacementDockSettings,
} from "../lib/types";
import type { WidgetCatalogDescriptor, WidgetPackageStatus } from "../lib/widget-types";
import { type WidgetPackageActions, WidgetPackages } from "../widgets/WidgetPackages";
import { WidgetSettings } from "../widgets/WidgetSettings";
import { type LauncherItemSettingsActions, LauncherItems } from "./LauncherItems";
import { LauncherProfileTransfer } from "./LauncherProfileTransfer";
import { HINT, ROW } from "./shared";

export function ReplacementDock({
  value,
  t,
  onChange,
  status,
  error,
  onUseNativeDock,
  appChoices = [],
  widgetCatalog = [],
  itemActions,
  language = "",
  widgetPackages,
  profileTransfer,
}: {
  value: ReplacementDockSettings;
  t: Translate;
  onChange: (value: ReplacementDockSettings) => void;
  status?: LauncherStatus;
  error?: string;
  onUseNativeDock?: () => void;
  appChoices?: LauncherAppChoice[];
  widgetCatalog?: WidgetCatalogDescriptor[];
  itemActions?: LauncherItemSettingsActions;
  language?: string;
  widgetPackages?: {
    status: WidgetPackageStatus;
    actions: WidgetPackageActions;
    onRefresh: () => void;
  };
  profileTransfer?: LauncherProfileTransferActions;
}) {
  const [profileID, setProfileID] = useState(value.profiles[0]?.id ?? "");
  const [replacementID, setReplacementID] = useState("");
  const pendingImportedProfile = useRef("");
  useEffect(() => {
    if (pendingImportedProfile.current) {
      if (value.profiles.some((profile) => profile.id === pendingImportedProfile.current)) {
        setProfileID(pendingImportedProfile.current);
        pendingImportedProfile.current = "";
      }
      return;
    }
    if (!value.profiles.some((profile) => profile.id === profileID))
      setProfileID(value.profiles[0]?.id ?? "");
  }, [profileID, value.profiles]);
  useEffect(() => setReplacementID(""), [profileID]);
  const profile = value.profiles.find((profile) => profile.id === profileID) ?? value.profiles[0];
  const rules = value.rules ?? [];
  const patchRules = (next: LauncherProfileRule[]) => onChange({ ...value, rules: next });
  const patchRule = (id: string, partial: Partial<LauncherProfileRule>) =>
    patchRules(rules.map((rule) => (rule.id === id ? { ...rule, ...partial } : rule)));
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
                rules: rules.map((rule) =>
                  rule.profileID === profile.id ? { ...rule, profileID: replacement.id } : rule,
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
        {profile && profileTransfer ? (
          <LauncherProfileTransfer
            key={`transfer-${profile.id}`}
            profileID={profile.id}
            t={t}
            actions={profileTransfer}
            onImported={(importedProfileID) => {
              pendingImportedProfile.current = importedProfileID;
              if (value.profiles.some((candidate) => candidate.id === importedProfileID)) {
                setProfileID(importedProfileID);
                pendingImportedProfile.current = "";
              }
            }}
          />
        ) : null}
        {widgetPackages ? (
          <WidgetPackages
            catalog={widgetCatalog}
            status={widgetPackages.status}
            actions={widgetPackages.actions}
            onRefresh={widgetPackages.onRefresh}
            t={t}
            language={resolveLang(language)}
          />
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
        {profile && itemActions ? (
          <LauncherItems
            key={`items-${profile.id}`}
            profileID={profile.id}
            t={t}
            actions={itemActions}
          />
        ) : null}
        {profile ? (
          <WidgetSettings
            key={profile.id}
            instances={profile.widgets}
            stacks={profile.stacks ?? []}
            catalog={widgetCatalog}
            language={resolveLang(language)}
            t={t}
            onChange={({ widgets, stacks }) => patchProfile({ widgets, stacks })}
          />
        ) : null}
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
                    rules: rules.filter((rule) => rule.bindingID !== binding.id),
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
          <p className={HINT}>
            {t(
              "Removing a display assignment also removes focus rules scoped only to that display.",
            )}
          </p>
        </div>
        <section aria-labelledby="focus-rules-heading" className="space-y-2 pt-2">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h3 id="focus-rules-heading" className="m-0 text-sm font-semibold">
                {t("Focus rules")}
              </h3>
              <p className={HINT}>
                {t(
                  "The first matching app rule chooses a profile. Display assignments remain the base profile.",
                )}
              </p>
            </div>
            <Button
              variant="outline"
              disabled={rules.length >= 8 || value.profiles.length === 0}
              onClick={() => {
                let n = rules.length + 1;
                while (rules.some((rule) => rule.id === `focus-${n}`)) n++;
                patchRules([
                  ...rules,
                  {
                    id: `focus-${n}`,
                    enabled: true,
                    bundleID: "",
                    profileID: profile?.id ?? value.profiles[0]?.id ?? "",
                    bindingID: "",
                  },
                ]);
              }}
            >
              {t("Add focus rule")}
            </Button>
          </div>
          {rules.length === 0 ? <p className={HINT}>{t("No focus rules")}</p> : null}
          {rules.map((rule, index) => {
            const invalidBundle = !/^[A-Za-z0-9._-]{1,255}$/.test(rule.bundleID);
            const label = rule.bundleID || t("New rule");
            return (
              <article key={rule.id} className="rounded-xl border border-white/10 bg-black/10 p-3">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <span className="text-xs text-muted-foreground">
                    {t("Priority {number}").replace("{number}", String(index + 1))}
                  </span>
                  <div className="flex gap-1">
                    <Button
                      variant="outline"
                      aria-label={`${t("Move {app} up").replace("{app}", label)}`}
                      disabled={index === 0}
                      onClick={() => {
                        const next = [...rules];
                        [next[index - 1], next[index]] = [next[index], next[index - 1]];
                        patchRules(next);
                      }}
                    >
                      ↑
                    </Button>
                    <Button
                      variant="outline"
                      aria-label={`${t("Move {app} down").replace("{app}", label)}`}
                      disabled={index === rules.length - 1}
                      onClick={() => {
                        const next = [...rules];
                        [next[index], next[index + 1]] = [next[index + 1], next[index]];
                        patchRules(next);
                      }}
                    >
                      ↓
                    </Button>
                    <Button
                      variant="outline"
                      aria-label={t("Remove {app}").replace("{app}", label)}
                      onClick={() =>
                        patchRules(rules.filter((candidate) => candidate.id !== rule.id))
                      }
                    >
                      {t("Remove")}
                    </Button>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <label>
                    <span>{t("Running app")}</span>
                    <Select
                      aria-label="Running app"
                      value={
                        appChoices.some((choice) => choice.bundleID === rule.bundleID)
                          ? rule.bundleID
                          : ""
                      }
                      onChange={(event) => patchRule(rule.id, { bundleID: event.target.value })}
                    >
                      <option value="">{t("Enter bundle identifier")}</option>
                      {appChoices.map((choice) => (
                        <option key={choice.bundleID} value={choice.bundleID}>
                          {choice.name} — {choice.bundleID}
                        </option>
                      ))}
                    </Select>
                  </label>
                  <label>
                    <span>{t("Exact bundle identifier")}</span>
                    <Input
                      aria-label={`${t("Exact bundle identifier")} ${label}`}
                      value={rule.bundleID}
                      maxLength={255}
                      spellCheck={false}
                      onChange={(event) => patchRule(rule.id, { bundleID: event.target.value })}
                    />
                  </label>
                  <label>
                    <span>{t("Destination profile")}</span>
                    <Select
                      aria-label={`${t("Destination profile")} ${label}`}
                      value={rule.profileID}
                      onChange={(event) => patchRule(rule.id, { profileID: event.target.value })}
                    >
                      {value.profiles.map((candidate) => (
                        <option key={candidate.id} value={candidate.id}>
                          {candidate.name}
                        </option>
                      ))}
                    </Select>
                  </label>
                  <label>
                    <span>{t("Display scope")}</span>
                    <Select
                      aria-label={`${t("Display scope")} ${label}`}
                      value={rule.bindingID}
                      onChange={(event) => patchRule(rule.id, { bindingID: event.target.value })}
                    >
                      <option value="">{t("Every assigned display")}</option>
                      {value.bindings.map((binding) => (
                        <option key={binding.id} value={binding.id}>
                          {bindingLabel(binding)}
                        </option>
                      ))}
                    </Select>
                  </label>
                </div>
                <label className={ROW}>
                  <span>{t("Enabled")}</span>
                  <Checkbox
                    aria-label={t("Enable rule {app}").replace("{app}", label)}
                    checked={rule.enabled}
                    onChange={(event) => patchRule(rule.id, { enabled: event.target.checked })}
                  />
                </label>
                {invalidBundle ? (
                  <p role="alert" className={HINT}>
                    {t(
                      "Enter an exact bundle identifier using letters, numbers, dots, hyphens, or underscores.",
                    )}
                  </p>
                ) : null}
              </article>
            );
          })}
        </section>
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

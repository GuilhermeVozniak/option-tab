import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { AppScopeMode } from "../../lib/types";
import { HINT, type PermissionsControl, ROW, type TabContext } from "../shared";
import { AppearanceTab } from "./AppearanceTab";

export function DockTab({
  ctx,
  permissions,
}: {
  ctx: TabContext;
  permissions?: PermissionsControl;
}) {
  const { settings, t, patch } = ctx;
  const d = settings.dock;
  const patchDock = (p: Partial<typeof d>) => patch({ dock: { ...d, ...p } });
  const appearanceContext: TabContext = {
    ...ctx,
    modeAppearance: d.appearance,
    patchModeAppearance: (appearance) =>
      patchDock({ appearance: { ...d.appearance, ...appearance } }),
    patchModePreferences: ({ appearance }) => {
      if (appearance) patchDock({ appearance: { ...d.appearance, ...appearance } });
    },
  };
  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("Dock previews")}</CardTitle>
          <CardDescription>
            {t("Show an app’s windows when the pointer rests on its Dock icon.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Enable Dock previews")}</span>
            <Checkbox
              aria-label="Enable Dock previews"
              checked={d.enabled}
              onChange={(e) => {
                patchDock({ enabled: e.target.checked });
                if (e.target.checked && permissions) {
                  if (permissions.state.accessibility !== "granted")
                    permissions.onRequest("accessibility");
                  if (permissions.state.screenRecording !== "granted")
                    permissions.onRequest("screenRecording");
                }
              }}
            />
          </label>
          <p className={HINT}>
            {t(
              "Accessibility identifies Dock icons and window controls. Screen Recording provides thumbnails.",
            )}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Timing and movement")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          {(
            [
              ["Dock hover delay", "hoverDelayMs", 0, 2000],
              ["Dock dismiss delay", "dismissDelayMs", 0, 2000],
              ["Dock movement tolerance", "hoverSlopPx", 0, 32],
              ["Dock corridor padding", "bridgePaddingPx", 0, 48],
            ] as const
          ).map(([label, field, min, max]) => (
            <label className={ROW} key={field}>
              <span>{t(label)}</span>
              <Input
                className="w-24"
                type="number"
                aria-label={label}
                min={min}
                max={max}
                value={d[field]}
                onChange={(e) => patchDock({ [field]: Number(e.target.value) })}
              />
            </label>
          ))}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Dock window list")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Applications")}</span>
            <Select
              aria-label="Dock app scope"
              value={d.scope.appScope}
              onChange={(e) =>
                patchDock({ scope: { ...d.scope, appScope: e.target.value as AppScopeMode } })
              }
            >
              <option value="all">{t("All apps")}</option>
              <option value="activeApp">{t("Active app only")}</option>
            </Select>
          </label>
        </CardContent>
      </Card>
      <AppearanceTab ctx={appearanceContext} variant="dock" />
    </>
  );
}

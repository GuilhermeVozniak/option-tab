import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { LauncherProfileTransferActions } from "../../lib/launcher-profile-transfer-bridge";
import type {
  AppScopeMode,
  DockInputSettings,
  DockLockDisplay,
  DockMonitorLockState,
  LauncherAppChoice,
  LauncherStatus,
  PointerAction,
} from "../../lib/types";
import type { WidgetCatalogDescriptor, WidgetPackageStatus } from "../../lib/widget-types";
import type { WidgetPackageActions } from "../../widgets/WidgetPackages";
import { DockMonitorLock } from "../DockMonitorLock";
import type { LauncherItemSettingsActions } from "../LauncherItems";
import { ReplacementDock } from "../ReplacementDock";
import { HINT, type PermissionsControl, ROW, type TabContext } from "../shared";
import { AppearanceTab } from "./AppearanceTab";

export function DockTab({
  ctx,
  permissions,
  inputError,
  monitorLock,
  media,
  launcher,
}: {
  ctx: TabContext;
  permissions?: PermissionsControl;
  inputError?: string;
  monitorLock?: {
    state?: DockMonitorLockState;
    displays: DockLockDisplay[];
    error?: string;
    pending?: boolean;
    onEnable: () => void;
    onPlace: (session: number, revision: number, generation: number) => void;
    onCancel: () => void;
  };
  media?: {
    permissions: Record<string, { status: string; reason: string }>;
    onConnect: (provider: "music" | "spotify") => void;
  };
  launcher?: {
    status?: LauncherStatus;
    error?: string;
    appChoices?: LauncherAppChoice[];
    widgetCatalog?: WidgetCatalogDescriptor[];
    itemActions?: LauncherItemSettingsActions;
    profileTransfer?: LauncherProfileTransferActions;
    widgetPackages?: {
      status: WidgetPackageStatus;
      actions: WidgetPackageActions;
      onRefresh: () => void;
    };
    onUseNativeDock: () => void;
  };
}) {
  const { settings, t, patch } = ctx;
  const d = settings.dock;
  const patchDock = (p: Partial<typeof d>) => patch({ dock: { ...d, ...p } });
  const input: DockInputSettings = d.input ?? {
    clickToHide: false,
    scrollShowHide: false,
    modifiedRightClick: false,
    swipeTowardDock: "none",
    swipeAwayFromDock: "none",
    swipePrevious: "none",
    swipeNext: "none",
    previewDrag: false,
    aeroShakeAction: "none",
  };
  const patchInput = (value: Partial<DockInputSettings>) =>
    patchDock({ input: { ...input, ...value } });
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
      <ReplacementDock
        value={settings.replacementDock}
        t={t}
        onChange={(replacementDock) => patch({ replacementDock })}
        status={launcher?.status}
        error={launcher?.error}
        onUseNativeDock={launcher?.onUseNativeDock}
        appChoices={launcher?.appChoices}
        widgetCatalog={launcher?.widgetCatalog}
        itemActions={launcher?.itemActions}
        profileTransfer={launcher?.profileTransfer}
        language={settings.behavior.language}
        widgetPackages={launcher?.widgetPackages}
      />
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
          <label className={ROW}>
            <span>{t("Enable Folder Pop")}</span>
            <Checkbox
              aria-label="Enable Folder Pop"
              checked={d.folderPop?.enabled ?? false}
              onChange={(event) => {
                patchDock({ folderPop: { enabled: event.target.checked } });
                if (event.target.checked && permissions?.state.accessibility !== "granted")
                  permissions?.onRequest("accessibility");
              }}
            />
          </label>
          <p className={HINT}>
            {t("Show a folder’s contents when the pointer rests on its Dock icon.")}
          </p>
          {inputError ? (
            <p role="alert" className="text-sm text-red-600 dark:text-red-400">
              {t("Dock input unavailable")}: {inputError}
            </p>
          ) : null}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Media controls")}</CardTitle>
          <CardDescription>
            {t(
              "Show playback details and controls for enabled players. Connecting is always explicit.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Enable media controls")}</span>
            <Checkbox
              aria-label="Enable media controls"
              checked={d.media?.enabled ?? false}
              onChange={(e) =>
                patchDock({
                  media: {
                    ...(d.media ?? {
                      enabled: false,
                      musicEnabled: false,
                      spotifyEnabled: false,
                      remoteArtwork: false,
                    }),
                    enabled: e.target.checked,
                  },
                })
              }
            />
          </label>
          {(["music", "spotify"] as const).map((provider) => {
            const field = provider === "music" ? "musicEnabled" : "spotifyEnabled";
            const label = provider === "music" ? "Apple Music" : "Spotify";
            return (
              <div key={provider}>
                <label className={ROW}>
                  <span>{t(`Enable ${label}`)}</span>
                  <Checkbox
                    aria-label={`Enable ${label}`}
                    checked={d.media?.[field] ?? false}
                    onChange={(e) =>
                      patchDock({ media: { ...d.media, [field]: e.target.checked } })
                    }
                  />
                </label>
                {d.media?.[field] && media ? (
                  <div className="flex items-center justify-between gap-3">
                    <small>{media.permissions[provider]?.reason || t("Not connected")}</small>
                    <button type="button" onClick={() => media.onConnect(provider)}>
                      {t("Connect")}
                    </button>
                  </div>
                ) : null}
              </div>
            );
          })}
          <label className={ROW}>
            <span>{t("Allow remote artwork")}</span>
            <Checkbox
              aria-label="Allow remote artwork"
              checked={d.media?.remoteArtwork ?? false}
              onChange={(e) =>
                patchDock({ media: { ...d.media, remoteArtwork: e.target.checked } })
              }
            />
          </label>
          <p className={HINT}>
            {t(
              "Remote artwork contacts the image host. Metadata and controls still work when this is off.",
            )}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Dock monitor lock")}</CardTitle>
          <CardDescription>
            {t(
              "Keep the system Dock on the selected monitor without changing persistent Dock preferences.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {monitorLock ? (
            <DockMonitorLock
              value={d.monitorLock}
              state={monitorLock.state}
              displays={monitorLock.displays}
              error={monitorLock.error}
              pending={monitorLock.pending}
              t={t}
              onChange={(value) => patchDock({ monitorLock: value })}
              onEnable={monitorLock.onEnable}
              onPlace={monitorLock.onPlace}
              onCancel={monitorLock.onCancel}
            />
          ) : null}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Input and gestures")}</CardTitle>
          <CardDescription>
            {t(
              "Precise trackpad scrolling powers preview swipes. macOS does not reliably expose the number of fingers.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          {(
            [
              ["Click Dock icon to hide app", "clickToHide"],
              ["Scroll Dock icon to show or hide app", "scrollShowHide"],
              ["Command-right-click to quit app", "modifiedRightClick"],
              ["Drag previews to move windows", "previewDrag"],
            ] as const
          ).map(([label, field]) => (
            <label className={ROW} key={field}>
              <span>{t(label)}</span>
              <Checkbox
                aria-label={label}
                checked={input[field]}
                onChange={(event) => patchInput({ [field]: event.target.checked })}
              />
            </label>
          ))}
          {(
            [
              ["Swipe toward Dock", "swipeTowardDock"],
              ["Swipe away from Dock", "swipeAwayFromDock"],
              ["Swipe to previous preview", "swipePrevious"],
              ["Swipe to next preview", "swipeNext"],
            ] as const
          ).map(([label, field]) => (
            <label className={ROW} key={field}>
              <span>{t(label)}</span>
              <Select
                aria-label={label}
                value={input[field]}
                onChange={(event) => patchInput({ [field]: event.target.value as PointerAction })}
              >
                {(
                  [
                    ["none", "None"],
                    ["close", "Close"],
                    ["minimize", "Minimize"],
                    ["fullscreen", "Fullscreen"],
                    ["hide", "Hide app"],
                    ["quit", "Quit app"],
                  ] as const
                ).map(([value, text]) => (
                  <option value={value} key={value}>
                    {t(text)}
                  </option>
                ))}
              </Select>
            </label>
          ))}
          <label className={ROW}>
            <span>{t("Aero Shake action")}</span>
            <Select
              aria-label="Aero Shake action"
              value={input.aeroShakeAction}
              onChange={(event) =>
                patchInput({
                  aeroShakeAction: event.target.value as DockInputSettings["aeroShakeAction"],
                })
              }
            >
              <option value="none">{t("None")}</option>
              <option value="minimizeOthers">{t("Minimize other windows")}</option>
              <option value="closeOthers">{t("Close other windows")}</option>
            </Select>
          </label>
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
              ["Dock card spacing", "cardSpacingPx", 0, 24],
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

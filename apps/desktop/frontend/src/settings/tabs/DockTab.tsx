import { useId, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Switch } from "@/components/ui/switch";
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
import { DockSettingsNavigation } from "../DockSettingsNavigation";
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
    pending?: Readonly<Record<string, boolean>>;
    onConnect: (provider: "music" | "spotify") => void;
  };
  launcher?: {
    status?: LauncherStatus;
    error?: string;
    appChoices?: LauncherAppChoice[];
    interactionCapabilities?: Partial<
      Record<"preciseScroll" | "pinch" | "swipe" | "letterNavigation" | "haptics", boolean>
    >;
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
  const sectionPrefix = useId();
  const [section, setSection] = useState("launcher");
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
    <div className="ot-dock-settings">
      <DockSettingsNavigation
        label={t("Dock sections")}
        prefix={sectionPrefix}
        value={section}
        onChange={setSection}
        sections={[
          ["launcher", t("Launcher")],
          ["previews", t("Window previews")],
          ["media", t("Media")],
          ["monitor", t("Monitor")],
        ]}
      />
      <section
        id={`${sectionPrefix}-launcher`}
        aria-labelledby={`${sectionPrefix}-launcher-nav`}
        hidden={section !== "launcher"}
        className="ot-dock-settings-panel"
      >
        <ReplacementDock
          value={settings.replacementDock}
          t={t}
          onChange={(replacementDock) => patch({ replacementDock })}
          status={launcher?.status}
          error={launcher?.error}
          onUseNativeDock={launcher?.onUseNativeDock}
          appChoices={launcher?.appChoices}
          interactionCapabilities={launcher?.interactionCapabilities}
          widgetCatalog={launcher?.widgetCatalog}
          itemActions={launcher?.itemActions}
          profileTransfer={launcher?.profileTransfer}
          language={settings.behavior.language}
          widgetPackages={launcher?.widgetPackages}
        />
      </section>
      <section
        id={`${sectionPrefix}-previews`}
        aria-labelledby={`${sectionPrefix}-previews-nav`}
        hidden={section !== "previews"}
        className="ot-dock-settings-panel"
      >
        <Card>
          <CardHeader>
            <CardTitle>{t("Dock previews")}</CardTitle>
            <CardDescription>
              {t("Show an app’s windows when the pointer rests on its Dock icon.")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-1">
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Enable Dock previews")}</span>
              <Switch
                aria-label={t("Enable Dock previews")}
                checked={d.enabled}
                onCheckedChange={(checked) => {
                  patchDock({ enabled: checked });
                  if (checked && permissions) {
                    if (permissions.state.accessibility !== "granted")
                      permissions.onRequest("accessibility");
                    if (permissions.state.screenRecording !== "granted")
                      permissions.onRequest("screenRecording");
                  }
                }}
              />
            </Label>
            <p className={HINT}>
              {t(
                "Accessibility identifies Dock icons and window controls. Screen Recording provides thumbnails.",
              )}
            </p>
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Enable Folder Pop")}</span>
              <Switch
                aria-label={t("Enable Folder Pop")}
                checked={d.folderPop?.enabled ?? false}
                onCheckedChange={(checked) => {
                  patchDock({ folderPop: { enabled: checked } });
                  if (checked && permissions?.state.accessibility !== "granted")
                    permissions?.onRequest("accessibility");
                }}
              />
            </Label>
            <p className={HINT}>
              {t("Show a folder’s contents when the pointer rests on its Dock icon.")}
            </p>
            {inputError ? (
              <Alert appearance="unstyled" asChild>
                <p role="alert" className="text-sm text-red-600 dark:text-red-400">
                  {t("Dock input unavailable")}: {inputError}
                </p>
              </Alert>
            ) : null}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("Dock window list")}</CardTitle>
            <CardDescription>
              {t("Choose which applications appear in window previews.")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-1">
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Applications")}</span>
              <NativeSelect
                aria-label={t("Dock app scope")}
                value={d.scope.appScope}
                onChange={(e) =>
                  patchDock({ scope: { ...d.scope, appScope: e.target.value as AppScopeMode } })
                }
              >
                <NativeSelectOption value="all">{t("All apps")}</NativeSelectOption>
                <NativeSelectOption value="activeApp">{t("Active app only")}</NativeSelectOption>
              </NativeSelect>
            </Label>
          </CardContent>
        </Card>
        <AppearanceTab ctx={appearanceContext} variant="dock" />
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
              <Label appearance="unstyled" className={ROW} key={field}>
                <span>{t(label)}</span>
                <Switch
                  aria-label={t(label)}
                  checked={input[field]}
                  onCheckedChange={(checked) => patchInput({ [field]: checked })}
                />
              </Label>
            ))}
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Dock middle-click action")}</span>
              <NativeSelect
                aria-label={t("Dock middle-click action")}
                value={input.middleClickAction}
                onChange={(event) =>
                  patchInput({ middleClickAction: event.target.value as PointerAction })
                }
              >
                <NativeSelectOption value="none">{t("None")}</NativeSelectOption>
                <NativeSelectOption value="close">{t("Close")}</NativeSelectOption>
                <NativeSelectOption value="minimize">{t("Minimize")}</NativeSelectOption>
              </NativeSelect>
            </Label>
            {(
              [
                ["Swipe toward Dock", "swipeTowardDock"],
                ["Swipe away from Dock", "swipeAwayFromDock"],
                ["Swipe to previous preview", "swipePrevious"],
                ["Swipe to next preview", "swipeNext"],
              ] as const
            ).map(([label, field]) => (
              <Label appearance="unstyled" className={ROW} key={field}>
                <span>{t(label)}</span>
                <NativeSelect
                  aria-label={t(label)}
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
                    <NativeSelectOption value={value} key={value}>
                      {t(text)}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </Label>
            ))}
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Aero Shake action")}</span>
              <NativeSelect
                aria-label={t("Aero Shake action")}
                value={input.aeroShakeAction}
                onChange={(event) =>
                  patchInput({
                    aeroShakeAction: event.target.value as DockInputSettings["aeroShakeAction"],
                  })
                }
              >
                <NativeSelectOption value="none">{t("None")}</NativeSelectOption>
                <NativeSelectOption value="minimizeOthers">
                  {t("Minimize other windows")}
                </NativeSelectOption>
                <NativeSelectOption value="closeOthers">
                  {t("Close other windows")}
                </NativeSelectOption>
              </NativeSelect>
            </Label>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("Timing and movement")}</CardTitle>
            <CardDescription>
              {t(
                "Delays are in milliseconds. Movement tolerance, padding, and spacing are in pixels.",
              )}
            </CardDescription>
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
              <Label appearance="unstyled" className={ROW} key={field}>
                <span>{t(label)}</span>
                <Input
                  className="w-24"
                  type="number"
                  aria-label={t(label)}
                  min={min}
                  max={max}
                  value={d[field]}
                  onChange={(e) => patchDock({ [field]: Number(e.target.value) })}
                />
              </Label>
            ))}
          </CardContent>
        </Card>
      </section>
      <section
        id={`${sectionPrefix}-media`}
        aria-labelledby={`${sectionPrefix}-media-nav`}
        hidden={section !== "media"}
        className="ot-dock-settings-panel"
      >
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
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Enable media controls")}</span>
              <Switch
                aria-label={t("Enable media controls")}
                checked={d.media?.enabled ?? false}
                onCheckedChange={(checked) =>
                  patchDock({
                    media: {
                      ...(d.media ?? {
                        enabled: false,
                        musicEnabled: false,
                        spotifyEnabled: false,
                        remoteArtwork: false,
                      }),
                      enabled: checked,
                    },
                  })
                }
              />
            </Label>
            {(["music", "spotify"] as const).map((provider) => {
              const field = provider === "music" ? "musicEnabled" : "spotifyEnabled";
              const label = provider === "music" ? "Apple Music" : "Spotify";
              const connecting =
                !!media?.pending?.[provider] ||
                media?.permissions[provider]?.status === "connecting";
              const loading = media?.permissions[provider]?.status === "loading";
              return (
                <div key={provider}>
                  <Label appearance="unstyled" className={ROW}>
                    <span>{t(`Enable ${label}`)}</span>
                    <Switch
                      aria-label={t(`Enable ${label}`)}
                      checked={d.media?.[field] ?? false}
                      onCheckedChange={(checked) =>
                        patchDock({ media: { ...d.media, [field]: checked } })
                      }
                    />
                  </Label>
                  {d.media?.[field] && media ? (
                    <div className="flex items-center justify-between gap-3">
                      <small>
                        {connecting
                          ? t("Connecting…")
                          : loading
                            ? t("Loading…")
                            : media.permissions[provider]?.status === "ready"
                              ? t("Connected")
                              : t(media.permissions[provider]?.reason || "Not connected")}
                      </small>
                      <Button
                        variant="unstyled"
                        type="button"
                        disabled={!d.media?.enabled || connecting || loading}
                        onClick={() => media.onConnect(provider)}
                      >
                        {connecting ? t("Connecting…") : t("Connect")}
                      </Button>
                    </div>
                  ) : null}
                </div>
              );
            })}
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Allow remote artwork")}</span>
              <Switch
                aria-label={t("Allow remote artwork")}
                checked={d.media?.remoteArtwork ?? false}
                onCheckedChange={(checked) =>
                  patchDock({ media: { ...d.media, remoteArtwork: checked } })
                }
              />
            </Label>
            <p className={HINT}>
              {t(
                "Remote artwork contacts the image host. Metadata and controls still work when this is off.",
              )}
            </p>
          </CardContent>
        </Card>
      </section>
      <section
        id={`${sectionPrefix}-monitor`}
        aria-labelledby={`${sectionPrefix}-monitor-nav`}
        hidden={section !== "monitor"}
        className="ot-dock-settings-panel"
      >
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
      </section>
    </div>
  );
}

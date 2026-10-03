import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { FormDisabledProvider } from "@/components/ui/form-disabled";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Tabs, TabsContent } from "@/components/ui/tabs";
import { makeT, resolveLang } from "../lib/i18n";
import type { JSONExportResult } from "../lib/json-export-bridge";
import type { LauncherProfileTransferActions } from "../lib/launcher-profile-transfer-bridge";
import type {
  DockLockDisplay,
  DockMonitorLockState,
  LauncherAppChoice,
  LauncherStatus,
  Settings as SettingsModel,
  SwitcherMode,
} from "../lib/types";
import type { WidgetCatalogDescriptor, WidgetPackageStatus } from "../lib/widget-types";
import type { WidgetPackageActions } from "../widgets/WidgetPackages";
import type { LauncherItemSettingsActions } from "./LauncherItems";
import { Onboarding } from "./Onboarding";
import {
  SETTINGS_PAGES,
  SETTINGS_TABS,
  type SettingsPage,
  SettingsSidebar,
  useSettingsNavigationOrientation,
  useSettingsTheme,
} from "./SettingsChrome";
import "./settings.css";
import {
  type AboutControl,
  type CrashControl,
  type PermissionsControl,
  PROJECT_URL,
  type TabContext,
} from "./shared";
import { AboutTab } from "./tabs/AboutTab";
import { AppearanceTab } from "./tabs/AppearanceTab";
import { BlacklistsTab } from "./tabs/BlacklistsTab";
import { ControlsTab } from "./tabs/ControlsTab";
import { DockTab } from "./tabs/DockTab";
import { FilteringTab } from "./tabs/FilteringTab";
import { GeneralTab } from "./tabs/GeneralTab";

export type { AboutControl, CrashControl, PermissionsControl };
// Re-exported so consumers (App, hooks, tests) keep one import site.
export { PROJECT_URL };

interface SettingsProps {
  disabled?: boolean;
  settings: SettingsModel;
  /** Replaced on canonical refresh/import, not on ordinary saves. */
  draftAuthority?: string | number;
  onChange: (next: SettingsModel) => void;
  onImport?: (text: string) => Promise<void>;
  onExport?: () => Promise<JSONExportResult>;
  saveError?: string | null;
  permissions?: PermissionsControl;
  about?: AboutControl;
  crash?: CrashControl;
  /**
   * Deep-link from the menubar, either a tab ("About") or a tab and a section
   * within it ("General#updates"); applied when it changes.
   */
  requestedTab?: string | null;
  dockInputError?: string;
  switcherGesturesAvailable?: boolean;
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
  diagnostics?: boolean;
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
}

const TABS = SETTINGS_TABS;
type Tab = SettingsPage;

// Settings is a controlled preferences form. It never holds the settings itself:
// every edit produces a new Settings object passed to onChange, so persistence
// and live-apply are the parent's concern. Only the active tab is local UI state.
// All tab panels stay mounted (inactive ones hidden) so the whole form is a
// single controlled surface. On first run (behavior.onboarded false, with live
// permissions available) it renders the onboarding wizard instead.
export function Settings({
  disabled = false,
  settings,
  draftAuthority = 0,
  onChange,
  onImport,
  onExport,
  saveError,
  permissions,
  about,
  crash,
  requestedTab,
  dockInputError,
  switcherGesturesAvailable = false,
  monitorLock,
  media,
  diagnostics,
  launcher,
}: SettingsProps) {
  const [tab, setTab] = useState<Tab>("General");
  const [settingsTheme, setSettingsTheme, resolvedTheme] = useSettingsTheme();
  const orientation = useSettingsNavigationOrientation();
  const settingsID = useId();
  const contentRef = useRef<HTMLDivElement>(null);
  const selectTab = (next: Tab) => {
    setTab(next);
    if (next !== tab) contentRef.current?.scrollTo?.({ top: 0 });
  };
  const [mode, setMode] = useState<SwitcherMode>("windows");
  // Updates live in a section of the General tab; the global banner and the
  // menubar's "Check for updates…" both jump there rather than to another tab.
  const [pendingUpdatesScroll, setPendingUpdatesScroll] = useState(false);
  const updatesRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!requestedTab) return;
    const [name, section] = requestedTab.split("#");
    if ((TABS as readonly string[]).includes(name)) setTab(name as Tab);
    if (section === "updates") setPendingUpdatesScroll(true);
  }, [requestedTab]);

  // Scroll only once the General tab is the active one, so the section is
  // actually on screen when it is revealed.
  useEffect(() => {
    if (!pendingUpdatesScroll || tab !== "General") return;
    updatesRef.current?.scrollIntoView?.({ block: "start", behavior: "smooth" });
    setPendingUpdatesScroll(false);
  }, [pendingUpdatesScroll, tab]);

  const showUpdateSettings = useCallback(() => {
    setTab("General");
    setPendingUpdatesScroll(true);
  }, []);
  const openURL = about?.onOpenURL ?? ((url: string) => window.open(url, "_blank", "noopener"));
  const checkUpdates = about?.onCheckUpdates ?? (() => openURL(`${PROJECT_URL}/releases`));
  const t = makeT(resolveLang(settings.behavior.language));

  const patch = (partial: Partial<SettingsModel>) => onChange({ ...settings, ...partial });
  const windowMode = {
    appearance: settings.appearance,
    behavior: {
      holdToCycle: settings.behavior.holdToCycle,
      vimKeys: settings.behavior.vimKeys,
      arrowKeys: settings.behavior.arrowKeys,
      mouseHoverSelect: settings.behavior.mouseHoverSelect,
      cursorFollowFocus: settings.behavior.cursorFollowFocus,
      hapticFeedback: settings.behavior.hapticFeedback,
      actionBindings: settings.behavior.actionBindings,
      middleClickAction: settings.behavior.middleClickAction,
      swipeUpAction: settings.behavior.swipeUpAction,
      swipeDownAction: settings.behavior.swipeDownAction,
    },
    order: settings.order,
    placement: settings.placement,
  };
  const modePrefs = mode === "apps" ? settings.appSwitcher : windowMode;
  const patchModePreferences = (p: Partial<typeof modePrefs>) =>
    mode === "apps"
      ? patch({ appSwitcher: { ...settings.appSwitcher, ...p } })
      : patch({
          appearance: p.appearance ?? settings.appearance,
          behavior: p.behavior ? { ...settings.behavior, ...p.behavior } : settings.behavior,
          order: p.order ?? settings.order,
          placement: p.placement ?? settings.placement,
        });
  const ctx: TabContext = {
    settings,
    t,
    onChange,
    patch,
    patchAppearance: (p) => patch({ appearance: { ...settings.appearance, ...p } }),
    patchBehavior: (p) => patch({ behavior: { ...settings.behavior, ...p } }),
    patchFilters: (p) => patch({ filters: { ...settings.filters, ...p } }),
    patchShortcut: (id, p) =>
      patch({ shortcuts: settings.shortcuts.map((s) => (s.id === id ? { ...s, ...p } : s)) }),
    mode,
    modeAppearance: modePrefs.appearance,
    modeBehavior: modePrefs.behavior,
    modePlacement: modePrefs.placement,
    patchModeAppearance: (p) =>
      patchModePreferences({ appearance: { ...modePrefs.appearance, ...p } }),
    patchModeBehavior: (p) => patchModePreferences({ behavior: { ...modePrefs.behavior, ...p } }),
    patchModePreferences,
  };

  if (permissions && !settings.behavior.onboarded) {
    return (
      <FormDisabledProvider disabled={disabled}>
        <div
          className={`ot-settings ot-settings-onboarding${resolvedTheme === "dark" ? " dark" : ""}`}
          data-theme={settingsTheme}
        >
          <Onboarding
            permissions={permissions}
            t={t}
            onFinish={() => ctx.patchBehavior({ onboarded: true })}
          />
        </div>
      </FormDisabledProvider>
    );
  }

  const update = about?.update;
  const progress = about?.progress;
  const checked = about?.checked;
  const installing = progress != null && progress.stage !== "error";
  const stageText: Record<string, string> = {
    downloading: t("Downloading update…"),
    installing: t("Installing update…"),
    restarting: t("Restarting…"),
  };
  const updateBanner = update ? (
    <div className="ot-settings-update">
      <Button
        variant="unstyled"
        type="button"
        aria-label="Show update settings"
        onClick={showUpdateSettings}
        className="m-0 cursor-pointer border-0 bg-transparent p-0 text-left text-[13px] text-foreground underline-offset-2 hover:underline"
      >
        {progress?.stage === "error"
          ? t("Update failed.").concat(progress.message ? ` ${progress.message}` : "")
          : installing
            ? (stageText[progress.stage] ?? t("Downloading update…"))
            : t("Version {v} is available.").replace("{v}", update.version)}
      </Button>
      <Button
        variant="default"
        size="sm"
        aria-label="Install update"
        disabled={installing}
        onClick={() => about?.onInstallUpdate?.()}
      >
        {t("Install update & restart")}
      </Button>
    </div>
  ) : null;

  // The check result stays in the Updates section: it answers a check the user
  // just ran there, unlike the banner, which is app-level news.
  const updateCheckResult = checked ? (
    <p className="my-2 text-xs leading-relaxed text-muted-foreground">
      {checked.error
        ? `${t("Could not check for updates.")} ${checked.error}`
        : t("You're up to date.")}
    </p>
  ) : null;

  return (
    <FormDisabledProvider disabled={disabled}>
      <Tabs
        appearance="unstyled"
        orientation={orientation}
        value={tab}
        onValueChange={(value) => selectTab(value as Tab)}
        className={`ot-settings${resolvedTheme === "dark" ? " dark" : ""}`}
        data-theme={settingsTheme}
      >
        <SettingsSidebar
          tab={tab}
          onSelect={selectTab}
          theme={settingsTheme}
          onTheme={setSettingsTheme}
          t={t}
          id={settingsID}
        />
        <main className="ot-settings-main" ref={contentRef}>
          <header className="ot-settings-page-header">
            <h1>{t(SETTINGS_PAGES[tab].label)}</h1>
            <p>{t(SETTINGS_PAGES[tab].description)}</p>
          </header>
          <div className="ot-settings-content">
            {saveError ? (
              <Alert appearance="unstyled" asChild>
                <p role="alert" className="ot-settings-alert">
                  {saveError}
                </p>
              </Alert>
            ) : null}
            {updateBanner}
            {tab === "Controls" || tab === "Appearance" || tab === "Filtering" ? (
              <div className="ot-settings-mode">
                <Label appearance="unstyled">
                  <span>{t("Editing")}</span>
                  <NativeSelect
                    aria-label={t("Switcher settings mode")}
                    value={mode}
                    onChange={(e) => setMode(e.target.value as SwitcherMode)}
                  >
                    <NativeSelectOption value="windows">{t("Window switcher")}</NativeSelectOption>
                    <NativeSelectOption value="apps">{t("App switcher")}</NativeSelectOption>
                  </NativeSelect>
                </Label>
                <span>
                  {t(
                    tab === "Filtering"
                      ? "Window visibility rules are shared by both switchers."
                      : tab === "Controls"
                        ? "Each opening shortcut chooses its own switcher."
                        : "Each switcher has its own appearance.",
                  )}
                </span>
              </div>
            ) : null}

            <TabsContent appearance="unstyled" forceMount value="General" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "General"}
                role="tabpanel"
                id={`${settingsID}-panel-General`}
                aria-labelledby={`${settingsID}-tab-General`}
                className="ot-settings-page"
              >
                <GeneralTab
                  ctx={ctx}
                  permissions={permissions}
                  crash={crash}
                  updatesRef={updatesRef}
                  updateCheckResult={updateCheckResult}
                  checkUpdates={checkUpdates}
                  onImport={onImport}
                  onExport={onExport}
                />
              </section>
            </TabsContent>
            <TabsContent appearance="unstyled" forceMount value="Controls" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "Controls"}
                role="tabpanel"
                id={`${settingsID}-panel-Controls`}
                aria-labelledby={`${settingsID}-tab-Controls`}
                className="ot-settings-page"
              >
                <ControlsTab ctx={ctx} switcherGesturesAvailable={switcherGesturesAvailable} />
              </section>
            </TabsContent>
            <TabsContent appearance="unstyled" forceMount value="Appearance" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "Appearance"}
                role="tabpanel"
                id={`${settingsID}-panel-Appearance`}
                aria-labelledby={`${settingsID}-tab-Appearance`}
                className="ot-settings-page"
              >
                <AppearanceTab ctx={ctx} />
              </section>
            </TabsContent>
            <TabsContent appearance="unstyled" forceMount value="Filtering" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "Filtering"}
                role="tabpanel"
                id={`${settingsID}-panel-Filtering`}
                aria-labelledby={`${settingsID}-tab-Filtering`}
                className="ot-settings-page"
              >
                <FilteringTab ctx={ctx} />
              </section>
            </TabsContent>
            <TabsContent appearance="unstyled" forceMount value="Blacklists" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "Blacklists"}
                role="tabpanel"
                id={`${settingsID}-panel-Blacklists`}
                aria-labelledby={`${settingsID}-tab-Blacklists`}
                className="ot-settings-page"
              >
                <BlacklistsTab key={draftAuthority} ctx={ctx} />
              </section>
            </TabsContent>
            <TabsContent appearance="unstyled" forceMount value="Dock" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "Dock"}
                role="tabpanel"
                id={`${settingsID}-panel-Dock`}
                aria-labelledby={`${settingsID}-tab-Dock`}
                className="ot-settings-page"
              >
                <DockTab
                  ctx={ctx}
                  permissions={permissions}
                  inputError={dockInputError}
                  monitorLock={monitorLock}
                  media={media}
                  launcher={launcher}
                />
              </section>
            </TabsContent>
            <TabsContent appearance="unstyled" forceMount value="About" asChild>
              <section
                tabIndex={-1}
                hidden={tab !== "About"}
                role="tabpanel"
                id={`${settingsID}-panel-About`}
                aria-labelledby={`${settingsID}-tab-About`}
                className="ot-settings-page"
              >
                <AboutTab
                  ctx={ctx}
                  about={about}
                  openURL={openURL}
                  checkUpdates={checkUpdates}
                  diagnostics={diagnostics}
                />
              </section>
            </TabsContent>
          </div>
        </main>
      </Tabs>
    </FormDisabledProvider>
  );
}

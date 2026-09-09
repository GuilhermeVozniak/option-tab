import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { AppSwitcher } from "./app-switcher/AppSwitcher";
import { AutomationPreviewRoute } from "./automation/AutomationPreviewRoute";
import { type DockPanelHandlers, DockPanelView } from "./dock/DockPanelView";
import { useAbout, useCrash, usePermissions } from "./hooks/useBridge";
import { LauncherItemPanelRoute } from "./launcher/LauncherItemPanelRoute";
import { LauncherRoute } from "./launcher/LauncherRoute";
import { automationPreview, onAutomationPreviewEvents } from "./lib/automation-preview-bridge";
import {
  hasBackend,
  loadSettings,
  loadSettingsState,
  onPrefsSettings,
  onPrefsTab,
  onSwitcherEvent,
  onSwitcherMaterial,
  saveSettingsAtRevision,
  saveSettingsDocumentAtRevision,
  switcher,
} from "./lib/bridge";
import { demoStateFor } from "./lib/demo";
import {
  type DockPointer,
  dock,
  onDockEvent,
  onDockInputStatus,
  onDockMaterial,
} from "./lib/dock-bridge";
import { dockLock } from "./lib/dock-lock-bridge";
import { makeT, resolveLang } from "./lib/i18n";
import { saveSettingsExport } from "./lib/json-export-bridge";
import { launcherBadges } from "./lib/launcher-badge-bridge";
import {
  launcher,
  onLauncherState,
  onLauncherStatus,
  onLauncherWidgets,
  onWidgetPackageStatus,
} from "./lib/launcher-bridge";
import {
  getLauncherInteractionCapabilities,
  launcherInteractions,
} from "./lib/launcher-interaction-bridge";
import { showLauncherItemPanel } from "./lib/launcher-item-panel-bridge";
import { mutateLauncherItems, relaunchLauncherItem } from "./lib/launcher-item-runtime-bridge";
import { launcherItemSettings } from "./lib/launcher-items-bridge";
import { launcherProfileTransfer } from "./lib/launcher-profile-transfer-bridge";
import { admitMaterialStatus, type MaterialStatus } from "./lib/material";
import { media, onMediaEvents } from "./lib/media-bridge";
import type {
  DockLockDisplay,
  DockMonitorLockState,
  DockViewState,
  LauncherAppChoice,
  LauncherInteractionCapabilities,
  LauncherStatus,
  MediaViewState,
  VisualStyle,
} from "./lib/types";
import {
  defaultSettings,
  emptyState,
  type Settings as SettingsModel,
  type SwitcherState,
} from "./lib/types";
import type { WidgetCatalogDescriptor, WidgetPackageStatus } from "./lib/widget-types";
import type { OverlayHandlers } from "./overlay/Overlay";
import { Overlay } from "./overlay/Overlay";
import "./action-notice.css";
import { Settings } from "./settings/Settings";

function route(): string {
  return typeof window !== "undefined" ? window.location.hash.replace(/^#\/?/, "") : "";
}

function isSettingsRoute(): boolean {
  return route() === "settings";
}

function isDemoRoute(): boolean {
  return route().startsWith("demo");
}
function isDockRoute(): boolean {
  return route() === "dock";
}
function mediaRouteSession(): number {
  const match = route().match(/^media\/(\d+)$/);
  return match ? Number(match[1]) : 0;
}
function automationRouteSession(): number {
  const match = route().match(/^automation\/(\d+)$/);
  return match ? Number(match[1]) : 0;
}
function launcherRouteSession(): number {
  const match = route().match(/^launcher\/(\d+)$/);
  return match ? Number(match[1]) : 0;
}

function launcherItemRouteSession(): number {
  const match = route().match(/^launcher-item\/(\d+)$/);
  return match ? Number(match[1]) : 0;
}

function LauncherChildAppRoute({ session }: { session: number }) {
  const t = useRuntimeTranslator();
  return <LauncherItemPanelRoute session={session} t={t} />;
}

function useRuntimeTranslator() {
  const [language, setLanguage] = useState("");
  useEffect(() => {
    let active = true;
    void loadSettings().then((settings) => {
      if (active && settings?.behavior) setLanguage(settings.behavior.language);
    });
    return () => {
      active = false;
    };
  }, []);
  return useMemo(() => makeT(resolveLang(language)), [language]);
}

function LauncherAppRoute({ session }: { session: number }) {
  const [language, setLanguage] = useState("");
  useEffect(() => {
    let active = true;
    void loadSettings().then((settings) => {
      if (active) setLanguage(settings?.behavior?.language ?? "");
    });
    return () => {
      active = false;
    };
  }, []);
  const resolved = resolveLang(language);
  return (
    <LauncherRoute
      session={session}
      language={resolved}
      t={makeT(resolved)}
      transport={{
        getState: launcher.state,
        badges: launcherBadges,
        activate: launcher.activate,
        relaunch: relaunchLauncherItem,
        mutate: mutateLauncherItems,
        showPanel: showLauncherItemPanel,
        subscribe: onLauncherState,
        interactions: launcherInteractions,
        widgets: {
          get: launcher.widgets,
          subscribe: onLauncherWidgets,
          options: launcher.widgetOptions,
          perform: launcher.widgetPerform,
          asset: launcher.widgetAsset,
          select: launcher.selectWidget,
        },
      }}
    />
  );
}

// demoStyle reads the style from a #demo route like "demo:appIcons" (default
// thumbnails), so all three visual styles can be screenshotted for parity.
function demoStyle(): VisualStyle {
  const s = route().split(":")[1];
  return s === "appIcons" || s === "titles" ? s : "thumbnails";
}

const noopHandlers: OverlayHandlers = {
  onAdvance: () => {},
  onReverse: () => {},
  onConfirm: () => {},
  onConfirmWindow: () => {},
  onCancel: () => {},
  onSelect: () => {},
  onSearchChange: () => {},
  onClose: () => {},
  onMinimize: () => {},
  onFullscreen: () => {},
  onQuit: () => {},
  onHide: () => {},
};

// useSettingsModel loads the persisted settings once and saves every edit
// through the bridge; without Wails it starts from defaults and saving is a
// no-op (browser/dev/tests).
function useSettingsModel() {
  const [settings, setSettings] = useState<SettingsModel>(defaultSettings);
  const [saveError, setSaveError] = useState<string | null>(null);
  const queue = useRef<Promise<void>>(Promise.resolve());
  const backendRevision = useRef(0);
  const importedRevision = useRef(0);
  const authorityEpoch = useRef(0);
  const modelEpoch = useRef(0);
  const mounted = useRef(true);
  const admissionBlocked = useRef(true);
  const refreshGeneration = useRef(0);
  const completedRefreshGeneration = useRef(0);
  const pendingImports = useRef(0);
  const [importing, setImporting] = useState(false);
  const [refreshing, setRefreshing] = useState(true);
  const [settingsStale, setSettingsStale] = useState(false);
  useEffect(() => {
    mounted.current = true;
    let active = true;
    const initialModelEpoch = modelEpoch.current;
    const initialAuthorityEpoch = authorityEpoch.current;
    const admit = (revision: number, next: SettingsModel) => {
      if (!active || revision < backendRevision.current) return;
      backendRevision.current = revision;
      authorityEpoch.current++;
      modelEpoch.current++;
      admissionBlocked.current = false;
      setSettings(next);
      setSettingsStale(false);
      setRefreshing(false);
      setSaveError(null);
    };
    const acceptRaw = (value: { generation?: number; revision: number; json: string }) => {
      try {
        const next = JSON.parse(value.json) as SettingsModel;
        const generation = value.generation ?? 0;
        if (
          Number.isSafeInteger(generation) &&
          generation >= refreshGeneration.current &&
          Number.isSafeInteger(value.revision) &&
          value.revision > 0 &&
          next?.behavior
        ) {
          refreshGeneration.current = generation;
          completedRefreshGeneration.current = Math.max(
            completedRefreshGeneration.current,
            generation,
          );
          admit(value.revision, next);
        }
      } catch {
        // A malformed event carries no settings authority.
      }
    };
    const off = onPrefsSettings(({ generation = 0 }) => {
      if (!active) return;
      if (
        !Number.isSafeInteger(generation) ||
        generation <= completedRefreshGeneration.current ||
        generation <= refreshGeneration.current
      )
        return;
      refreshGeneration.current = generation;
      authorityEpoch.current++;
      admissionBlocked.current = true;
      setRefreshing(true);
    }, acceptRaw);
    void loadSettingsState().then(async (state) => {
      if (
        !active ||
        modelEpoch.current !== initialModelEpoch ||
        authorityEpoch.current !== initialAuthorityEpoch
      )
        return;
      if (state) {
        admit(state.revision, state.settings);
      } else {
        const backend = await hasBackend();
        if (
          !active ||
          modelEpoch.current !== initialModelEpoch ||
          authorityEpoch.current !== initialAuthorityEpoch
        )
          return;
        setRefreshing(false);
        if (!backend) {
          admissionBlocked.current = false;
          return;
        }
        setSettingsStale(true);
        setSaveError("The latest settings could not be loaded.");
      }
    });
    return () => {
      active = false;
      mounted.current = false;
      admissionBlocked.current = true;
      authorityEpoch.current++;
      off();
    };
  }, []);
  const onChange = useCallback(
    (next: SettingsModel) => {
      if (pendingImports.current > 0 || settingsStale || admissionBlocked.current) return;
      const editEpoch = ++modelEpoch.current;
      const owner = authorityEpoch.current;
      setSettings(next);
      queue.current = queue.current.then(async () => {
        if (!mounted.current || owner !== authorityEpoch.current) return;
        try {
          const saved = await saveSettingsAtRevision(next, backendRevision.current);
          if (
            !mounted.current ||
            owner !== authorityEpoch.current ||
            saved.revision < backendRevision.current
          )
            return;
          backendRevision.current = saved.revision;
          if (editEpoch === modelEpoch.current) setSettings(saved.settings);
          setSaveError(null);
        } catch (error) {
          if (!mounted.current || owner !== authorityEpoch.current) return;
          const recoveryOwner = ++authorityEpoch.current;
          admissionBlocked.current = true;
          setRefreshing(true);
          const canonical = await loadSettingsState();
          if (!mounted.current || recoveryOwner !== authorityEpoch.current) return;
          if (canonical && canonical.revision >= backendRevision.current) {
            backendRevision.current = canonical.revision;
            authorityEpoch.current++;
            modelEpoch.current++;
            admissionBlocked.current = false;
            setSettings(canonical.settings);
            setSettingsStale(false);
          } else {
            setSettingsStale(true);
          }
          setRefreshing(false);
          setSaveError(`Could not save settings: ${String(error)}`);
        }
      });
    },
    [settingsStale],
  );
  const onImport = useCallback((text: string): Promise<void> => {
    if (admissionBlocked.current)
      return Promise.reject(new Error("Settings are still refreshing."));
    modelEpoch.current++;
    const owner = authorityEpoch.current;
    ++pendingImports.current;
    setImporting(true);
    const imported = queue.current
      .then(async () => {
        if (!mounted.current || owner !== authorityEpoch.current)
          throw new Error("Settings changed before this import could be saved.");
        const document = JSON.parse(text) as SettingsModel;
        if (document === null || typeof document !== "object" || Array.isArray(document))
          throw new Error("Settings must be a JSON object.");
        let canonical: Awaited<ReturnType<typeof saveSettingsDocumentAtRevision>>;
        try {
          canonical = await saveSettingsDocumentAtRevision(text, backendRevision.current);
        } catch (error) {
          if (!mounted.current || owner !== authorityEpoch.current) throw error;
          const recoveryOwner = ++authorityEpoch.current;
          admissionBlocked.current = true;
          setRefreshing(true);
          const recovered = await loadSettingsState();
          if (!mounted.current || recoveryOwner !== authorityEpoch.current) throw error;
          if (recovered && recovered.revision >= backendRevision.current) {
            backendRevision.current = recovered.revision;
            authorityEpoch.current++;
            modelEpoch.current++;
            admissionBlocked.current = false;
            setSettings(recovered.settings);
            setSettingsStale(false);
          } else {
            setSettingsStale(true);
          }
          setRefreshing(false);
          throw error;
        }
        if (
          !mounted.current ||
          owner !== authorityEpoch.current ||
          canonical.revision < backendRevision.current
        )
          throw new Error("Settings changed before this import could be saved.");
        backendRevision.current = canonical.revision;
        importedRevision.current = canonical.revision;
        setSettings(canonical.settings);
        setSaveError(null);
      })
      .finally(() => {
        --pendingImports.current;
        if (mounted.current) setImporting(pendingImports.current > 0);
      });
    // Publish every successful import in queue order, even when a later one
    // fails. Edits are disabled until the queue drains to avoid saving a
    // pre-import snapshot over newly imported settings.
    queue.current = imported.catch(() => {});
    return imported;
  }, []);
  // Export reads canonical backend data, so it must wait for earlier edits to
  // save and retain the same settings authority. It never reloads or mutates
  // settings when a native save dialog is cancelled.
  const withSavedSettings = useCallback(<T,>(operation: () => Promise<T>): Promise<T> => {
    if (admissionBlocked.current)
      return Promise.reject(new Error("Settings are still refreshing."));
    const owner = authorityEpoch.current;
    ++pendingImports.current;
    setImporting(true);
    const result = queue.current
      .then(() => {
        if (!mounted.current || owner !== authorityEpoch.current || admissionBlocked.current)
          throw new Error("Settings changed before this export could start.");
        return operation();
      })
      .finally(() => {
        --pendingImports.current;
        if (mounted.current) setImporting(pendingImports.current > 0);
      });
    queue.current = result.then(
      () => undefined,
      () => undefined,
    );
    return result;
  }, []);
  const mutateSettings = useCallback(
    <T,>(
      operation: () => Promise<T>,
      recover?: (settings: SettingsModel, value: T) => SettingsModel,
    ): Promise<T> => {
      if (admissionBlocked.current)
        return Promise.reject(new Error("Settings are still refreshing."));
      modelEpoch.current++;
      const owner = authorityEpoch.current;
      ++pendingImports.current;
      setImporting(true);
      let value: T;
      let succeeded = false;
      const mutation = queue.current
        .then(async () => {
          if (!mounted.current || owner !== authorityEpoch.current)
            throw new Error("Settings changed before this operation could start.");
          try {
            value = await operation();
            succeeded = true;
            if (mounted.current && owner === authorityEpoch.current) setSaveError(null);
            return value;
          } finally {
            // Reload even when the mutation reports a failure: a native operation
            // may have committed private-reference cleanup before returning it.
            if (mounted.current && owner === authorityEpoch.current) {
              admissionBlocked.current = true;
              setRefreshing(true);
              const canonical = await loadSettingsState();
              if (
                mounted.current &&
                owner === authorityEpoch.current &&
                canonical &&
                canonical.revision >= backendRevision.current
              ) {
                backendRevision.current = canonical.revision;
                authorityEpoch.current++;
                modelEpoch.current++;
                admissionBlocked.current = false;
                setSettings(canonical.settings);
                setSettingsStale(false);
                setRefreshing(false);
              } else if (
                mounted.current &&
                owner === authorityEpoch.current &&
                succeeded &&
                recover
              ) {
                setSettings((current) => recover(current, value));
                setSettingsStale(true);
                setSaveError("Settings changed, but the latest settings could not be reloaded.");
              } else if (mounted.current && owner === authorityEpoch.current) {
                setSettingsStale(true);
                setSaveError("Settings changed, but the latest settings could not be reloaded.");
              }
              if (mounted.current && owner === authorityEpoch.current) setRefreshing(false);
            }
          }
        })
        .finally(() => {
          --pendingImports.current;
          if (mounted.current) setImporting(pendingImports.current > 0);
        });
      queue.current = mutation.then(
        () => undefined,
        () => undefined,
      );
      return mutation;
    },
    [],
  );
  const reload = useCallback(async () => {
    const owner = authorityEpoch.current;
    admissionBlocked.current = true;
    setRefreshing(true);
    const next = await loadSettingsState();
    if (
      mounted.current &&
      owner === authorityEpoch.current &&
      next &&
      next.revision >= backendRevision.current
    ) {
      modelEpoch.current++;
      authorityEpoch.current++;
      backendRevision.current = next.revision;
      admissionBlocked.current = false;
      setSettings(next.settings);
      setSettingsStale(false);
      setRefreshing(false);
      setSaveError(null);
    }
    if (mounted.current && owner === authorityEpoch.current) setRefreshing(false);
  }, []);
  return {
    settings,
    settingsAuthority: `${authorityEpoch.current}:${importedRevision.current}`,
    onChange,
    onImport,
    withSavedSettings,
    mutateSettings,
    saveError,
    importing: importing || refreshing,
    settingsStale,
    reload,
  };
}

// App is the desktop frontend shell. Two Wails windows load it: the overlay
// window (default route) subscribes to the Go controller's events and renders
// the switcher; the preferences window (#/settings) renders the settings form.
// It holds no business logic.
export default function App() {
  if (isSettingsRoute()) return <SettingsRoute />;
  if (isDockRoute()) return <DockRoute />;
  if (automationRouteSession()) return <AutomationRoute session={automationRouteSession()} />;
  if (mediaRouteSession()) return <MediaRoute session={mediaRouteSession()} />;
  if (launcherItemRouteSession())
    return <LauncherChildAppRoute session={launcherItemRouteSession()} />;
  if (launcherRouteSession()) return <LauncherAppRoute session={launcherRouteSession()} />;
  if (isDemoRoute()) {
    return (
      <div className="ot-demo-backdrop">
        <Overlay state={demoStateFor(demoStyle())} handlers={noopHandlers} />
      </div>
    );
  }
  return <OverlayRoute />;
}

function AutomationRoute({ session }: { session: number }) {
  return <AutomationPreviewRoute session={session} t={useRuntimeTranslator()} />;
}

function OverlayRoute() {
  const t = useRuntimeTranslator();
  const [state, setState] = useState<SwitcherState>(emptyState);
  const [actionError, setActionError] = useState<string | null>(null);
  const actionRevision = useRef(0);
  const [thumbs, setThumbs] = useState<Record<string, string>>({});
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const currentSession = useRef(0);
  const activeSession = useRef(0);
  const retiredSession = useRef(0);
  const latestRevision = useRef(0);
  const scopedSeen = useRef(false);
  // The overlay window never becomes key (the app is not activated on show), so
  // in the real app keyboard input arrives as native-tap "switcher:key" events.
  // The DOM listener in Overlay is only a fallback for browser dev, enabled
  // until the backend probe answers.
  const [nativeKeys, setNativeKeys] = useState(true);
  const [materialStatus, setMaterialStatus] = useState<MaterialStatus | null>(null);
  const materialSession = useRef(0);
  useEffect(() => {
    let active = true;
    hasBackend().then((ok) => {
      if (active) setNativeKeys(ok);
    });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    const acceptState = (next: SwitcherState) => {
      const session = next.session ?? 0;
      const revision = next.revision ?? 0;
      if (session === 0) {
        if (scopedSeen.current) return;
      } else {
        scopedSeen.current = true;
        if (session < currentSession.current || session <= retiredSession.current) return;
        if (session === currentSession.current && revision < latestRevision.current) return;
      }
      const changed = session !== activeSession.current;
      if (session > currentSession.current) currentSession.current = session;
      latestRevision.current = revision;
      activeSession.current = session;
      if (changed) {
        ++actionRevision.current;
        setActionError(null);
        setThumbs({});
        setPreviews({});
        materialSession.current = session;
        setMaterialStatus(null);
        if (session > 0) {
          void switcher
            .materialStatus(session)
            .then((next) => {
              if (materialSession.current === session)
                setMaterialStatus((old) => admitMaterialStatus(old, next, session));
            })
            .catch(() => {});
        }
      }
      setState(next);
    };
    return onSwitcherEvent({
      onShow: (next) => {
        if ((next.session ?? 0) === 0 && !scopedSeen.current) {
          ++actionRevision.current;
          setActionError(null);
          setThumbs({});
          setPreviews({});
        }
        acceptState(next);
      },
      onUpdate: acceptState,
      onHide: (session, revision) => {
        if (session === 0) {
          if (scopedSeen.current) return;
        } else {
          scopedSeen.current = true;
          if (session < currentSession.current) return;
          if (session === currentSession.current && revision < latestRevision.current) return;
          currentSession.current = session;
          latestRevision.current = revision;
          retiredSession.current = Math.max(retiredSession.current, session);
        }
        activeSession.current = 0;
        ++actionRevision.current;
        setActionError(null);
        setThumbs({});
        setPreviews({});
        materialSession.current = 0;
        setMaterialStatus(null);
        setState((s) => ({ ...s, open: false }));
      },
      onThumbnails: (session, next) => {
        if (
          (session === 0 && !scopedSeen.current) ||
          (session > 0 && session === activeSession.current)
        )
          setThumbs((prev) => ({ ...prev, ...next }));
      },
      onPreview: (session, next) => {
        if (
          (session === 0 && !scopedSeen.current) ||
          (session > 0 && session === activeSession.current)
        )
          setPreviews((prev) => ({ ...prev, ...next }));
      },
      onError: (message) => setActionError(message),
    });
  }, []);

  useEffect(
    () =>
      onSwitcherMaterial((next) => {
        const session = materialSession.current;
        if (session > 0) setMaterialStatus((old) => admitMaterialStatus(old, next, session));
      }),
    [],
  );

  const stateWithThumbs = useMemo<SwitcherState>(
    () => ({
      ...state,
      entries: state.entries.map((e) => {
        const key = String(e.windowId);
        const thumbnail = thumbs[key] ?? e.thumbnail;
        const preview = previews[key] ?? e.preview;
        return thumbnail !== e.thumbnail || preview !== e.preview
          ? { ...e, thumbnail, preview }
          : e;
      }),
    }),
    [state, thumbs, previews],
  );

  const performAction = useCallback(async (kind: string, windowId: number, appId: number) => {
    const revision = ++actionRevision.current;
    setActionError(null);
    try {
      const result = await switcher.performAction(kind, windowId, appId);
      if (revision === actionRevision.current && result.failures.length > 0) {
        const accepted =
          result.succeeded > 0
            ? `${result.succeeded} action request${result.succeeded === 1 ? "" : "s"} accepted. `
            : "";
        setActionError(accepted + result.failures.map((failure) => failure.error).join("; "));
      }
    } catch (error) {
      if (revision === actionRevision.current) {
        setActionError(error instanceof Error ? error.message : String(error));
      }
    }
  }, []);
  const windowAction = useCallback(
    (kind: string, windowId: number) => {
      const entry = state.entries.find((e) => e.windowId === windowId);
      if (entry) void performAction(kind, windowId, entry.appId);
    },
    [state.entries, performAction],
  );
  const performCommit = useCallback(async (commit: () => Promise<unknown>) => {
    const revision = ++actionRevision.current;
    setActionError(null);
    try {
      await commit();
    } catch (error) {
      if (revision === actionRevision.current)
        setActionError(error instanceof Error ? error.message : String(error));
    }
  }, []);

  const handlers = useMemo<OverlayHandlers>(
    () => ({
      onAdvance: () => void switcher.advance(),
      onReverse: () => void switcher.reverse(),
      onConfirm: () => void performCommit(() => switcher.confirm()),
      onConfirmWindow: (windowId) => void performCommit(() => switcher.confirmWindow(windowId)),
      onCancel: () => void switcher.cancel(),
      onSelect: (i) => void switcher.select(i),
      onSearchChange: (q) => void switcher.setSearch(q),
      onClose: (windowId) => windowAction("close", windowId),
      onMinimize: (windowId) => windowAction("minimize", windowId),
      onFullscreen: (windowId) => windowAction("fullscreen", windowId),
      onQuit: (appId) => void performAction("quit", 0, appId),
      onHide: (appId) => void performAction("hide", 0, appId),
      onAction: (kind, windowId, appId) => void performAction(kind, windowId, appId),
      onSelectApp: (appId) => void switcher.selectApp(appId),
      onSelectAppWindow: (windowId) => void switcher.selectAppWindow(windowId),
      onConfirmApp: (appId) => void performCommit(() => switcher.confirmApp(appId)),
    }),
    [windowAction, performAction, performCommit],
  );

  return (
    <>
      {(stateWithThumbs.mode ?? "windows") === "apps" ? (
        <AppSwitcher
          state={stateWithThumbs}
          handlers={handlers}
          nativeKeys={nativeKeys}
          t={t}
          material={{
            status: materialStatus,
            onRect: (rect) => void switcher.materialRect(rect).catch(() => {}),
          }}
        />
      ) : (
        <Overlay
          state={stateWithThumbs}
          handlers={handlers}
          nativeKeys={nativeKeys}
          material={{
            status: materialStatus,
            onRect: (rect) => void switcher.materialRect(rect).catch(() => {}),
          }}
        />
      )}
      {state.open && actionError ? (
        <div className="ot-action-notice" role="alert">
          <span>{actionError}</span>
          <button
            type="button"
            onClick={() => setActionError(null)}
            aria-label="Dismiss action error"
          >
            ×
          </button>
        </div>
      ) : null}
    </>
  );
}

function DockRoute() {
  const t = useRuntimeTranslator();
  const [state, setState] = useState<DockViewState | null>(null);
  const [frames, setFrames] = useState<Record<string, string>>({});
  const [nativePointer, setNativePointer] = useState<DockPointer | null>(null);
  const [materialStatus, setMaterialStatus] = useState<MaterialStatus | null>(null);
  const currentSession = useRef(0);
  const activeSession = useRef(0);
  const retiredSession = useRef(0);
  const latestRevision = useRef(0);
  const actionRevision = useRef(0);
  const pointerSequence = useRef(0);
  const pendingPointer = useRef<DockPointer | null>(null);
  const activeMediaSession = useRef(0);
  const latestMediaRevision = useRef(0);
  const retiredMediaSessions = useRef(new Set<number>());
  const mediaProgressSequence = useRef(0);
  const acceptEmbeddedMedia = useCallback((next: DockViewState) => {
    const embedded = next.media;
    if (embedded && retiredMediaSessions.current.has(embedded.session)) return false;
    const session = embedded?.session ?? 0;
    const revision = embedded?.revision ?? 0;
    if (session !== activeMediaSession.current || revision !== latestMediaRevision.current)
      mediaProgressSequence.current = 0;
    activeMediaSession.current = session;
    latestMediaRevision.current = revision;
    return true;
  }, []);
  const acceptShow = useCallback(
    (next: DockViewState) => {
      if (!next || next.session < currentSession.current || next.session <= retiredSession.current)
        return;
      const revision = next.revision ?? 0;
      if (next.session === currentSession.current && revision < latestRevision.current) return;
      if (next.session > currentSession.current) {
        currentSession.current = next.session;
        ++actionRevision.current;
        setFrames({});
        setNativePointer(null);
        pointerSequence.current = 0;
        setMaterialStatus(null);
        void dock
          .materialStatus(next.session)
          .then((status) => {
            if (activeSession.current === next.session)
              setMaterialStatus((old) => admitMaterialStatus(old, status, next.session));
          })
          .catch(() => {});
      }
      latestRevision.current = revision;
      activeSession.current = next.session;
      if (!acceptEmbeddedMedia(next)) {
        setState(null);
        return;
      }
      setState(next);
      const pointer =
        pendingPointer.current?.session === next.session &&
        (next.pointer?.sequence ?? 0) < pendingPointer.current.sequence
          ? pendingPointer.current
          : next.pointer;
      if (pendingPointer.current?.session === next.session) pendingPointer.current = null;
      if (pointer?.session === next.session && pointer.sequence > pointerSequence.current) {
        pointerSequence.current = pointer.sequence;
        setNativePointer(pointer);
      }
    },
    [acceptEmbeddedMedia],
  );
  const acceptUpdate = useCallback(
    (next: DockViewState) => {
      const revision = next?.revision ?? 0;
      if (next?.session === activeSession.current && revision >= latestRevision.current) {
        latestRevision.current = revision;
        if (!acceptEmbeddedMedia(next)) {
          setState(null);
          return;
        }
        setState(next);
        const pointer =
          pendingPointer.current?.session === next.session &&
          (next.pointer?.sequence ?? 0) < pendingPointer.current.sequence
            ? pendingPointer.current
            : next.pointer;
        if (pendingPointer.current?.session === next.session) pendingPointer.current = null;
        if (pointer?.session === next.session && pointer.sequence > pointerSequence.current) {
          pointerSequence.current = pointer.sequence;
          setNativePointer(pointer);
        }
      }
    },
    [acceptEmbeddedMedia],
  );
  const retire = useCallback((session: number, revision: number) => {
    if (session < currentSession.current) return;
    if (session === currentSession.current && revision < latestRevision.current) return;
    currentSession.current = session;
    latestRevision.current = revision;
    retiredSession.current = Math.max(retiredSession.current, session);
    activeSession.current = 0;
    activeMediaSession.current = 0;
    ++actionRevision.current;
    setState(null);
    setFrames({});
    setNativePointer(null);
    setMaterialStatus(null);
    pointerSequence.current = 0;
    if ((pendingPointer.current?.session ?? 0) <= session) pendingPointer.current = null;
  }, []);
  useEffect(
    () =>
      onDockMaterial((next) => {
        const session = activeSession.current;
        if (session > 0) setMaterialStatus((old) => admitMaterialStatus(old, next, session));
      }),
    [],
  );
  useEffect(() => {
    let active = true;
    const off = onDockEvent({
      show: acceptShow,
      update: acceptUpdate,
      hide: (session, revision) => {
        retire(session, revision);
      },
      frames: (session, next) => {
        if (session === activeSession.current) setFrames((old) => ({ ...old, ...next }));
      },
      error: (session, revision, message) => {
        if (session === activeSession.current && revision >= latestRevision.current) {
          latestRevision.current = revision;
          setState((old) => (old ? { ...old, revision, error: message } : old));
        }
      },
      pointer: (pointer) => {
        if (pointer.session !== activeSession.current) {
          const pending = pendingPointer.current;
          if (
            pointer.session >= currentSession.current &&
            pointer.session > retiredSession.current &&
            (!pending ||
              pointer.session > pending.session ||
              (pointer.session === pending.session && pointer.sequence > pending.sequence))
          )
            pendingPointer.current = pointer;
          return;
        }
        if (pointer.sequence <= pointerSequence.current) return;
        pointerSequence.current = pointer.sequence;
        setNativePointer(pointer);
      },
    });
    void dock
      .state()
      .then((snapshot) => {
        if (active && snapshot) {
          if (snapshot.open === false) retire(snapshot.session, snapshot.revision ?? 0);
          else acceptShow(snapshot);
        }
      })
      .catch(() => {});
    return () => {
      active = false;
      off();
    };
  }, [acceptShow, acceptUpdate, retire]);
  useEffect(
    () =>
      onMediaEvents({
        update: (next) => {
          if (
            next.session !== activeMediaSession.current ||
            next.revision < latestMediaRevision.current
          )
            return;
          latestMediaRevision.current = next.revision;
          mediaProgressSequence.current = 0;
          setState((old) =>
            old && old.contentKind === "media" && old.media?.session === next.session
              ? { ...old, media: next }
              : old,
          );
        },
        hide: (session, revision) => {
          if (session !== activeMediaSession.current || revision < latestMediaRevision.current)
            return;
          activeMediaSession.current = 0;
          latestMediaRevision.current = revision;
          retiredMediaSessions.current.add(session);
          setState((old) => (old?.media?.session === session ? { ...old, media: undefined } : old));
        },
        progress: (next) =>
          setState((old) => {
            if (
              !old?.media ||
              old.media.session !== next.session ||
              old.media.revision !== next.revision ||
              next.sequence <= mediaProgressSequence.current
            )
              return old;
            mediaProgressSequence.current = next.sequence;
            return {
              ...old,
              media: { ...old.media, positionMS: next.positionMS, activeCue: next.activeCue },
            };
          }),
      }),
    [retire],
  );
  const visible = useMemo(
    () =>
      state
        ? {
            ...state,
            entries: state.entries.map((entry) => ({
              ...entry,
              thumbnail: frames[String(entry.windowId)] ?? entry.thumbnail,
            })),
          }
        : null,
    [state, frames],
  );
  const run = useCallback(
    async (
      session: number,
      request: () => Promise<{ succeeded: number; failures: Array<{ error: string }> }>,
    ) => {
      const revision = ++actionRevision.current;
      setState((old) => (old && old.session === session ? { ...old, error: undefined } : old));
      try {
        const result = await request();
        if (session !== activeSession.current || revision !== actionRevision.current) return;
        if (result.failures.length)
          setState((old) =>
            old && old.session === session
              ? { ...old, error: result.failures.map((failure) => failure.error).join("; ") }
              : old,
          );
      } catch (error) {
        if (session === activeSession.current && revision === actionRevision.current)
          setState((old) =>
            old && old.session === session
              ? { ...old, error: error instanceof Error ? error.message : String(error) }
              : old,
          );
      }
    },
    [],
  );
  const runMedia = useCallback(
    async <T,>(session: number, revision: number, request: () => Promise<T>) => {
      try {
        return await request();
      } catch (error) {
        if (session === activeMediaSession.current && revision === latestMediaRevision.current)
          setState((old) =>
            old?.media?.session === session && old.media.revision === revision
              ? {
                  ...old,
                  media: {
                    ...old.media,
                    error: error instanceof Error ? error.message : String(error),
                  },
                }
              : old,
          );
        return undefined;
      }
    },
    [],
  );
  const runMediaVoid = useCallback(
    (session: number, revision: number, request: () => Promise<unknown>): Promise<void> =>
      runMedia(session, revision, request).then(() => undefined),
    [runMedia],
  );
  const handlers = useMemo<DockPanelHandlers>(
    () => ({
      onSelectWindow: (session, id) => {
        if (session === activeSession.current) void dock.select(session, id);
      },
      onSelectContent: (session, revision, kind) => {
        if (session !== activeSession.current || revision !== latestRevision.current) return;
        void dock.selectContent(session, revision, kind).catch((error) => {
          if (session === activeSession.current && revision === latestRevision.current)
            setState((old) =>
              old?.session === session && old.revision === revision
                ? { ...old, error: error instanceof Error ? error.message : String(error) }
                : old,
            );
        });
      },
      onFocusWindow: (session, id, appId) => {
        if (session === activeSession.current)
          void run(session, () => dock.focus(session, id, appId));
      },
      onAction: (session, kind, id, appId) => {
        if (session === activeSession.current)
          void run(session, () => dock.action(session, kind, id, appId));
      },
      onSize: (session, width, height) => {
        if (session === activeSession.current)
          void dock.size(session, width, height).catch(() => {});
      },
      onRegions: (session, revision, regions) => {
        if (session === activeSession.current)
          void dock.regions(session, revision, regions).catch(() => {});
      },
      onBeginDrag: (request) => {
        if (request.session === activeSession.current)
          return dock.beginDrag(
            request.session,
            request.gesture,
            request.windowId,
            request.appId,
            request.pointerX,
            request.pointerY,
            request.grabX,
            request.grabY,
          );
      },
      onCancelDrag: (session, gesture) => dock.cancelDrag(session, gesture),
      onFolderSort: (session, revision, field, direction, foldersFirst) => {
        if (session === activeSession.current && revision === latestRevision.current)
          return dock.folderSort(session, revision, field, direction, foldersFirst);
      },
      onRequestFolderAccess: (session, revision) => {
        if (session === activeSession.current && revision === latestRevision.current)
          return dock.requestFolderAccess(session, revision);
      },
      onCancelFolderAccess: (session, revision) => dock.cancelFolderAccess(session, revision),
      onOpenFolderEntry: (session, revision, itemID) => {
        if (session === activeSession.current && revision === latestRevision.current)
          return dock.openFolderEntry(session, revision, itemID);
      },
      media: {
        onAction: (session, revision, kind, position) =>
          session === activeMediaSession.current && revision === latestMediaRevision.current
            ? runMediaVoid(session, revision, () => media.action(session, revision, kind, position))
            : undefined,
        onPin: (session, revision) =>
          runMedia(session, revision, () => media.pin(session, revision)).then((value) =>
            typeof value === "number" ? value : 0,
          ),
        onClose: (session, revision) =>
          runMediaVoid(session, revision, () => media.close(session, revision)),
        onSize: (session, revision, width, height) =>
          runMediaVoid(session, revision, () => media.size(session, revision, width, height)),
        onImport: (session, revision) =>
          runMediaVoid(session, revision, () => media.importLyrics(session, revision)),
        onCancelImport: (session, revision) =>
          runMediaVoid(session, revision, () => media.cancelImport(session, revision)),
        onReload: (session, revision) =>
          runMediaVoid(session, revision, () => media.reloadLyrics(session, revision)),
        onRemove: (session, revision) =>
          runMediaVoid(session, revision, () => media.removeLyrics(session, revision)),
        onOffset: (session, revision, offset) =>
          runMediaVoid(session, revision, () => media.offset(session, revision, offset)),
      },
    }),
    [run, runMedia, runMediaVoid],
  );
  return visible ? (
    <DockPanelView
      state={visible}
      handlers={handlers}
      nativePointer={nativePointer}
      materialStatus={materialStatus}
      t={t}
    />
  ) : null;
}

function MediaRoute({ session }: { session: number }) {
  const t = useRuntimeTranslator();
  const [state, setState] = useState<MediaViewState | null>(null);
  const revision = useRef(0),
    progress = useRef(0),
    retired = useRef(false);
  const accept = useCallback(
    (next: MediaViewState) => {
      if (next.session !== session || retired.current || next.revision < revision.current) return;
      if (next.revision > revision.current) progress.current = 0;
      revision.current = next.revision;
      setState(next);
    },
    [session],
  );
  const request = useCallback(
    async <T,>(atRevision: number, operation: () => Promise<T>): Promise<T | undefined> => {
      try {
        return await operation();
      } catch (error) {
        if (!retired.current && revision.current === atRevision)
          setState((old) =>
            old?.session === session && old.revision === atRevision
              ? { ...old, error: error instanceof Error ? error.message : String(error) }
              : old,
          );
        return undefined;
      }
    },
    [session],
  );
  useEffect(() => {
    let mounted = true;
    const off = onMediaEvents({
      update: accept,
      hide: (s, r) => {
        if (s === session && r >= revision.current) {
          retired.current = true;
          revision.current = r;
          setState(null);
        }
      },
      progress: (p) => {
        if (
          p.session !== session ||
          p.revision !== revision.current ||
          p.sequence <= progress.current ||
          retired.current
        )
          return;
        progress.current = p.sequence;
        setState((old) =>
          old ? { ...old, positionMS: p.positionMS, activeCue: p.activeCue } : old,
        );
      },
    });
    void media
      .state(session)
      .then((s) => {
        if (mounted && s) accept(s);
      })
      .catch(() => {});
    return () => {
      mounted = false;
      off();
    };
  }, [accept, session]);
  if (!state?.open) return null;
  return (
    <DockPanelView
      state={{
        session,
        revision: state.revision,
        open: state.open,
        item: {
          appId: 0,
          bundleId: "",
          path: "",
          title: "",
          bounds: { x: 0, y: 0, w: 0, h: 0 },
          screenId: 0,
          edge: "",
          kind: "media",
        },
        entries: [],
        selectedWindowId: 0,
        appearance: state.appearance,
        emptyReason: "",
        contentKind: "media",
        media: state,
      }}
      handlers={{
        onSelectWindow: () => {},
        onFocusWindow: () => {},
        onAction: () => {},
        onSize: () => {},
        media: {
          onAction: (s, r, kind, position) => request(r, () => media.action(s, r, kind, position)),
          onPin: (s, r) => request(r, () => media.pin(s, r)).then((value) => value ?? 0),
          onClose: (s, r) => request(r, () => media.close(s, r)),
          onSize: (s, r, width, height) => request(r, () => media.size(s, r, width, height)),
          onImport: (s, r) => request(r, () => media.importLyrics(s, r)),
          onCancelImport: (s, r) => request(r, () => media.cancelImport(s, r)),
          onReload: (s, r) => request(r, () => media.reloadLyrics(s, r)),
          onRemove: (s, r) => request(r, () => media.removeLyrics(s, r)),
          onOffset: (s, r, offset) => request(r, () => media.offset(s, r, offset)),
        },
      }}
      t={t}
    />
  );
}

// SettingsRoute renders the preferences window's contents. The window is a
// regular titled window (its own Wails window since the Wails v3 migration);
// the menubar can deep-link a tab via the "prefs:tab" event.
function SettingsRoute() {
  const {
    settings,
    settingsAuthority,
    onChange,
    onImport,
    withSavedSettings,
    mutateSettings,
    saveError,
    importing,
    settingsStale,
    reload,
  } = useSettingsModel();
  const perms = usePermissions();
  const about = useAbout();
  const crash = useCrash();
  const [requestedTab, setRequestedTab] = useState<string | null>(null);
  const [dockInputError, setDockInputError] = useState("");
  const dockInputRevision = useRef(0);
  const [lockState, setLockState] = useState<DockMonitorLockState>();
  const [lockDisplays, setLockDisplays] = useState<DockLockDisplay[]>([]);
  const [lockLoadError, setLockLoadError] = useState("");
  const [lockPlacementError, setLockPlacementError] = useState("");
  const [lockPlacePending, setLockPlacePending] = useState(false);
  const [mediaPermissions, setMediaPermissions] = useState<
    Record<string, { status: string; reason: string }>
  >({ music: { status: "loading", reason: "" }, spotify: { status: "loading", reason: "" } });
  const [mediaConnectPending, setMediaConnectPending] = useState<Record<string, boolean>>({});
  const mediaConnectJobs = useRef(new Set<string>());
  const mediaPermissionVersions = useRef<Record<string, number>>({});
  const mediaPermissionMounted = useRef(true);
  const [diagnosticsAvailable, setDiagnosticsAvailable] = useState(false);
  const [launcherStatus, setLauncherStatus] = useState<LauncherStatus>();
  const [launcherAppChoices, setLauncherAppChoices] = useState<LauncherAppChoice[]>([]);
  const [launcherInteractionCapabilities, setLauncherInteractionCapabilities] =
    useState<LauncherInteractionCapabilities>();
  const [launcherError, setLauncherError] = useState("");
  const [widgetCatalog, setWidgetCatalog] = useState<WidgetCatalogDescriptor[]>([]);
  const [widgetPackageStatus, setWidgetPackageStatus] = useState<WidgetPackageStatus>({
    available: false,
    busy: false,
    reason: "",
  });
  const widgetPackageLoad = useRef(0);
  const lockMark = useRef<[number, number, number]>([0, 0, 0]);
  const lockSeen = useRef(false);

  useEffect(() => onPrefsTab(setRequestedTab), []);
  useEffect(() => {
    let active = true;
    const accept = (status: LauncherStatus) => {
      if (active)
        setLauncherStatus((old) =>
          !old ||
          status.epoch > old.epoch ||
          (status.epoch === old.epoch && status.revision >= old.revision)
            ? status
            : old,
        );
    };
    const off = onLauncherStatus(accept);
    void launcher
      .status()
      .then(accept)
      .catch((error) => active && setLauncherError(String(error)));
    return () => {
      active = false;
      off();
    };
  }, []);
  useEffect(() => {
    let active = true;
    setLauncherInteractionCapabilities(undefined);
    void getLauncherInteractionCapabilities()
      .then((capabilities) => {
        if (active) setLauncherInteractionCapabilities(capabilities);
      })
      .catch(() => {
        if (active) setLauncherInteractionCapabilities(undefined);
      });
    return () => {
      active = false;
    };
  }, [launcherStatus?.epoch, launcherStatus?.status]);
  const refreshWidgetPackages = useCallback(async () => {
    const request = ++widgetPackageLoad.current;
    const [status, catalog] = await Promise.allSettled([
      launcher.packageStatus(),
      launcher.catalog(),
    ]);
    if (request !== widgetPackageLoad.current) return;
    if (status.status === "fulfilled") setWidgetPackageStatus(status.value);
    if (catalog.status === "fulfilled") setWidgetCatalog(catalog.value);
  }, []);
  useEffect(() => {
    void refreshWidgetPackages();
    const off = onWidgetPackageStatus((status) => {
      setWidgetPackageStatus(status);
      void refreshWidgetPackages();
    });
    return off;
  }, [refreshWidgetPackages]);
  useEffect(() => {
    let active = true;
    void launcher
      .appChoices()
      .then((choices) => {
        if (active) setLauncherAppChoices(choices);
      })
      .catch(() => {
        if (active) setLauncherAppChoices([]);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    let active = true;
    void hasBackend().then((available) => {
      if (active) setDiagnosticsAvailable(available);
    });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    let active = true;
    mediaPermissionMounted.current = true;
    const versions = { ...mediaPermissionVersions.current };
    const acceptSnapshot = (value: typeof mediaPermissions) => {
      if (!active) return;
      setMediaPermissions((old) => {
        const next = { ...old };
        for (const provider of ["music", "spotify"]) {
          if ((mediaPermissionVersions.current[provider] ?? 0) === (versions[provider] ?? 0))
            next[provider] = value[provider] ?? { status: "permissionRequired", reason: "" };
        }
        return next;
      });
    };
    void media
      .permissions()
      .then(acceptSnapshot)
      .catch(() =>
        acceptSnapshot({
          music: { status: "unavailable", reason: "Media unavailable" },
          spotify: { status: "unavailable", reason: "Media unavailable" },
        }),
      );
    const off = onMediaEvents({
      update: () => {},
      hide: () => {},
      progress: () => {},
      permission: (provider, status, reason) => {
        if (!active) return;
        mediaPermissionVersions.current[provider] =
          (mediaPermissionVersions.current[provider] ?? 0) + 1;
        setMediaPermissions((old) => ({ ...old, [provider]: { status, reason } }));
      },
    });
    return () => {
      active = false;
      mediaPermissionMounted.current = false;
      off();
    };
  }, []);
  useEffect(() => {
    let active = true;
    void dock.state().then((state) => {
      const revision = Number(state?.revision ?? 0);
      if (!active || !state || revision < dockInputRevision.current) return;
      dockInputRevision.current = revision;
      setDockInputError(state.error ?? "");
    });
    const off = onDockInputStatus((revision, message) => {
      if (!active || revision < dockInputRevision.current) return;
      dockInputRevision.current = revision;
      setDockInputError(message);
    });
    return () => {
      active = false;
      off();
    };
  }, []);
  useEffect(() => {
    let active = true;
    const accept = (state: DockMonitorLockState) => {
      const next: [number, number, number] = [state.session, state.revision, state.sequence];
      const old = lockMark.current;
      if (
        !active ||
        next[0] < old[0] ||
        (next[0] === old[0] &&
          (next[1] < old[1] ||
            (next[1] === old[1] && (next[2] < old[2] || (lockSeen.current && next[2] === old[2])))))
      )
        return;
      lockSeen.current = true;
      lockMark.current = next;
      setLockState(state);
      if (state.generation > 0) {
        setLockDisplays(state.displays ?? []);
        setLockLoadError("");
      }
    };
    const off = dockLock.onState(accept);
    void dockLock
      .state()
      .then(accept)
      .catch((e) => active && !lockSeen.current && setLockLoadError(String(e)));
    void dockLock
      .displays()
      .then((value) => active && setLockDisplays(value))
      .catch((e) => active && !lockSeen.current && setLockLoadError(String(e)));
    return () => {
      active = false;
      off();
    };
  }, []);

  const launcherItemActions = useMemo(
    () => ({
      ...launcherItemSettings,
      save: (
        profileID: string,
        itemRevision: string,
        items: Parameters<typeof launcherItemSettings.save>[2],
      ) =>
        mutateSettings(
          () => launcherItemSettings.save(profileID, itemRevision, items),
          (current, result) => ({
            ...current,
            replacementDock: {
              ...current.replacementDock,
              profiles: current.replacementDock.profiles.map((profile) =>
                profile.id === profileID ? { ...profile, items: result.items } : profile,
              ),
            },
          }),
        ),
    }),
    [mutateSettings],
  );

  const launcherProfileActions = useMemo(
    () => ({
      ...launcherProfileTransfer,
      exportProfile: (profileID: string) =>
        withSavedSettings(() => launcherProfileTransfer.exportProfile(profileID)),
      importProfile: (document: string, digest: string, expectedRevision: string) =>
        mutateSettings(
          async () => {
            const result = await launcherProfileTransfer.importProfile(
              document,
              digest,
              expectedRevision,
            );
            const canonical = JSON.parse(result.settingsJSON) as SettingsModel;
            if (
              !canonical?.behavior ||
              !canonical.replacementDock?.profiles?.some((p) => p.id === result.profileID)
            ) {
              throw new Error("Could not reload imported settings.");
            }
            return { ...result, canonical };
          },
          (_current, result) => result.canonical,
        ),
    }),
    [mutateSettings, withSavedSettings],
  );

  return (
    <>
      {settingsStale ? (
        <Button type="button" onClick={() => void reload()}>
          Reload settings
        </Button>
      ) : null}
      <fieldset
        disabled={importing || settingsStale}
        aria-busy={importing}
        className="m-0 min-w-0 border-0 p-0"
      >
        <Settings
          settings={settings}
          draftAuthority={settingsAuthority}
          onChange={onChange}
          onImport={onImport}
          onExport={() => withSavedSettings(saveSettingsExport)}
          saveError={saveError}
          permissions={perms}
          about={about}
          crash={crash}
          requestedTab={requestedTab}
          dockInputError={dockInputError}
          monitorLock={{
            state: lockState,
            displays: lockDisplays,
            error: lockPlacementError || lockLoadError,
            pending: lockPlacePending,
            onEnable: () => {
              if (perms && perms.state.accessibility !== "granted")
                perms.onRequest("accessibility");
            },
            onPlace: (session, revision, generation) => {
              setLockPlacementError("");
              setLockPlacePending(true);
              void dockLock
                .place(session, revision, generation)
                .then((r) => {
                  if (!r.verified) setLockPlacementError(r.reason || r.status);
                })
                .catch((e) => setLockPlacementError(String(e)))
                .finally(() => setLockPlacePending(false));
            },
            onCancel: () => {
              void dockLock.cancel().catch((e) => setLockPlacementError(String(e)));
            },
          }}
          media={{
            permissions: mediaPermissions,
            pending: mediaConnectPending,
            onConnect: (provider) => {
              if (
                mediaConnectJobs.current.has(provider) ||
                ["loading", "connecting"].includes(mediaPermissions[provider]?.status) ||
                !settings.dock.media?.enabled ||
                !(provider === "music"
                  ? settings.dock.media.musicEnabled
                  : settings.dock.media.spotifyEnabled)
              )
                return;
              mediaConnectJobs.current.add(provider);
              setMediaConnectPending((old) => ({ ...old, [provider]: true }));
              const version = (mediaPermissionVersions.current[provider] ?? 0) + 1;
              mediaPermissionVersions.current[provider] = version;
              const current = () =>
                mediaPermissionMounted.current &&
                mediaPermissionVersions.current[provider] === version;
              void media
                .connect(provider)
                .then((result) => {
                  if (current()) setMediaPermissions((old) => ({ ...old, [provider]: result }));
                })
                .catch((error) => {
                  if (current())
                    setMediaPermissions((old) => ({
                      ...old,
                      [provider]: {
                        status: "unavailable",
                        reason: error instanceof Error ? error.message : String(error),
                      },
                    }));
                })
                .finally(() => {
                  mediaConnectJobs.current.delete(provider);
                  if (mediaPermissionMounted.current)
                    setMediaConnectPending((old) => ({ ...old, [provider]: false }));
                });
            },
          }}
          diagnostics={diagnosticsAvailable}
          launcher={{
            status: launcherStatus,
            error: launcherError,
            appChoices: launcherAppChoices,
            itemActions: launcherItemActions,
            profileTransfer: launcherProfileActions,
            widgetCatalog,
            interactionCapabilities: launcherInteractionCapabilities
              ? {
                  preciseScroll: launcherInteractionCapabilities.gestureAvailable,
                  pinch: launcherInteractionCapabilities.pinchAvailable,
                  swipe: launcherInteractionCapabilities.swipeAvailable,
                  letterNavigation: launcherInteractionCapabilities.letterInputAvailable,
                  haptics: launcherInteractionCapabilities.hapticsAvailable,
                }
              : undefined,
            widgetPackages: {
              status: widgetPackageStatus,
              actions: {
                review: launcher.reviewPackage,
                install: (token) => mutateSettings(() => launcher.installPackage(token)),
                cancel: launcher.cancelPackageReview,
                remove: (digest) => mutateSettings(() => launcher.removePackage(digest)),
              },
              onRefresh: () => void refreshWidgetPackages(),
            },
            onUseNativeDock: () => {
              setLauncherError("");
              void launcher
                .useNativeDock()
                .then(async () => {
                  await reload();
                  setLauncherStatus(await launcher.status());
                })
                .catch(async (error: unknown) => {
                  setLauncherError(String(error));
                  try {
                    setLauncherStatus(await launcher.status());
                  } catch {}
                });
            },
          }}
        />
      </fieldset>
    </>
  );
}

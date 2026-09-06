import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAbout, useCrash, usePermissions } from "./hooks/useBridge";
import {
  hasBackend,
  importSettings,
  loadSettings,
  onPrefsTab,
  onSwitcherEvent,
  saveSettings,
  switcher,
} from "./lib/bridge";
import { demoStateFor } from "./lib/demo";
import type { VisualStyle } from "./lib/types";
import {
  defaultSettings,
  emptyState,
  type Settings as SettingsModel,
  type SwitcherState,
} from "./lib/types";
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
  const revision = useRef(0);
  const pendingImports = useRef(0);
  const [importing, setImporting] = useState(false);
  useEffect(() => {
    let active = true;
    const initialRevision = revision.current;
    loadSettings().then((s) => {
      if (active && s && s.behavior && revision.current === initialRevision) setSettings(s);
    });
    return () => {
      active = false;
    };
  }, []);
  const onChange = useCallback((next: SettingsModel) => {
    if (pendingImports.current > 0) return;
    const current = ++revision.current;
    setSettings(next);
    queue.current = queue.current.then(async () => {
      try {
        await saveSettings(next);
        if (revision.current === current) setSaveError(null);
      } catch (error) {
        setSaveError(`Could not save settings: ${String(error)}`);
      }
    });
  }, []);
  const onImport = useCallback((text: string): Promise<void> => {
    ++revision.current;
    ++pendingImports.current;
    setImporting(true);
    const imported = queue.current
      .then(async () => {
        const canonical = await importSettings(text);
        setSettings(canonical);
        setSaveError(null);
      })
      .finally(() => {
        --pendingImports.current;
        setImporting(pendingImports.current > 0);
      });
    // Publish every successful import in queue order, even when a later one
    // fails. Edits are disabled until the queue drains to avoid saving a
    // pre-import snapshot over newly imported settings.
    queue.current = imported.catch(() => {});
    return imported;
  }, []);
  return { settings, onChange, onImport, saveError, importing };
}

// App is the desktop frontend shell. Two Wails windows load it: the overlay
// window (default route) subscribes to the Go controller's events and renders
// the switcher; the preferences window (#/settings) renders the settings form.
// It holds no business logic.
export default function App() {
  if (isSettingsRoute()) return <SettingsRoute />;
  if (isDemoRoute()) {
    return (
      <div className="ot-demo-backdrop">
        <Overlay state={demoStateFor(demoStyle())} handlers={noopHandlers} />
      </div>
    );
  }
  return <OverlayRoute />;
}

function OverlayRoute() {
  const [state, setState] = useState<SwitcherState>(emptyState);
  const [actionError, setActionError] = useState<string | null>(null);
  const actionRevision = useRef(0);
  const [thumbs, setThumbs] = useState<Record<string, string>>({});
  const [previews, setPreviews] = useState<Record<string, string>>({});
  // The overlay window never becomes key (the app is not activated on show), so
  // in the real app keyboard input arrives as native-tap "switcher:key" events.
  // The DOM listener in Overlay is only a fallback for browser dev, enabled
  // until the backend probe answers.
  const [nativeKeys, setNativeKeys] = useState(true);
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
    return onSwitcherEvent({
      onShow: (s) => {
        ++actionRevision.current;
        setActionError(null);
        setThumbs({}); // new session: drop the previous capture's previews
        setPreviews({});
        setState(s);
      },
      onUpdate: setState,
      onHide: () => {
        ++actionRevision.current;
        setActionError(null);
        setState((s) => ({ ...s, open: false }));
      },
      onThumbnails: (t) => setThumbs((prev) => ({ ...prev, ...t })),
      onPreview: (p) => setPreviews((prev) => ({ ...prev, ...p })),
    });
  }, []);

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

  const handlers = useMemo<OverlayHandlers>(
    () => ({
      onAdvance: () => void switcher.advance(),
      onReverse: () => void switcher.reverse(),
      onConfirm: () => void switcher.confirm(),
      onConfirmWindow: (windowId) => void switcher.confirmWindow(windowId),
      onCancel: () => void switcher.cancel(),
      onSelect: (i) => void switcher.select(i),
      onSearchChange: (q) => void switcher.setSearch(q),
      onClose: (windowId) => windowAction("close", windowId),
      onMinimize: (windowId) => windowAction("minimize", windowId),
      onFullscreen: (windowId) => windowAction("fullscreen", windowId),
      onQuit: (appId) => void performAction("quit", 0, appId),
      onHide: (appId) => void performAction("hide", 0, appId),
      onAction: (kind, windowId, appId) => void performAction(kind, windowId, appId),
    }),
    [windowAction, performAction],
  );

  return (
    <>
      <Overlay state={stateWithThumbs} handlers={handlers} nativeKeys={nativeKeys} />
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

// SettingsRoute renders the preferences window's contents. The window is a
// regular titled window (its own Wails window since the Wails v3 migration);
// the menubar can deep-link a tab via the "prefs:tab" event.
function SettingsRoute() {
  const { settings, onChange, onImport, saveError, importing } = useSettingsModel();
  const perms = usePermissions();
  const about = useAbout();
  const crash = useCrash();
  const [requestedTab, setRequestedTab] = useState<string | null>(null);

  useEffect(() => onPrefsTab(setRequestedTab), []);

  return (
    <fieldset disabled={importing} aria-busy={importing} className="m-0 min-w-0 border-0 p-0">
      <Settings
        settings={settings}
        onChange={onChange}
        onImport={onImport}
        saveError={saveError}
        permissions={perms}
        about={about}
        crash={crash}
        requestedTab={requestedTab}
      />
    </fieldset>
  );
}

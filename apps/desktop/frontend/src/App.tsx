import { useCallback, useEffect, useMemo, useState } from "react";
import { useAbout, useCrash, usePermissions } from "./hooks/useBridge";
import { loadSettings, onSwitcherEvent, saveSettings, switcher } from "./lib/bridge";
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
  useEffect(() => {
    let active = true;
    loadSettings().then((s) => {
      if (active && s) setSettings(s);
    });
    return () => {
      active = false;
    };
  }, []);
  const onChange = useCallback((next: SettingsModel) => {
    setSettings(next);
    void saveSettings(next);
  }, []);
  return { settings, onChange };
}

// App is the desktop frontend shell. In the overlay window it subscribes to the
// Go controller's events and renders the switcher; in the preferences window
// (#settings) it renders the settings form. It holds no business logic.
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
  const [thumbs, setThumbs] = useState<Record<string, string>>({});
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const [prefsOpen, setPrefsOpen] = useState(false);
  const [prefsTab, setPrefsTab] = useState<string | null>(null);

  useEffect(() => {
    return onSwitcherEvent({
      onShow: (s) => {
        setThumbs({}); // new session: drop the previous capture's previews
        setPreviews({});
        setState(s);
      },
      onUpdate: setState,
      onHide: () => setState((s) => ({ ...s, open: false })),
      onThumbnails: (t) => setThumbs((prev) => ({ ...prev, ...t })),
      onPreview: (p) => setPreviews((prev) => ({ ...prev, ...p })),
      onPrefsOpen: () => setPrefsOpen(true),
      onPrefsClose: () => {
        setPrefsOpen(false);
        setPrefsTab(null); // next plain open starts on the default tab
      },
      onPrefsTab: setPrefsTab,
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

  const indexOfWindow = useCallback(
    (windowId: number) => state.entries.findIndex((e) => e.windowId === windowId),
    [state.entries],
  );
  const indexOfApp = useCallback(
    (appId: number) => state.entries.findIndex((e) => e.appId === appId),
    [state.entries],
  );

  const handlers = useMemo<OverlayHandlers>(
    () => ({
      onAdvance: () => void switcher.advance(),
      onReverse: () => void switcher.reverse(),
      onConfirm: () => void switcher.confirm(),
      onCancel: () => void switcher.cancel(),
      onSelect: (i) => void switcher.select(i),
      onSearchChange: (q) => void switcher.setSearch(q),
      onClose: (windowId) =>
        void switcher.select(indexOfWindow(windowId)).then(() => switcher.closeSelected()),
      onMinimize: (windowId) =>
        void switcher.select(indexOfWindow(windowId)).then(() => switcher.minimizeSelected()),
      onFullscreen: (windowId) =>
        void switcher.select(indexOfWindow(windowId)).then(() => switcher.fullscreenSelected()),
      onQuit: (appId) =>
        void switcher.select(indexOfApp(appId)).then(() => switcher.quitSelectedApp()),
      onHide: (appId) =>
        void switcher.select(indexOfApp(appId)).then(() => switcher.hideSelectedApp()),
    }),
    [indexOfWindow, indexOfApp],
  );

  return (
    <>
      <Overlay state={stateWithThumbs} handlers={handlers} />
      {prefsOpen ? <PreferencesPanel requestedTab={prefsTab} /> : null}
    </>
  );
}

// PreferencesPanel fills the window with the settings form. The Go side turns
// the shared window into a regular titled window while it is open (prefs:open/
// prefs:close events), so closing happens via the native close button.
function PreferencesPanel({ requestedTab }: { requestedTab?: string | null }) {
  const { settings, onChange } = useSettingsModel();
  const perms = usePermissions();
  const about = useAbout();
  const crash = useCrash();

  return (
    <div className="fixed inset-0 overflow-auto" role="dialog" aria-label="Preferences">
      <Settings
        settings={settings}
        onChange={onChange}
        permissions={perms}
        about={about}
        crash={crash}
        requestedTab={requestedTab}
      />
    </div>
  );
}

function SettingsRoute() {
  const { settings, onChange } = useSettingsModel();
  const perms = usePermissions();
  const about = useAbout();
  const crash = useCrash();

  return (
    <Settings
      settings={settings}
      onChange={onChange}
      permissions={perms}
      about={about}
      crash={crash}
    />
  );
}

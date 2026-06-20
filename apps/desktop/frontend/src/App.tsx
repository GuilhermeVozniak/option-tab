import { useCallback, useEffect, useMemo, useState } from "react";
import { loadSettings, onSwitcherEvent, saveSettings, switcher } from "./lib/bridge";
import {
  defaultSettings,
  emptyState,
  type Settings as SettingsModel,
  type SwitcherState,
} from "./lib/types";
import type { OverlayHandlers } from "./overlay/Overlay";
import { Overlay } from "./overlay/Overlay";
import { Settings } from "./settings/Settings";

function isSettingsRoute(): boolean {
  return typeof window !== "undefined" && window.location.hash.replace(/^#\/?/, "") === "settings";
}

// App is the desktop frontend shell. In the overlay window it subscribes to the
// Go controller's events and renders the switcher; in the preferences window
// (#settings) it renders the settings form. It holds no business logic.
export default function App() {
  if (isSettingsRoute()) return <SettingsRoute />;
  return <OverlayRoute />;
}

function OverlayRoute() {
  const [state, setState] = useState<SwitcherState>(emptyState);

  useEffect(() => {
    return onSwitcherEvent({
      onShow: setState,
      onUpdate: setState,
      onHide: () => setState((s) => ({ ...s, open: false })),
    });
  }, []);

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
      onQuit: (appId) =>
        void switcher.select(indexOfApp(appId)).then(() => switcher.quitSelectedApp()),
      onHide: (appId) =>
        void switcher.select(indexOfApp(appId)).then(() => switcher.hideSelectedApp()),
    }),
    [indexOfWindow, indexOfApp],
  );

  return <Overlay state={state} handlers={handlers} />;
}

function SettingsRoute() {
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

  return <Settings settings={settings} onChange={onChange} />;
}

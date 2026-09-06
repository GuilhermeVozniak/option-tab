import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AppSwitcher } from "./app-switcher/AppSwitcher";
import { type DockPanelHandlers, DockPanelView } from "./dock/DockPanelView";
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
import { type DockPointer, dock, onDockEvent, onDockInputStatus } from "./lib/dock-bridge";
import { makeT, resolveLang } from "./lib/i18n";
import type { DockViewState, VisualStyle } from "./lib/types";
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
function isDockRoute(): boolean {
  return route() === "dock";
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
  if (isDockRoute()) return <DockRoute />;
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
        <AppSwitcher state={stateWithThumbs} handlers={handlers} nativeKeys={nativeKeys} t={t} />
      ) : (
        <Overlay state={stateWithThumbs} handlers={handlers} nativeKeys={nativeKeys} />
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
  const currentSession = useRef(0);
  const activeSession = useRef(0);
  const retiredSession = useRef(0);
  const latestRevision = useRef(0);
  const actionRevision = useRef(0);
  const pointerSequence = useRef(0);
  const pendingPointer = useRef<DockPointer | null>(null);
  const acceptShow = useCallback((next: DockViewState) => {
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
    }
    latestRevision.current = revision;
    activeSession.current = next.session;
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
  }, []);
  const acceptUpdate = useCallback((next: DockViewState) => {
    const revision = next?.revision ?? 0;
    if (next?.session === activeSession.current && revision >= latestRevision.current) {
      latestRevision.current = revision;
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
  }, []);
  const retire = useCallback((session: number, revision: number) => {
    if (session < currentSession.current) return;
    if (session === currentSession.current && revision < latestRevision.current) return;
    currentSession.current = session;
    latestRevision.current = revision;
    retiredSession.current = Math.max(retiredSession.current, session);
    activeSession.current = 0;
    ++actionRevision.current;
    setState(null);
    setFrames({});
    setNativePointer(null);
    pointerSequence.current = 0;
    if ((pendingPointer.current?.session ?? 0) <= session) pendingPointer.current = null;
  }, []);
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
  const handlers = useMemo<DockPanelHandlers>(
    () => ({
      onSelectWindow: (session, id) => {
        if (session === activeSession.current) void dock.select(session, id);
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
    }),
    [run],
  );
  return visible ? (
    <DockPanelView state={visible} handlers={handlers} nativePointer={nativePointer} t={t} />
  ) : null;
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
  const [dockInputError, setDockInputError] = useState("");
  const dockInputRevision = useRef(0);

  useEffect(() => onPrefsTab(setRequestedTab), []);
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
        dockInputError={dockInputError}
      />
    </fieldset>
  );
}

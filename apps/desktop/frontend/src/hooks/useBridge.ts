// Bridge-backed hooks shared by the settings window and the preferences panel:
// live OS-permission state, About-tab info, and crash-report surfacing. Each
// degrades to "absent" without Wails so the UI simply omits the section.
import { useEffect, useMemo, useState } from "react";
import {
  crashReports,
  loadCrashReport,
  loadPermissions,
  loadVersion,
  onUpdateAvailable,
  onUpdateChecked,
  onUpdateProgress,
  permissions as permissionsApi,
  system,
  type UpdateCheck,
  type UpdateInfo,
  type UpdateProgress,
} from "../lib/bridge";
import type { Permissions } from "../lib/types";
import type { AboutControl, CrashControl, PermissionsControl } from "../settings/Settings";

// usePermissions loads the OS-permission state and, when Wails is present,
// polls it so a grant made in System Settings reflects live without a restart.
// Returns undefined without Wails so the Permissions UI is not rendered.
export function usePermissions(): PermissionsControl | undefined {
  const [state, setState] = useState<Permissions | null>(null);

  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setInterval> | undefined;
    loadPermissions().then((p) => {
      if (!active || !p) return; // no Wails: no permissions UI, no polling
      setState(p);
      timer = setInterval(() => {
        loadPermissions().then((next) => {
          if (active && next) setState(next);
        });
      }, 2000);
    });
    return () => {
      active = false;
      if (timer) clearInterval(timer);
    };
  }, []);

  if (!state) return undefined;
  return {
    state,
    onRequest: (kind) => void permissionsApi.request(kind),
    onOpenSettings: (kind) => void permissionsApi.openSettings(kind),
  };
}

// useAbout provides the About-tab control: the Go-reported version (falling
// back to "dev" without Wails) and link/update actions routed through Go so
// they open in the system browser.
export function useAbout(): AboutControl {
  const [version, setVersion] = useState("dev");
  const [update, setUpdate] = useState<UpdateInfo | undefined>(undefined);
  const [progress, setProgress] = useState<UpdateProgress | undefined>(undefined);
  const [checked, setChecked] = useState<UpdateCheck | undefined>(undefined);
  useEffect(() => {
    let active = true;
    loadVersion().then((v) => {
      if (active && v) setVersion(v);
    });
    const off = onUpdateAvailable((u) => {
      if (active) setUpdate(u);
    });
    const offProgress = onUpdateProgress((p) => {
      if (active) setProgress(p);
    });
    const offChecked = onUpdateChecked((c) => {
      if (active) setChecked(c);
    });
    return () => {
      active = false;
      off();
      offProgress();
      offChecked();
    };
  }, []);
  return useMemo(
    () => ({
      version,
      update,
      progress,
      checked,
      onOpenURL: (url) => void system.openURL(url),
      onCheckUpdates: () => void system.checkForUpdates(),
      onInstallUpdate: () => void system.installUpdate(),
    }),
    [version, update, progress, checked],
  );
}

// useCrash surfaces a crash log from the previous run (if any) with report/
// dismiss actions. Returns undefined when there is nothing to report or Wails
// is absent, so the banner simply doesn't render.
export function useCrash(): CrashControl | undefined {
  const [log, setLog] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    loadCrashReport().then((l) => {
      if (active && l) setLog(l);
    });
    return () => {
      active = false;
    };
  }, []);
  if (!log) return undefined;
  return {
    summary: log.split("\n", 1)[0] ?? "",
    onReport: () => void crashReports.report(),
    onDismiss: () => {
      setLog(null);
      void crashReports.dismiss();
    },
  };
}

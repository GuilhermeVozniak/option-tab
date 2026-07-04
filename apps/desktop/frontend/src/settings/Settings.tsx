import { useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { makeT, resolveLang } from "../lib/i18n";
import type { Settings as SettingsModel } from "../lib/types";
import { Onboarding } from "./Onboarding";
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
import { FilteringTab } from "./tabs/FilteringTab";
import { GeneralTab } from "./tabs/GeneralTab";

export type { AboutControl, CrashControl, PermissionsControl };
// Re-exported so consumers (App, hooks, tests) keep one import site.
export { PROJECT_URL };

interface SettingsProps {
  settings: SettingsModel;
  onChange: (next: SettingsModel) => void;
  permissions?: PermissionsControl;
  about?: AboutControl;
  crash?: CrashControl;
}

const TABS = ["General", "Controls", "Appearance", "Filtering", "Blacklists", "About"] as const;
type Tab = (typeof TABS)[number];

// Settings is a controlled preferences form. It never holds the settings itself:
// every edit produces a new Settings object passed to onChange, so persistence
// and live-apply are the parent's concern. Only the active tab is local UI state.
// All tab panels stay mounted (inactive ones hidden) so the whole form is a
// single controlled surface. On first run (behavior.onboarded false, with live
// permissions available) it renders the onboarding wizard instead.
export function Settings({ settings, onChange, permissions, about, crash }: SettingsProps) {
  const [tab, setTab] = useState<Tab>("General");
  const openURL = about?.onOpenURL ?? ((url: string) => window.open(url, "_blank", "noopener"));
  const checkUpdates = about?.onCheckUpdates ?? (() => openURL(`${PROJECT_URL}/releases`));
  const t = makeT(resolveLang(settings.behavior.language));

  const patch = (partial: Partial<SettingsModel>) => onChange({ ...settings, ...partial });
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
  };

  if (permissions && !settings.behavior.onboarded) {
    return (
      <div className="ot-settings px-6 py-6 text-foreground">
        <Onboarding
          permissions={permissions}
          t={t}
          onFinish={() => ctx.patchBehavior({ onboarded: true })}
        />
      </div>
    );
  }

  const update = about?.update;
  const updateBanner = update ? (
    <div className="my-2 flex flex-wrap items-center gap-3 rounded-xl border border-primary/40 bg-primary/15 px-3.5 py-2.5 text-[13px] shadow-[inset_0_1px_0_rgba(255,255,255,0.12)] backdrop-blur-md">
      <span>{t("Version {v} is available.").replace("{v}", update.version)}</span>
      <Button
        variant="default"
        size="sm"
        aria-label="Download update"
        onClick={() => openURL(update.url)}
      >
        {t("Download update…")}
      </Button>
    </div>
  ) : null;

  return (
    <div className="ot-settings px-6 py-6 text-foreground">
      <div className="mx-auto max-w-[760px]">
        <h1 className="m-0 mb-5 text-xl font-semibold tracking-tight">
          {t("Option Tab — Preferences")}
        </h1>

        <nav
          className="mb-6 flex w-fit flex-wrap gap-1 rounded-xl border border-white/12 bg-white/6 p-1 shadow-[inset_0_1px_0_rgba(255,255,255,0.1)] backdrop-blur-xl"
          role="tablist"
        >
          {TABS.map((name) => (
            <button
              key={name}
              type="button"
              role="tab"
              aria-selected={tab === name}
              className={cn(
                "cursor-pointer rounded-lg px-3.5 py-1.5 text-[13px] font-medium text-foreground/60 transition-colors hover:text-foreground",
                tab === name &&
                  "bg-white/15 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.2)]",
              )}
              onClick={() => setTab(name)}
            >
              {t(name)}
            </button>
          ))}
        </nav>

        <section hidden={tab !== "General"} aria-label="General" className="space-y-4">
          <GeneralTab
            ctx={ctx}
            permissions={permissions}
            crash={crash}
            updateBanner={updateBanner}
            checkUpdates={checkUpdates}
          />
        </section>
        <section hidden={tab !== "Controls"} aria-label="Controls" className="space-y-4">
          <ControlsTab ctx={ctx} />
        </section>
        <section hidden={tab !== "Appearance"} aria-label="Appearance" className="space-y-4">
          <AppearanceTab ctx={ctx} />
        </section>
        <section hidden={tab !== "Filtering"} aria-label="Filtering" className="space-y-4">
          <FilteringTab ctx={ctx} />
        </section>
        <section hidden={tab !== "Blacklists"} aria-label="Blacklists" className="space-y-4">
          <BlacklistsTab ctx={ctx} />
        </section>
        <section hidden={tab !== "About"} aria-label="About" className="space-y-4">
          <AboutTab
            ctx={ctx}
            about={about}
            updateBanner={updateBanner}
            openURL={openURL}
            checkUpdates={checkUpdates}
          />
        </section>
      </div>
    </div>
  );
}

import { useState, useSyncExternalStore } from "react";
import type { Translate } from "../lib/i18n";

export const SETTINGS_PAGES = {
  General: {
    label: "General",
    description: "Make Option Tab feel at home on your Mac.",
    icon: "M4 7h12M4 13h12M7 4v6M13 10v6",
  },
  Controls: {
    label: "Shortcuts",
    description: "Choose how to open the switcher and what happens while you use it.",
    icon: "M3 5h14v10H3zM6 8h.01M10 8h.01M14 8h.01M6 12h8",
  },
  Appearance: {
    label: "Appearance",
    description: "Adjust the look and layout of each switcher.",
    icon: "M3 4h14v12H3zM3 8h14M8 8v8",
  },
  Filtering: {
    label: "Window rules",
    description: "Choose which windows appear and how they are ordered.",
    icon: "M3 5h14M6 10h8M8 15h4",
  },
  Blacklists: {
    label: "Excluded apps",
    description: "Keep selected apps out of your switcher.",
    icon: "M15.7 4.3 4.3 15.7M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  },
  Dock: {
    label: "Dock",
    description: "Set up your launcher, window previews, and desktop controls.",
    icon: "M3 12v4h14v-4M5 7h2v5H5zM9 5h2v7H9zM13 7h2v5h-2z",
  },
  About: {
    label: "About",
    description: "App information, support, and diagnostics.",
    icon: "M10 9v5M10 6h.01M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  },
} as const;

export type SettingsPage = keyof typeof SETTINGS_PAGES;
export const SETTINGS_TABS = Object.keys(SETTINGS_PAGES) as SettingsPage[];
type SettingsTheme = "system" | "light" | "dark";
const THEME_KEY = "option-tab.settings-theme";

export function useSettingsTheme() {
  const [theme, setTheme] = useState<SettingsTheme>(() => {
    try {
      const saved = localStorage.getItem(THEME_KEY);
      return saved === "light" || saved === "dark" ? saved : "system";
    } catch {
      return "system";
    }
  });
  const changeTheme = (value: SettingsTheme) => {
    setTheme(value);
    // A local presentation preference, deliberately independent of exported
    // switcher/Dock settings. Storage can be unavailable in restricted webviews.
    try {
      localStorage.setItem(THEME_KEY, value);
    } catch {
      /* Keep this session's choice. */
    }
  };
  return [theme, changeTheme] as const;
}

const compactQuery = "(max-width: 560px)";
function subscribeToNavigationLayout(onChange: () => void) {
  const query = window.matchMedia?.(compactQuery);
  query?.addEventListener("change", onChange);
  return () => query?.removeEventListener("change", onChange);
}
function isCompactNavigation() {
  return window.matchMedia?.(compactQuery).matches ?? false;
}

export function SettingsSidebar({
  tab,
  onSelect,
  theme,
  onTheme,
  t,
  id,
}: {
  tab: SettingsPage;
  onSelect: (tab: SettingsPage) => void;
  theme: SettingsTheme;
  onTheme: (value: SettingsTheme) => void;
  t: Translate;
  id: string;
}) {
  const compact = useSyncExternalStore(
    subscribeToNavigationLayout,
    isCompactNavigation,
    () => false,
  );
  return (
    <aside className="ot-settings-sidebar">
      <div className="ot-settings-brand">
        <span className="ot-settings-brand-mark" aria-hidden="true">
          ⌥
        </span>
        <div>
          <strong>Option Tab</strong>
          <span>{t("Settings")}</span>
        </div>
      </div>
      <nav
        role="tablist"
        aria-label={t("Settings sections")}
        aria-orientation={compact ? "horizontal" : "vertical"}
        className="ot-settings-nav"
      >
        {SETTINGS_TABS.map((name, index) => (
          <button
            key={name}
            id={`${id}-tab-${name}`}
            type="button"
            role="tab"
            aria-selected={tab === name}
            aria-controls={`${id}-panel-${name}`}
            tabIndex={tab === name ? 0 : -1}
            onClick={() => onSelect(name)}
            onKeyDown={(event) => {
              let next = index;
              if (event.key === "ArrowDown" || event.key === "ArrowRight")
                next = (index + 1) % SETTINGS_TABS.length;
              else if (event.key === "ArrowUp" || event.key === "ArrowLeft")
                next = (index - 1 + SETTINGS_TABS.length) % SETTINGS_TABS.length;
              else if (event.key === "Home") next = 0;
              else if (event.key === "End") next = SETTINGS_TABS.length - 1;
              else return;
              event.preventDefault();
              onSelect(SETTINGS_TABS[next]);
              document.getElementById(`${id}-tab-${SETTINGS_TABS[next]}`)?.focus();
            }}
          >
            <svg
              aria-hidden="true"
              viewBox="0 0 20 20"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d={SETTINGS_PAGES[name].icon} />
            </svg>
            {t(SETTINGS_PAGES[name].label)}
          </button>
        ))}
      </nav>
      <div className="ot-settings-sidebar-footer">
        <span className="ot-settings-theme-label">{t("Settings theme")}</span>
        <div className="ot-settings-theme-picker" role="group" aria-label={t("Settings theme")}>
          {(["system", "light", "dark"] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-label={`${t("Settings theme")} ${t(value === "system" ? "System" : value === "light" ? "Light" : "Dark").toLocaleLowerCase()}`}
              aria-pressed={theme === value}
              onClick={() => onTheme(value)}
            >
              {t(value === "system" ? "System" : value === "light" ? "Light" : "Dark")}
            </button>
          ))}
        </div>
        <p>{t("Only changes this settings window.")}</p>
      </div>
    </aside>
  );
}

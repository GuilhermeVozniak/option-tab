"use client";

import { useEffect, useState } from "react";

const storageKey = "option-tab-site-theme";
type Theme = "system" | "light" | "dark";

export function ThemeControl() {
  const [theme, setTheme] = useState<Theme>("system");

  useEffect(() => {
    try {
      const saved = localStorage.getItem(storageKey);
      if (saved === "light" || saved === "dark") setTheme(saved);
    } catch {
      // System appearance remains usable when browser storage is unavailable.
    }
  }, []);

  function changeTheme(value: Theme) {
    setTheme(value);
    document.documentElement.dataset.theme = value;
    try {
      localStorage.setItem(storageKey, value);
    } catch {
      // The selected appearance still applies for the current page.
    }
  }

  return (
    <label className="theme-control">
      <span className="sr-only">Website theme</span>
      <select value={theme} onChange={(event) => changeTheme(event.target.value as Theme)}>
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
  );
}

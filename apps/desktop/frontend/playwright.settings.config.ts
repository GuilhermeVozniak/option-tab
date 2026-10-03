import { defineConfig, devices } from "@playwright/test";
import base from "./playwright.config";

// WebKit checks the engine used by the native macOS Settings window.
export default defineConfig({
  ...base,
  testMatch: ["settings.spec.ts", "settings-design.spec.ts"],
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
  ],
});

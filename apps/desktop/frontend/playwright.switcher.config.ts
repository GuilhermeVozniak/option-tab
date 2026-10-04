import { defineConfig } from "@playwright/test";
import rendererConfig from "./playwright.widgets.config";

// Run switcher geometry and preview controls in Chromium and WebKit without
// launching the native app. Native gestures have their own classification fixture.
export default defineConfig(rendererConfig, {
  testMatch: ["switcher-gestures.spec.ts", "preview-controls.spec.ts"],
});

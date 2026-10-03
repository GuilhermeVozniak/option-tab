import { defineConfig } from "@playwright/test";
import rendererConfig from "./playwright.widgets.config";

// Run the switcher geometry contract in Chromium and WebKit without launching
// the native app. Native gesture classification is verified by its own fixture.
export default defineConfig(rendererConfig, {
  testMatch: "switcher-gestures.spec.ts",
});

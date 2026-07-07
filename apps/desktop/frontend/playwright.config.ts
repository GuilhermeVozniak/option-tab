import { defineConfig, devices } from "@playwright/test";

// The desktop UI is a Vite React app. For e2e we build it and serve the static
// bundle, then drive the real overlay and preferences in Chromium. The Go/Wails
// backend is faked per-test (e2e/support/fakeWails.ts) so this exercises the UI
// end-to-end without the native app. E2E_PORT lets parallel runs pick a port.
const PORT = Number(process.env.E2E_PORT ?? 4174);

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "line" : "list",
  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: `bunx vite build && bunx vite preview --port ${PORT} --strictPort`,
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});

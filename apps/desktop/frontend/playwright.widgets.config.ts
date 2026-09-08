import { defineConfig, devices } from "@playwright/test";

// Exercise the native renderer's engine without building or launching the app.
// Install with `bunx playwright install chromium webkit`, then use this config explicitly.
const port = Number(process.env.E2E_PORT ?? 4187);

export default defineConfig({
  testDir: "./e2e",
  testMatch: "launcher-widget-layout.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "line" : "list",
  use: { baseURL: `http://localhost:${port}`, trace: "on-first-retry" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
  ],
  webServer: {
    command: `bunx vite --port ${port} --strictPort`,
    url: `http://localhost:${port}`,
    timeout: 30_000,
  },
});

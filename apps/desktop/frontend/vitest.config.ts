import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      // Never load the real @wailsio/runtime in tests: it starts a
      // window.setInterval drag poll at import time that can fire after jsdom
      // teardown and fail the run with an unhandled "window is not defined".
      "@wailsio/runtime": fileURLToPath(new URL("./src/test/wailsRuntimeStub.ts", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
  },
});

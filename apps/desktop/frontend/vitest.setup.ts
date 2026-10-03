import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// jsdom has no layout observer. Radix uses it to size hidden form controls;
// browser suites exercise real measurement and pointer geometry.
if (!globalThis.ResizeObserver) {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
}

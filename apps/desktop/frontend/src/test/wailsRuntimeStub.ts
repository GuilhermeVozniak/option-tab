// Test stub for @wailsio/runtime, wired via resolve.alias in vitest.config.ts.
//
// The real module must never load under vitest: drag.js starts a
// window.setInterval poll at import time, and when a tick fires after the
// jsdom environment is torn down it throws "ReferenceError: window is not
// defined" as an unhandled error — failing the run even when every test
// passed (a timing race, so it is flaky).
//
// Tests have no Wails backend, so Events are no-ops and Call rejects exactly
// like a dead transport would; bridge.ts already degrades both to its
// browser/no-backend fallbacks. Test files that need to fire Go-side events
// vi.mock("@wailsio/runtime") on top of this stub and override Events.

type EventCallback = (ev: { data: unknown }) => void;
type Unsubscribe = () => void;

export const Events = {
  On:
    (_name: string, _cb: EventCallback): Unsubscribe =>
    () => {},
  Once:
    (_name: string, _cb: EventCallback): Unsubscribe =>
    () => {},
  Off: (_name: string, _cb?: EventCallback): void => {},
  OffAll: (): void => {},
  Emit: (_name: string, _data?: unknown): void => {},
};

const noBackend = (): Promise<never> =>
  Promise.reject(new Error("@wailsio/runtime test stub: no backend"));

export const Call = {
  ByID: noBackend,
  ByName: noBackend,
};

// Referenced only in the generated bindings' JSDoc types.
export const CancellablePromise = Promise;

// Generated model constructors build these converters at module scope.
export const Create = {
  Any: <T>(source: T): T => source,
  Nullable:
    <T>(createFrom: (source: unknown) => T) =>
    (source: unknown): T | null =>
      source == null ? null : createFrom(source),
  Array:
    <T>(createFrom: (source: unknown) => T) =>
    (source: unknown[]) =>
      source.map(createFrom),
  Map:
    <T>(keyFrom: (source: string) => string, valueFrom: (source: unknown) => T) =>
    (source: Record<string, unknown>) =>
      Object.fromEntries(
        Object.entries(source).map(([key, value]) => [keyFrom(key), valueFrom(value)]),
      ),
};

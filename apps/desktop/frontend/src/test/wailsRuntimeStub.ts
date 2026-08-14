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

// Generated models call $Create.Array(createFrom) at module scope; the result
// is only used to deserialize Call responses, which never arrive in tests.
export const Create = {
  Array: <T>(createFrom: T): T => createFrom,
};

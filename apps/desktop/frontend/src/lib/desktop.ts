// Typed accessor for the Wails-exposed Go bindings. Wails injects the bound
// App methods on the global `go.main.App` object at runtime. We read them
// here (instead of importing the generated `wailsjs/` files) so the frontend
// builds standalone in CI and the binding is trivially mockable in tests.
interface WailsApp {
  Greet(name: string): Promise<string>;
}

function wailsApp(): WailsApp {
  const app = (globalThis as { go?: { main?: { App?: WailsApp } } }).go?.main?.App;
  if (!app) {
    throw new Error("Wails runtime bindings unavailable (run the app via `wails dev`).");
  }
  return app;
}

export function greet(name: string): Promise<string> {
  try {
    return wailsApp().Greet(name);
  } catch (e) {
    return Promise.reject(e);
  }
}

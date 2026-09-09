# Isolated App/Dock native acceptance smoke

Run from the repository root on macOS with existing Accessibility and Screen Recording grants, Go, Bun, Xcode command tools, and `/opt/homebrew/bin/cliclick`:

```sh
SMOKE_RETAIN_CLOSED=0 python3 apps/desktop/testdata/app_dock_smoke/run.py
```

For the deliberately retained-NSWindow regression, run `SMOKE_RETAIN_CLOSED=1 python3 apps/desktop/testdata/app_dock_smoke/run.py` (retention is also the default when unset). This preserves the closed object and reports whether a stale CG surface remains in the production inventory; a failure here is kept distinct from the owner-release run.

The runner builds the current frontend, compiles two disposable bundled AppKit fixtures, and builds the real desktop Go package using `go build -overlay` to replace only `main.go` with the retained `main.go.in`. It creates an isolated accessory Wails application and unique SingleInstance identity, with a temporary settings path. All build products/logs stay in the printed temporary evidence directory. It terminates only its own fixture processes and restores the original pointer on completion, timeout, or failure. It does not change Dock preferences, restart Dock, alter settings for the user's installed app, or send actions to user applications.

`smokePlatform` delegates to the real native platform while allowing only the two fixture PIDs and the two window IDs positively announced by the document fixture. The separate windowless fixture is admitted through the real native application inventory. Native runs also expose 66×20 AXWindow/AXDialog auxiliary panels. Their origin is unconfirmed; they persist with both NSTextView and the retained animated NSTextField fixture, so changing the text control does not eliminate them. Additional app blacklists corroborate the inventory guard. Native actions, window capture, and activation reject targets outside those identities. Restricting exact document IDs is deliberate: AppKit creates auxiliary CG surfaces (including transient surfaces titled `Window`) which are not the two test documents.

The harness uses real App/controller code, generated App bindings, the actual built React views, a dedicated hidden Wails host, and the real `DockPanelHost`. A separate `SmokeProbe.Report` service reads DOM evidence through a direct call to the locally inspected Wails HTTP runtime endpoint without replacing application handlers. It deliberately does not import another `/wails/runtime.js`: that module overwrites the bundled runtime event dispatcher and invalidates real event-delivery tests. Its steps distinguish:

- Go controller activation + actual grouped React DOM: this proves app-mode grouping/rendering, **not** native key delivery.
- Actual native AX Dock-icon hover through the real observer/controller/App pipeline.
- Hidden host / visible NSPanel / non-key / non-main native state.
- Actual DOM second-preview click through the generated App bridge while another window is selected, verifying exact native focus.
- Real native pointer packet exposes `article.is-hovered` controls and selects the exact second window; native coordinate click on its close control while a different disposable app is foreground. CSS hover stays empty in the non-key panel.
- Last document close with the app retained, followed by explicit windowless activation.
- Peak live-stream invocation count and eventual idle after capture shutdown.

No real event-tap hotkey delivery, system lock, Spaces switch, Dock restart, or permission revocation is claimed by this harness. Evidence files record reached steps and failures; `passed` exists only when all asserted steps complete.

## Native screen regression probe

```sh
go run ./apps/desktop/testdata/app_dock_smoke/screens
```

This read-only probe prints the actual native screen JSON, parses the `main` field as a Go boolean, and requires usable native screen IDs/bounds. It detected the production screen bug where `@(did == CGMainDisplayID())` serialized C integers `1/0`, causing `Screens()` to fall back to screen ID 0. The fix serializes `@YES/@NO` explicitly.

Historical native behavior: even `SMOKE_RETAIN_CLOSED=0` logs owner-array removal without SmokeWindow.dealloc, and macOS retains the closed CG surface. The positive observed-window retirement fix now passes the unchanged stale-inventory assertion with SMOKE_RETAIN_CLOSED=1. It uses exact AX destruction evidence rather than relying on object deallocation. This is scoped to observed roots; owner-release mode is not proof of actual NSWindow destruction.

The close stage registers an independent exact-target AXUIElementDestroyed observer before the native click. It verifies PID/window ID while the AX object is valid, then logs captured identity only from its callback. The callback never queries the destroyed AX element. Registration failure is reported explicitly; an observed notification does not bypass the separate stale-CG inventory assertion.

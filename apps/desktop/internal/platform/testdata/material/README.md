# Native material fixture

Run from `apps/desktop`:

```sh
go test -race ./internal/platform -run 'TestMaterial|TestLauncherNativeAdmission' -count=1
```

The fixture includes the production native implementation and uses real NSView/NSVisualEffectView objects with an inert NSObject host. It creates no NSWindow or application, installs no input hook, and performs no desktop actions. It verifies interior coordinate conversion, clipping, bottom-layer insertion, pass-through hit testing, solid removal, cancellation after preparation, stale fade completion, exact host replacement and close cleanup, and preview role exclusions. Existing launcher admission tests cover the shared visual recipe extraction.

Visible material appearance, Wails rendering transparency, real window recreation, and animation appearance require separate coordinated acceptance. This fixture does not claim those results.

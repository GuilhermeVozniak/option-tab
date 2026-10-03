# Disposable native Dock observer smoke

On macOS, coordinate the desktop input slot with other native tests, then run:

```sh
python3 apps/desktop/internal/platform/testdata/dock_observer_smoke.py --log /tmp/dock-observer-smoke.log
```

The runner builds a unique temporary Cocoa app, native icon probe, and opt-in platform test binary. It finds only the fixture's exact AX application URL, reveals the current Dock through ordinary pointer movement if auto-hidden, waits/requeries animated icon bounds, and hovers that icon. It restores the original cursor and terminates/reaps only its fixture afterward. It never writes Dock preferences, pins icons, restarts Dock, injects keys, or clicks other applications.

The production observer does not generate or intercept input. Pointer movement in the test helper is solely QA instrumentation. Ordinary platform test runs skip the native test unless the exact fixture PID/path are supplied by the runner.

The opt-in test starts/cancels three real observation sessions on the same disposable icon, checks exact PID/bundle/path and top-left global-point geometry, and confirms replacement generations and no delivery after cancellation returns. Diagnostics are enabled only in the test subprocess through `OPTION_TAB_DOCK_OBSERVER_DIAGNOSTICS=1`; they do not log arbitrary application identities.

This baseline does not change orientation or auto-hide preferences or restart Dock. Left/right orientation, monitor migration, display disconnect, Space changes, permission revocation/regrant, wake, and Dock restart require separately coordinated QA. Geometry unit tests do not establish those native transitions or Retina behavior on hardware that lacks a Retina display.

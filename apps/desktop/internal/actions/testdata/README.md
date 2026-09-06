# Native action smoke

On macOS with Go, Xcode command-line tools, and Accessibility access for the launching terminal:

```sh
python3 apps/desktop/internal/actions/testdata/native_smoke.py
```

Run from any directory using the script's absolute path if needed. `--log /path/to/output.log` preserves the result. Ordinary `go test` runs skip the opt-in test.

The runner builds a disposable Cocoa application and Go test binary in a temporary directory, starts exactly that app, passes its PID/state file only to the test subprocess, reaps it, and removes generated files even after failure. The test validates its native bundle identity before any actions. Never set the opt-in fixture variables to a user application's PID.

The fixture exercises AX New Window, focus and actual key-window state, bulk minimize, single-window minimize toggle, bulk close, a delegate-vetoed close, and exact-PID Force Quit. The fixture's independent JSON state establishes actual window changes. Bulk actions must complete all three known fixture windows without per-window failures. Native replicas that cannot be positively classified must produce one explicit `windowId: 0` incomplete-enumeration failure instead of being silently omitted or attempted as eight bogus window operations. This is an honest partial-result boundary, not proof that every raw CG surface can be classified.

Close-veto success means AX accepted the request; it does not mean an application was forced past a save confirmation. This fixture uses a deterministic delegate veto, not a document save dialog. It does not test cross-Space focus or permission revocation.

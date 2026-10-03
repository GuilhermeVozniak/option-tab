# Apple event transport fixtures

`main.m` includes the production transport with only a substituted suspension manager and synthetic read-only sender descriptors. It has no NSApplication/windows, event tap, target app or permission prompt. It tests typed decoding, remote refusal, existing-handler preservation, exactly-once replies, native timer expiry without a Go consumer, no-reply retirement, 16-pending overflow, cancellation-before-install and shutdown. Its metadata substitution is not evidence of real macOS sender authentication or TCC compatibility.

Run the default seam and Go lifecycle regressions from the worktree root:

```sh
go test -race ./apps/desktop/internal/platform -run '^TestAutomation(NativeRetirement|Stop|NativeTransport|DispatchDelay|Reply|ExpiredRequest|CancelledParent)' -count=1
```

`delivery.m` exercises actual own-process AESendMessage delivery with public NSAppleEventManager. On this macOS version, the kernel reports `kAEDirectCall`, which production deliberately refuses, even with own PID/EUID/audit metadata. The fixture reports REFUSED, not accepted delivery. It never targets another app or asks for permission.

```sh
clang -fobjc-arc -fblocks -framework Cocoa -framework Carbon apps/desktop/internal/platform/testdata/automation-transport/delivery.m -o /tmp/option-tab-automation-self-delivery
/tmp/option-tab-automation-self-delivery
```

`run_delivery.py` is an explicit optional OS test. It builds and ad-hoc signs a unique LSUIElement bundle, copies the actual dictionary, and launches a windowless, activation-prohibited receiver through LaunchServices using `open -g -j -n`. It uses a private temporary PID file and verifies exact executable/bundle paths before sending or terminating. A compiled sender performs no-prompt preflight against ONLY that fixture. Nonzero preflight means UNAVAILABLE and no send. It never accepts consent dialogs, executes AppleScript, changes Dock settings, moves the cursor or acts on user windows. The receiver has an eight-second lifetime bound; cleanup targets only the announced verified fixture PID.

This test launches a disposable app, so coordinate the native fixture slot before running:

```sh
python3 apps/desktop/internal/platform/testdata/automation-transport/run_delivery.py
```

Observed 2026-09-07: direct executable receivers returned -600; a proper LaunchServices receiver with AppKit event dispatch reached -1744 (`errAEEventWouldRequireUserConsent`). No consent was requested and no event was sent. Therefore cross-process successful suspended reply delivery and actual osascript dictionary execution remain acceptance gaps. Do not interpret script exit zero plus UNAVAILABLE as delivery PASS.

The default focused test also compiles `lease.m`: a windowless activation-prohibited process inspects real public AEGetEventHandler metadata and proves a later lower-level replacement survives drain. It sends no events. NSAppleEventManager target-to-target replacements return identical generic UPP/refcon on the tested SDK/runtime, so they cannot be distinguished by this public lease. The app must remain the sole NSAppleEventManager owner of private OpTb pairs. The fake manager's changed-refcon case represents an observable lower-level lease change, not Foundation target-to-target identity.

The packaged runner also checks `sdef <bundle>` discovers exactly seven private commands and logs native registration status=1 before preflight. Thus installed scripting keys/dictionary are covered even when TCC prevents the subsequent event send.

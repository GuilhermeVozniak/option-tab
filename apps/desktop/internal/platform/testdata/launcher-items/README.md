# Launcher reference fixture

Run from apps/desktop:

```sh
go test -race ./internal/platform -run 'TestLauncherItem|TestLauncherReference|TestLauncherIconPrivate' -count=1
```

The compiled Objective-C fixture includes production native code. NSOpenPanel factory aborts if called; the queued main block is canceled before presentation. NSWorkspace open operations, process inventory/start reads, graceful termination and relaunch timing are injected. The fixture creates only its own temporary text file and minimal app-bundle metadata, then removes that directory. No real application/device inventory, GUI, activation, input or user-file access is exercised.

Coverage includes current temporary resource identity, Foundation bookmark canonicalization, fresh stale-bookmark replacement refusal, exact app URL/bundle matching, ambiguous instances/PID reuse, queued cancellation, cancellation during preparation, graceful relaunch, termination refusal/timeout, and source/icon lifecycle tests. Native visible chooser, actual app opening/activation/relaunch, system restart bookmark behavior, Gatekeeper and physical launcher parent acceptance remain separate coordinated tests.

# Owned native bulk acceptance (A07 / A08)

This opt-in test launches only two disposable accessory fixtures at a time. It
uses the production `actions.Service`, native root enumeration and AX actions.
It accepts no external PID. No product app, user document, AppleScript, input
injection, activation helper or save-dialog button is used.

From `apps/desktop`, ordinary checks compile the Go driver and leave UI skipped:

```sh
GOWORK=off GOFLAGS=-mod=readonly go test ./internal/actions \
  -run '^(TestNativeBulkFixtureAdmission|TestDisposableNativeBulkAcceptance)$' -count=1 -v
```

An explicitly authorized native run needs existing Accessibility permission and
an absolute, existing evidence directory:

```sh
GOWORK=off GOFLAGS=-mod=readonly \
OPTION_TAB_NATIVE_BULK_ACCEPTANCE=1 \
OPTION_TAB_NATIVE_BULK_EVIDENCE=/absolute/private/evidence \
go test ./internal/actions -run '^TestDisposableNativeBulkAcceptance$' \
  -count=1 -v -timeout=60s
```

Set `OPTION_TAB_NATIVE_BULK_DIAGNOSTICS=1` alongside the explicit opt-in to
collect exactly three read-only samples of each owned window's process identity,
AX-current check and production `ActionWindows` snapshot. This mode uses only
the minimize-scenario fixtures, never invokes `actions.Service`, and does not
claim bulk-action acceptance. Identity diagnostics omit the private nonce.

The driver performs a no-prompt permission preflight, builds a private fixture
bundle, launches its own child handles, and binds their process start times,
private nonce, executable, bundle, role and window IDs before native mutation.
Native dispatch allows only `close` or `setMinimized` to exact target roots.
The second process is an untouched bystander; its window state is checked
before every mutation and throughout outcome observation. Read-only AX
readiness can wait; native mutations are never retried.

Both fixtures observe foreground changes on their AppKit main thread. Any
activation or foreground change writes a refusal receipt and exits. The driver
also checks foreground and fresh heartbeats. Each fixture exits if its parent
disappears or its 30-second lifetime expires, independently of Go cleanup. Normal
cleanup signals and joins only the handles started by this test. Cleanup is not
included as successful application closure evidence.

A08 first proves three real windows are minimized, including one already
minimized window, then repeats the bulk setter to prove it does not restore
anything. Each outcome requires fresh target and bystander sample sequences
and at least 500ms of stable observation. `minimize-first.json` and
`minimize-repeat.json` retain those independent results before A07 is attempted.

A07 uses an actual edited `NSDocument` attached through `NSWindowController`.
Its close path calls AppKit's normal `canCloseDocument…` implementation; the
fixture does not construct or simulate a save alert. Autosaving drafts and
in-place autosave are disabled; `autosavingFileType` is nil, and all document
write entry points count and refuse writes. No document URL or data is supplied.
The expected outcome is a closed clean sibling, an edited document still open
with a visible attached AppKit sheet and its buttons, no writes, and an unchanged
bystander. Accepted AX requests are recorded separately from native outcomes.
No confirmation button is pressed. If AppKit activates the app to show the
sheet, the probe fails closed and records the limitation; this is not a product
failure or evidence of save-dialog preservation.

The private `run-*` directory keeps preflight, action results, observed states,
fixture logs and compiler output, including unsuccessful phases. A denied
preflight launches nothing. Any later admission failure stops the run and
cleans up owned children; it does not retry using activation or input.

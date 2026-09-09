# Widget package chooser fixture

Run from `apps/desktop`:

```sh
go test -race ./internal/platform -run TestWidgetPackage -count=1
```

The Objective-C fixture includes the production translation unit and substitutes
panel construction, queue delivery, context admission and security-scope hooks.
It creates only disposable files in the Go test directory. It never creates an
NSApplication, real NSOpenPanel, user window, input event or persistent grant.

It checks queued context cancellation before panel construction, active panel
cancellation, successful approval snapshot/cleanup, local regular-file identity,
symlink/executable/directory/URL/oversize refusal, replacement during reading and
cancel-before-final-result. The bytes intentionally need not form a valid ZIP:
this port only selects immutable bytes; widgets.Preview owns archive validation.

Go tests inject the complete native operation to verify busy-until-joined
cancellation, late-result refusal, pre-cancel/nil admission and copied bytes.
Actual visible chooser interaction and sandbox-issued consent remain unverified.

# Diagnostic export fixture

Run from `apps/desktop` on macOS:

```sh
go test -race ./internal/diagnostics ./internal/platform -run TestDiagnostic -count=1
```

The platform test compiles `main.m`, which includes the production native implementation in its own translation unit. It uses only Go `t.TempDir()` destinations. The chooser factory is replaced with an aborting function; cancellation is delivered before the queued native presentation, so any attempt to display a chooser fails the fixture. It never moves the cursor, opens user files, posts input or accesses players.

The production writer is exercised against actual disposable files: exact bytes, existing-file and symlink refusal, a file created immediately before commit, precommit cancellation, successful commit followed by cancellation, private-temp cleanup and result publication only after cleanup. Exclusive `RENAME_EXCL` means this API does not overwrite existing files. Native result `destinationExists` requires choosing another filename.

This proves the file writer and pre-presentation cancellation seam. It does **not** prove an actual visible NSSavePanel grant/cancel, selected security scope in a distributed/sandboxed build, user-chosen remote-volume behavior, or final App/UI integration. OS filesystem latency is not a hard cancellation deadline; the caller retains the export slot until cleanup completes. Actual chooser acceptance needs separate explicit coordination.

Independent review regressions additionally use `context/`, a small Go/C driver linked to the actual production context callback. It holds the native main block, cancels a standard Go context, and executes the queued block before any native polling/cancel relay; a nil test panel factory records unexpected construction without creating UI. It also cancels that context at the commit seam. The writer fixture injects symlink/new-inode substitutions into the private0700 stage and verifies refusal while preserving those foreign resources. Only the fixture deletes its own injections. A substituted nonempty stage can remain on refusal; production cleanup never recursively deletes unknown content.

# Folder adapter native verification

Run from the repository/worktree root:

```sh
go test -race ./apps/desktop/internal/folder ./apps/desktop/internal/platform -run 'Folder|Bookmarks' -count=1 -v
```

These tests create and clean only their own temporary directories. They do not show NSOpenPanel, request TCC permissions, launch Finder, or open any file. The native open test uses the shared native preparation/final-guard helper with workspace dispatch disabled. It intentionally returns a sentinel error after actual parent/child stat preparation. This proves final-guard error propagation and identity refusal, not the production AppKit main-queue dispatch. Plain Go tests have no AppKit event loop; a follow-up run exposed that limitation and the test seam is explicit.

Actual evidence covers shallow directory enumeration, hidden files/subfolders/symlinks, all sort fields, folders-first, 500-entry partial results, opaque listing-ID retirement, parent/child replacement refusal, cancelled requests, and native security-scoped bookmark creation/reuse without a chooser. A fresh source reads the stored bookmark; a moved/replaced parent is refused. Corrupt bookmark data is removed with a revoked result. Store tests cover atomic private 0600 persistence, copied byte ownership and corrupt-store refusal. Platform portable stubs also compile for Linux.

Remaining acceptance: an actual explicit NSOpenPanel grant/cancellation and persisted permission reuse under the distributed application's sandbox/signing identity; TCC denial/revocation; exact disposable-item NSWorkspace dispatch and its user-default-app behavior. None is established by injected pre-cancellation or a guard-refused open.

Enumeration uses a 500 ms context/deadline and checks between filesystem operations, with a 500-entry cap. Individual filesystem calls and native URL/bookmark resolution are not forcibly interruptible; a stalled network filesystem can exceed the cooperative deadline. Enumeration is nonrecursive and never reads file contents. Partial sorting describes the enumerated subset, not the global first 500 items of an arbitrarily large directory.

The production `NewFolderSource(bookmarkPath)` constructor allows each application/test instance to use its own Application Support bookmark store, separate from settings. The optional darwinPlatform methods use the normal per-user option-tab store. Browser commands must supply only current opaque IDs through the App's scoped APIs; `FolderRef` is constructed from the captured canonical Dock URL by trusted Go code.

## AppKit runloop fixture and actual Dock AX probe

From `apps/desktop`:

```sh
OPTION_TAB_FOLDER_NATIVE_FIXTURE=1 go test -race ./internal/platform -run '^TestFolderNativeRunloopFixture$' -count=1 -v
OPTION_TAB_FOLDER_DOCK_READONLY=1 go test ./internal/platform -run '^TestFolderNativeReadOnlyDockClassification$' -count=1 -v
```

The first compiles `native_fixture.m` with production `darwin_folder.m` into a disposable executable. Its prohibited-activation AppKit host runs a real main runloop. Test-only method overrides replace NSOpenPanel with injected cancel/pre-cancel/wrong-folder/grant responses and intercept NSWorkspace `openURL:` without opening an application. This verifies production callback scheduling, real native bookmark creation/reuse, exact disposable-file/subfolder main-queue dispatch, final-guard cancellation, and parent/child identity refusal including a deterministic replacement during the external guard. It compares foreground before/after. The Go wrapper creates and removes only its own temporary directory and binary. It does not display an actual permission chooser or establish sandbox consent, TCC revocation, or default-app behavior.

The second is a bounded read-only AX traversal of the actual native Dock. It verifies same-PID AXDockItem/AXFolderDockItem metadata and canonical local file URL; no pointer movement, Dock preference changes, filesystem enumeration or user-file opening occurs. A missing folder icon or denied AX access fails this opt-in acceptance explicitly. On the current machine it found the existing Downloads icon in 32 AX nodes.

The fixture exposed and now covers Foundation's bookmark path alias (`/private/var/...`) differing from its canonical requested URL (`/var/...`). Scope ownership retains the security-scoped URL; exact-parent comparisons canonicalize it. Native opening repeats canonical parent and child device/inode checks after the external guard and immediately before workspace dispatch. Filesystem mutation can still occur between that final check and NSWorkspace resolving the URL; this is a narrow unavoidable path-based dispatch interval, not an atomic file-descriptor open guarantee.

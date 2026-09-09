# G04/G05 — User-controlled diagnostics and data-handling implementation plan

Approved backend contract, 2026-09-07, implemented in `.worktrees/parity-followups`. Scope: new pure diagnostics package and optional native export port, followed by separate App/UI integration. No actual chooser or user-file acceptance is authorized by this plan. G05 documentation is a parallel scoped deliverable. No telemetry or crash-flow redesign.

## Current evidence and gaps

- `app.go:dlog` writes debug strings to stderr only when `OPTIONTAB_DEBUG` is enabled. Normal slog uses process logging; no bounded application diagnostic log handler was found. Some errors include paths, e.g. settings load errors. Raw native errors and command/debug strings are not safe export fields.
- `internal/crash/crash.go` captures fatal Go runtime output through debug.SetCrashOutput to `crash-current.log`; nonempty current is promoted to `crash-pending.log` on next startup. `Pending` reads the whole pending file. The crash policy defaults to ask; it is independent of diagnostic export. The never path dismisses pending output but is not evidence that every existing current file or OS-native crash report has been erased.
- `app_crash.go:ReportCrash` includes up to 3000 raw characters in the query string of a GitHub new-issue URL. The browser request therefore sends those characters to GitHub before the user presses Submit. Existing wording about reviewing before submission must not imply that no data has left the machine yet. Changing that existing flow needs a scoped follow-up; do not silently bundle it into the new export.
- `GeneralTab.tsx` exports the whole settings object through a Blob/download link. No App native diagnostic save picker exists. Do not reuse the settings payload. The installed Wails save dialog exposes PromptForSingleSelection but no public cancellation/context handle; using it directly would not establish shutdown/cancel cleanup.
- Existing folder/lyrics native chooser owners provide a useful cancellable main-queue pattern, including holding the chooser slot until native completion. Reuse the pattern, not access bookmarks or folder/lyrics contents.

## Proposed user flow and boundaries

Provide “Diagnostics” in General/About support controls with: current report preview, Start recording (optional, bounded), Stop, Clear, and Save report. One report is inert local JSON or human-readable JSON text. Export never opens GitHub, a browser, email, clipboard or an upload endpoint. Copy/share remain separate explicit user actions outside this first feature.

A static snapshot works without recording and includes only supported version/platform fields, coarse permission states, feature status enums and safe aggregate counters. Optional recording is off by default, explicitly enabled for this process for at most 10 minutes, and bounded to 1024 typed events/256 KiB (whichever is reached first). When full, evict oldest complete events and record a dropped count. Stop keeps the bounded memory for review; Clear removes it. Shutdown discards it. No diagnostic log files or cross-launch retention are needed.

The review shows the exact immutable export bytes and omitted categories before opening Save. Save refers to a short-lived report token bound to that byte snapshot; it does not rebuild the report after the user reviewed it. One pending export at a time; busy refuses rather than replacing it. Suggested total serialized report limit 512 KiB, preview token lifetime five minutes, maximum one retained preview. A report generated after schema/permission/state change is a new token, not a mutation of the reviewed bytes.

Do not claim the report contains “nothing private” in an absolute sense. It includes product configuration/status facts the user may regard as private. Say concretely what it contains and excludes. Redaction means constructing an allowlisted model, not regex-scrubbing arbitrary strings.

## Exact payload policy

Allowed v1 fields:

- schemaVersion; Option Tab version/build identifier validated as bounded product version/hex; Go runtime version and OS/architecture enums; coarse OS release only if a read-only supported source is available. No system_profiler invocation, hostname, serial number or environment dump.
- Capture/accessibility/automation permission status enum from cached/read-only state, never prompting. Unknown remains unknown. No query of arbitrary target apps merely to fill diagnostics.
- Enabled/paused flags for an explicit reviewed list of Option Tab feature families; source status/reason code from a fixed dictionary; bounded error counts, retry counts, active subscription/capture counts and bucketed durations. No full settings serialization or reflection-based traversal.
- Relative event elapsed milliseconds and enumerated component/event/status codes with bounded numeric metrics. No wall-clock activity timeline by default; export creation time may be shown in UI but need not be embedded.
- Optional coarse display count/scale categories, without UUID, display name, position, precise geometry or persistent hardware identity. If unnecessary for first support cases, omit even those.
- Previous crash available boolean from safe bounded metadata only, with explicit `crashTextIncluded=false`. Never read the crash body during diagnostics collection.

Always excluded:

Window/document titles, app names/bundle IDs/PIDs/process-start values, window/Space IDs, focused-app history, user text, keyboard events/chords, clipboard, screenshots/thumbnails/artwork, media track/title/artist/lyrics/provider URLs, files/folders/URLs, bookmark data, paths/home directories, account names, network identifiers/addresses, tokens, raw settings/blacklists/hotkeys, raw slog/dlog/error strings, stack/panic bodies and native exception descriptions. No arbitrary `map[string]any` event attrs or `error.Error()` copied into an event.

A typed event constructor admits only an enum code and explicit numeric fields. Unknown codes become `unknown` and extra attributes are rejected, not stringified. The renderer receives only the final sanitized model/bytes; it cannot ask for raw records. Existing logs are not retroactively sanitized or bundled. If support needs a raw crash later, a separate review workflow must explain its contents and transmission boundary.

## Collection and ownership

New pure `internal/diagnostics` owns schema validation, finite limits, recording ring, counters, snapshot/token lifecycle and deterministic serialization. It accepts a clock and small typed StatusSnapshot/Record inputs. It does not access native APIs, disk, network, settings structures or arbitrary source errors.

App constructs the allowlisted status snapshot from existing controller snapshots and explicit settings fields, outside locks that native methods can reenter. Instrument only a short list of high-value lifecycle boundaries at first: source start/stop/refusal/retry, capture capacity/refusal, action result category and permission transition. This is a logging API for known support events, not a wrapper around all slog records. Native callbacks enqueue safe enums; no formatting, file I/O or App calls under tap/native locks.

Cancellation/session inactivity invalidates pending export admission and dismisses its chooser. Already recorded safe data may stay in the memory ring until Clear/shutdown; no app/window payload is retained. No need to stop ordinary diagnostics memory collection merely because a pane closes; recording has an explicit deadline. Terminal App shutdown cannot be undone by a late Save callback.

## Save port and native lifecycle

Suggested optional Go port: `SaveDiagnosticReport(ctx, suggestedName string, bytes []byte) (DiagnosticSaveResult, error)`, result status saved/cancelled/unavailable with no path in report/status events. The caller supplies only previously reviewed bounded bytes; suggested name is a fixed safe product filename. Native code validates limits again.

Implement a narrowly scoped NSSavePanel owner rather than pretending Wails' synchronous Prompt API supports cancellation. Creation/presentation/cancel on AppKit; disk writing off the main thread. Keep exact panel/token/session identity and a single slot until native completion drains, even if the initiating RPC context has already cancelled. Cancel before main-queue presentation must prevent the panel from opening. App shutdown must cancel without joining on the UI thread; join off-thread with no App mutex held. No callback after owner return.

After approval, retain only the chosen URL/access lease needed to write that exact target. Do not accept a path from frontend or automatically export beside settings. Do not follow a destination symlink to write arbitrary content. Use a same-directory private temporary file, bounded write, close/fsync as appropriate, then final context/owner check and atomic rename to the explicitly approved destination. V1 exclusively creates a new destination with parent-directory FD + atomic RENAME_EXCL. Existing files and symlinks refuse with stable destinationExists even if the system Save panel offered its usual overwrite confirmation; UI must say choose a new filename. Preserve existing destinations on all refusal/cancel paths. Cleanup only the exact created temp file. Once rename has committed, report saved even if cancellation races afterward; do not claim cancellation rolled back an already-created user export.

No persistent security-scoped bookmark or destination history is required. Do not log selected path or errors containing it. Map filesystem/native failures to reviewed codes such as permissionDenied, cancelled, ioFailure, hostClosed. Native file-write/chooser permission behavior and final replacement semantics require actual disposable-path acceptance before claiming support in distributed builds.

## G05 documentation content

Add `docs/data-handling.md`, linked from General/About and the README. Describe implemented behavior, not prospective guarantees:

| Area | Current handling to document |
| --- | --- |
| Settings | Per-user config path, on macOS `~/Library/Application Support/option-tab/settings.json`; settings export includes configured user entries and should be reviewed before sharing. |
| Folder grants | `folder-bookmarks.json` beside settings; security-scoped folder references, no automatic full-folder upload. Explain explicit chooser/access and removal behavior verified by code. |
| Lyrics | `media-lyrics.json` stores exact local association/bookmark references; local content read on explicit association, not a lyrics download/search service. Do not assert plaintext is never in memory. |
| Window previews | In-memory live/cached images under bounded owners; capture requires existing opt-in/permission. Automation image queries export only requested existing cached images after exact identity checks; scripts receive that data locally. No diagnostic bundle images. |
| Media | AppleEvents to enabled Music/Spotify providers may require Automation consent. Remote artwork is separate opt-in; enabled Spotify artwork requests go to public HTTPS URLs supplied by the provider, with DNS/redirect validation and limits. Hosts receive ordinary request/network metadata. Disabling cancels future/results but cannot recall an HTTP request already sent. Music local artwork is separate. |
| Updates | Default check policy contacts GitHub latest-release endpoint shortly after launch and daily; manual checks and selected policy behavior documented. Downloads use release assets when installation is requested/auto policy permits. Do not say no network by default: update checks default on. |
| Crash reporting | Local current/pending runtime crash files; retention/rotation/dismiss policy exactly as implemented. Explicit Report opens a GitHub URL carrying raw crash text, which reaches GitHub upon navigation. OS-native crash reports are managed by macOS separately. |
| Diagnostics | New feature's optional memory recording limits, exact excluded payloads, no automatic upload, user-chosen export file outside app-managed retention; users control copies after saving. Mark this section prospective until implemented. |
| Links/automation | Opening project/release/support links contacts the chosen external service via browser. Local AppleEvents automation is user-triggered and not an HTTP listener/telemetry channel. |

Document how to stop recording/clear diagnostics, dismiss crash output, disable optional provider/remote artwork/update checks, remove known local grant stores safely, and locate the config directory. Do not promise that disabling a feature revokes macOS TCC or erases user-exported files. No automatic cloud sync or analytics was found in inspected application paths; phrase conclusions as current Option Tab behavior, excluding OS/browser/provider activity.

## Minimal files and verification plan

Likely new files: internal/diagnostics/{types,recorder,export}.go + tests; app_diagnostics.go + tests; platform/diagnostics_export.go, darwin_diagnostics_export.go/m/h and stub + native seams. Narrow later hooks in main/App shutdown and known source status boundaries. UI General/About support component + bridge + EN/PT/ES strings. Documentation and README link. Avoid changing raw crash/settings export behavior without a separate regression-backed task.

Meaningful tests:

1. Sentinel secrets supplied in raw errors/settings/titles/URLs cannot appear in encoded report; unknown fields/codes and nested payloads refuse. No raw log/crash/bookmark/image reads occur. Schema additions fail closed.
2. Ring event/time/byte ceilings, complete-event eviction, dropped counts, finite metrics, deterministic ordering, no post-stop growth, Clear/shutdown release, one retained preview.
3. Review token binds immutable bytes; state changes after preview cannot modify export. Expiry/cancellation/shutdown/second-export busy produce no write. Export triggers zero HTTP/browser/clipboard calls.
4. Native seam: cancelled before main-queue presentation opens no dialog; cancel during chooser retains slot until drain; old completion cannot affect new owner. Write failure/cancel before rename preserves old target and removes only own temp; symlink and concurrent target creation refuse atomically; after-commit cancellation reports saved.
5. UI shows exact report and categories, cancellation distinct from failure, double-click is bounded, EN/PT/ES and keyboard accessibility, settings export unchanged. Browser Blob fallback is not silent native-export success.
6. G05 audit tests/checklist compares defaults/endpoints/store filenames with source; explicitly checks crash navigation wording and update-check defaults. Real chooser grant/cancel/write/overwrite acceptance uses only a disposable directory after root coordination; no native/UI acceptance claimed from mocks.

## Frozen service and implementation sequence

1. Pure package: New(Deps{Now,Exporter}), StartRecording (idempotent), StopRecording, Recording, Clear, CancelExport, Close, Record(Event), Preview(StatusSnapshot), Export(ctx,token). Clear stops/clears/invalidate/cancels; CancelExport only invalidates/cancels export. Close is terminal/nonblocking. Busy remains until source returns.
2. Typed data: enumerated component/event/status/permission and bounded Count/DurationMS only. Truthful PresentationRequested/PresentationRetired/ErrorReported codes describe publication intent rather than claiming native source acceptance. Review fields use camelCase JSON; report JSON remains the exact immutable reviewed bytes.
3. Optional platform.NewDiagnosticExportSource implements SaveDiagnosticReport(ctx,fixedName,data). Result is status only; stable typed errors carry no path. No non-Darwin fake success.
4. Native chooser owner: cancellation before queued presentation prevents opening; retained owner joins completion/write cleanup off AppKit. Exclusive new-file atomic write, mode0600, bounded bytes, no persistent destination bookmark. Native OS filesystem latency itself is not a hard wall-clock guarantee.
5. Parent integration: explicit safe event names only; no payload inspection, no raw logger wrapping. Stop/cancel hooks preserve lock ordering. General/About review/save controls and EN/PT/ES translations follow frozen APIs.
6. Focused pure/race/native-fixture tests precede App/UI integration and independent review. Real NSSavePanel approval/cancellation, sandbox/distributed save behavior and user-visible filesystem handling remain separately coordinated acceptance.

# Runtime Dock reordering and Preferences concurrency

H05 adds optional dragging and keyboard commands to move existing Dock pins, create app groups, transfer apps between groups and remove group members. Custom icons and Settings-based item editing remain available from H04. Runtime reordering defaults off per profile and has English, Portuguese and Spanish controls.

Dragging begins from an explicit handle after six logical pixels. Stable, untransformed item boxes preserve drop targeting during magnification. Escape, pointer cancellation, release outside a valid target, presentation retirement and unmount discard the draft. A completed drag suppresses its synthetic activation click. Missing configured pins retain structural controls; running items and decorations cannot be drag sources. Decorations remain valid insertion targets. Expanded groups keep each handle on its own icon. External file, text and URL drops are ignored.

The mutation RPC accepts only a closed operation and existing opaque item IDs. App validates the current profile, full presentation scope, item-content hash, exact native panel incarnation and original ordinary display Space. A monotonic display admission also rejects coalesced Space A→B→A transitions. The pure reducer copies and validates both sides of a mutation, preserves reference metadata and enforces the existing limit of 16 records including group members. App reads current settings after acquiring the shared persistence lock and validates admission again before saving. A save already admitted to disk may complete; cancellation does not claim rollback.

Preferences now uses a process-local settings revision and compare-and-swap saves. All successful settings writers advance the revision with the canonical snapshot; failed persistence does not. Reopening a retained Preferences window emits generation-tagged loading and canonical state events before edits resume. A stale full-settings save cannot overwrite runtime pin edits. Reordered refresh events, newer optimistic edits and late save/import/native-mutation completions are covered by ownership checks. Imports preserve original JSON bytes for strict Go decoding. A coherent read does not wait for the persistence lock or native work.

## Verification

- All 25 Go packages pass race/coverage tests; golangci-lint reports zero issues.
- 402 desktop, 7 shared and 4 website unit tests pass. Workspace Biome, TypeScript and production builds pass.
- All 91 desktop Chromium checks pass, including four runtime reorder cases and the existing standalone Settings route.
- Pinned Wails generation reports 125 methods and 70 models.
- Real CGO arm64 and x86_64 builds pass, with minimum macOS 14.0 recorded by `vtool` for both. Binaries were not launched or distributed.

Focused regressions cover mutation/group invariants, stale native hosts, final native refusal, Settings/runtime writer races in both orders, failed disk saves, exact scope/hash admission, Space ABA, concurrent CAS saves, nonblocking reads and retained Preferences refresh. Browser cases exercise pointer dragging with magnification, ignored external drops, keyboard grouping, expanded vertical group handles and decorative insertion markers through the generated RPC boundary.

Review and integration checks found and fixed stale completion ownership in Settings, refresh-event reordering, an expanded vertical group's misplaced handle and missing decorative target markers. The full browser suite also caught the new admission guard blocking local Settings demo edits after confirming no backend; a focused regression now preserves that fallback while real-backend load failures remain blocked. Existing browser fakes were updated to model revisioned writes and publish imported canonical settings. No verification hook was bypassed.

## Remaining acceptance

Physical pointer capture and dragging inside the nonactivating Wails/WebKit panel remain native acceptance checks. Automated browser tests use a fake Wails backend and do not establish actual native actions. Gestures, letter navigation and supported badge sources remain retained follow-ups. This checkpoint does not create a new release.

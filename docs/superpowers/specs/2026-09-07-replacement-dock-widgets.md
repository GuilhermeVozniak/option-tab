# H13/H14 — Declarative widget extensibility proposal

Status: independent design proposal only, 2026-09-07. No implementation, installation, native provider test, permissions, GUI or network execution. Depends on approval of the separate replacement-Dock lifecycle proposal. H17 profile backup and H12 audio controls are consumers of this boundary, not implemented here.

## Decision

Start with a versioned declarative widget package interpreted by trusted Option Tab code. Community authors compose a small layout vocabulary and bind fields from explicitly granted, host-owned data providers. They cannot ship JavaScript, HTML, CSS, WebAssembly, native libraries, shell/AppleScript, executables, timers, network clients or filesystem expressions. Installation is inert; activating a widget instance is separate opt-in execution of its declarative bindings.

Alternatives considered: arbitrary WebKit pages give authors flexibility but expose a second unbounded UI/network/runtime surface; sandboxed executable plugins require a separate isolation, IPC, signing and revocation project. Neither is necessary to deliver the retained clock/battery/network/audio widgets and stacks. Do not quietly treat “sandboxed” as permission for downloaded code. Future executable extensions require a new explicit design and approval, not a manifest flag.

The approved spec explicitly requires independent widget design, capabilities and opt-in execution, and excludes calendar/weather/saved commands/file staging/AirDrop. No capability alias may reintroduce them. Widgets cannot invoke F automation as a back door to arbitrary window actions.

## Package and manifest v1

Prefer an explicitly chosen local directory or bounded archive containing `widget.json` plus declared PNG assets. No remote marketplace downloader or auto-updater in v1. A package ID is stable ASCII reverse-domain-like text, not a claim of publisher verification. Version is validated semantic version, schemaVersion=1, minimum supported Option Tab version is explicit. Installation records package digest and source display name separately from the author-provided identity.

Suggested manifest fields: id, version, schemaVersion, name/description localized strings, required/optional capability enums, bounded user-setting schema, one root layout tree, and a declared asset list. Reject unknown executable fields and unknown capabilities rather than silently allowing them. Only trusted renderer tokens control materials, font scale, spacing, colors and icon sizes; no raw style strings, CSS selectors or resource URLs.

Layout nodes: row, column, text, icon, progress, sparkline (bounded history) and button. Stacks are a host feature containing explicitly configured widget instances; a manifest cannot instantiate other packages or recurse into itself. Bound depth (8), node count (128), children per node (16), text lengths, history samples (120), rendered min/max dimensions and per-instance resource use. Text binding supports literal + direct typed field reference + fixed host formatter, not arbitrary expressions, regex, templates with evaluation, loops or property-path traversal into App state.

Example: a clock node binds provider `clock`, field `time`, formatter `shortTime`, timezone from the validated widget setting. Its only capability is `clock.read`. The runtime owns tick scheduling. Locale/timezone IDs use trusted platform formatting; packages never supply formatting code. EN/PT/ES built-in settings/error/review strings remain product-owned; community strings may declare locales with safe fallback, always escaped as text.

Proposed installation limits: compressed package ≤4 MiB, expanded total ≤8 MiB, ≤64 entries, manifest ≤256 KiB, asset ≤512 KiB encoded and decoded image ≤4 megapixels. Initially PNG only; no SVG/HTML/font loading. Reject traversal/absolute paths, symlinks, hard links, duplicate/case-fold collisions, ambiguous Unicode names and archive expansion beyond limits while streaming. Canonicalize and validate before creating a private staging directory; never follow a package-supplied link outside it. Verify every declared asset digest/type before atomically publishing an immutable content-addressed install. Use a validated package handle for reads, not arbitrary manifest paths.

## Capabilities and consent

Capability catalog v1 should remain small:

| Capability | Read/control scope | Default behavior |
| --- | --- | --- |
| clock.read | Host time with selected validated timezone | No subscription until instance enabled |
| battery.read | Charge/power-source status, no device identity dump | Read-only, shared provider |
| network.status.read | Connectivity/interface category | No SSID, IP addresses, packet data or remote request |
| network.usage.read | Aggregate local byte-rate samples if native source supports it | Separate opt-in; bounded history |
| audio.status.read | Current output display name and volume/mute if supported | No microphone/input stream |
| audio.output.select | Select one currently enumerated output device | Explicit click, exact device identity and final owner guard |
| media.music.read / media.spotify.read | Existing scoped provider metadata | Existing app-level consent plus widget opt-in; never request from polling |
| media.music.control / media.spotify.control | Fixed existing transport controls | Explicit click/seek and exact current media scope |

Unavailable providers expose unsupported/unavailable, not made-up readings. Permission revocation must invalidate pending actions and subscriptions. No ambient network.fetch, file.read, process.execute, automation.send, window.close or arbitrary openURL capability in v1. General pinned files/links are H04 product items and keep their separate validation/consent path.

Install review shows package name/version/digest, unverified publisher status if appropriate, requested data and explicit actions. Installation alone grants nothing. Enabling an instance grants a subset of declared supported capabilities bound to installed package digest and profile instance. Required-capability refusal keeps the instance disabled with a readable explanation; missing optional capabilities produce a defined unavailable slot. Native permission requests remain separate explicit actions using existing source consent flows; no prompt on startup, import, preview, profile switch, subscribe or retry.

Package update creates a new immutable digest and staged review. Never auto-expand a grant based on matching package ID; old enabled version remains until an explicitly accepted swap. Cancel/join old subscriptions before activating the replacement. Removing a package disables its instances and retires guards first, then releases resources; filesystem deletion happens after live handles drain.

## Runtime lifecycle and data isolation

Use one pure widget controller/runtime with typed provider registry. It shares provider owners across instances/displays and subscribes only while at least one eligible visible instance needs data. Hidden stacks release expensive subscriptions after a bounded grace interval; clocks may retain host-local formatting state without a provider worker. Paused/inactive/disabled replacement releases all widget subscriptions independently of saved enabled flags. Other media pins or keyboard captures are not stopped.

A data lease includes installed digest, instance ID, display presentation session, profile revision, widget admission epoch and provider generation. Every queued sample/action verifies all components. Coalescing cannot erase disable→enable invalidation. Keep one bounded latest snapshot per instance/provider; sampling continues while a serial action prepares. Busy action requests refuse explicitly rather than silently replacing an already-consumed control action. No callbacks after a completed source lifetime; old results cannot enter a newly selected stack/profile.

Provider samples are typed field maps validated against a fixed catalog, with observation timestamp, sequence, status and reason. Missing/stale/unsupported is represented separately from zero. Rate budgets belong to the host: e.g. clock once a second (minute-only widgets slower), battery/status coalesced, usage ≤1 Hz, media through its existing shared source. A package cannot choose an unbounded polling interval or retain unlimited history. Animation is trusted UI-only and pauses offscreen; it does not increase native sample frequency.

UI renders only trusted React components and validated data. No innerHTML, iframe, external stylesheet, package-authored URL or Wails method name is accepted. Asset URLs are created by the host from admitted bytes; CSP is defense in depth rather than the primary manifest boundary. Errors are per-instance and do not crash the display host or hide the native recovery route. The host owns accessibility labels/focus order and keyboard handling; manifest text cannot redefine recovery controls.

Commands contain an enum, a typed target from the current provider snapshot, and the full instance/session/revision lease. Go validates them independently of renderer state. App/native adapters use the existing guarded-command pattern: check before preparation, after external lookup, immediately before native dispatch, and before publishing completion. No capability mutex across App/native callbacks. Revocation cannot recall a native action already entered; that limitation remains visible, with the preparation gap guarded.

## Narrow implementation boundaries to freeze later

- `internal/widgets`: manifest/schema validation, immutable package model, pure instance lifecycle, provider dependencies, data reduction and guarded command admission. No AppKit, general scripts or networking.
- Installer/storage: explicit user selection, bounded parse/extract, private staging/atomic install and digest-bound grants. No execution during validation. Profile export includes manifest reference/config but not bookmarks, opaque native handles, permission tokens or live provider state.
- Platform providers: separate fixed battery/network/audio read/action ports returning typed status. Do not expose a generic string-to-native API. Reuse current media controller rather than creating one observer per widget.
- App bridge: GetWidgetState/explicit command with exact instance and presentation revision; bounded update DTO. No call-by-name manifest mechanism. Replacement lifecycle owns display visibility; widget runtime cannot keep a hidden native panel alive.

## Required tests and acceptance

Pure/adversarial tests: unknown fields/capabilities, duplicate keys, huge/deep nodes, malformed locale/format/settings, path traversal, symlink/hardlink archives, case collision, compression bombs, undeclared assets, image decoding limits, and package digest mismatch. Assert no file/network/process/native permission call during parse/install preview.

Capability tests: no grant on install, disabled instances never subscribe, optional/required denial, immutable digest change cannot inherit expanded authority, revocation during blocked preparation gives zero action, no control through an undeclared capability. Validate all strings/URLs at the Go boundary, independent of frontend checks.

Lifecycle tests: two displays share one provider; hiding/removing one instance leaves another live; last unsubscribe cancels and joins; immediate resubscribe waits for old owner; profile A→B→A rejects old samples/actions; stopped runtime cannot resume; source-error/retry messages belong only to current epochs. Test bounded queues and a blocked renderer/source/action without global owner deadlock.

Renderer tests: hostile text is escaped, no remote URLs or scripts are created, malformed/oversized data degrades one widget, stacks retire inactive instances, large labels do not overlap recovery controls, native pointer-driven hover works in non-key hosts. EN/PT/ES product copy and accessible actions are required.

Native acceptance is a later explicit step against read-only power/network/audio status and disposable controls where possible; audio-output changes need coordination. No real provider correctness, TCC delivery, physical gestures, signed distribution behavior or downloaded-package safety claim is established by this proposal. Widget and lifecycle designs must both be approved before H coding.

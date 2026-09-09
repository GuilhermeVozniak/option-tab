# Focused-app profile switching

Implement retained H03 in widget-services after the audio/package prerequisite checkpoint. Existing profile/lifecycle/recovery contracts remain authoritative. No GUI activation, cursor/Dock action, preferences mutation or release during automated tests.

## Frozen data and selection

Add optional `Rules []LauncherProfileRule` to ReplacementDockSettings version2; absent means no rules. A rule has `ID`, `Enabled`, `BundleID`, `ProfileID`, and optional `BindingID` (empty means every resolved binding). JSON uses lower-camel names. Maximum8 rules; unique bounded stable IDs; nonempty bounded bundle identifier with no whitespace/control/path characters; profile references must exist, and nonempty binding references must exist. Reject rules supplied under legacy version1 rather than silently importing new behavior. Clone rules with all settings snapshots. No PIDs/process starts or display coordinates are persisted.

Rules are ordered: the first enabled exact bundle-ID match applicable to the resolved binding chooses its profile. Match bundle identity, never app display name or fuzzy text. No rule, unavailable focused-app evidence or no matching rule uses the binding's saved base profile. Rules cannot create a Dock on an unbound display. A matching rule affects only its configured binding scope. The normal native environment, Space, Dock, pause/preferences/session and permission gates still decide whether a surface is available.

Add `FocusedProcess ProcessIdentity`, `FocusedBundleID string`, `FocusKnown bool` to backend LauncherEnvironment. Native code observes NSWorkspace.frontmostApplication with matching nonterminated PID/start/bundle before and after the read; dirty notification on app activation plus the existing bounded polling. Unknown evidence clears the fields. It never activates an app. Go normalizes contradictory/invalid focused evidence to unknown without invalidating otherwise valid display topology.

On effective ProfileID change, retire the old interaction/session and create a fresh host/session before new actions are admitted, even when the same app list, display and Space remain. No profile switch for an unmatched app may churn an unchanged effective profile. Preserve per-display independence. Old clicks and late callbacks must not act after a focus-triggered switch. Returning A→B→A uses fresh sessions, not the earlier A host.

## UI and App

Settings adds an ordered Focus rules editor: choose app from a bounded running-app list or enter its exact bundle identifier, choose destination profile, optionally scope to an existing display binding, enable/disable, reorder and remove. Show the base profile separately. Removing/reassigning a profile updates rule references atomically with binding references. Removing a binding drops its specifically scoped rules as part of that explicit removal and leaves global rules unchanged; describe that consequence in the control hint.

Root adds `GetLauncherAppChoices() []LauncherAppChoice` (`name`, `bundleID`) as an explicit settings-only inventory query, bounded and deduplicated by bundle identity; no window query, native action or permission prompt. It returns empty on unavailable source and must not expose PIDs. Existing SaveSettings is the persistence path. No optimistic launcher activation or extra hotkey is introduced. EN/PT/ES labels and visible validation/errors.

## Ownership and tests

- Pure worker: config rules/validation/cloning/legacy decode checks plus launcher profile resolution/session retirement and focused tests. Do not edit native/App/frontend files.
- Native worker: launcher environment contract, native focused snapshot and app-activation dirty notification, mapping/normalization and native seams. Do not alter app activation or panel policy.
- UI worker: settings rule editor, typed fields, profile/binding reference maintenance, fake RPC and focused unit/browser checks. No generated files or Go.
- Root: App inventory RPC/integration tests, generated bindings, combined verification, git/PR.

Tests cover ordered scoped rules, disabled/unknown/unmatched fallback, duplicate app names with distinct bundle IDs, A→B→A retirement, two-display independence, malformed/legacy rules, deep copies, delete/reassignment, stale click refusal and no native side effects from observation. Native tests use injected focus identities and notifications; physical focus/Spaces acceptance remains separate.

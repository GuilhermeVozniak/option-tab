# Widget runtime and installation checkpoint

This branch implements H13/H14: Clock, Battery, Network and Audio widgets; declarative community packages; settings, grants and stacks; and local package review, installation and removal. It is unreleased work after v0.4.8. H13/H14 remain unchecked until packaged native acceptance is complete.

## Delivered behavior

- Each profile supports up to four visible widget slots and sixteen configured instances. A stack has two to four members; only its selected member subscribes to providers. Stack selection belongs to the current Dock presentation and does not rewrite the saved default.
- Built-ins use the same verified manifests, grants and renderer as community packages. Clock supports format and timezone. Battery distinguishes absent/unavailable from zero charge. Network separates local connection status from optional traffic rates. Audio separates output status from explicit output selection.
- Enabled instances need their declared required grants. Optional denials leave permitted content usable. Disabling, changing grants, switching profiles/stacks or losing the parent Dock presentation retires outstanding actions and subscriptions. Providers are shared across admitted instances and joined before replacement.
- Music/Spotify extensions attach to the existing media controller and retain its app-level enablement, consent and command restrictions. Progress updates keep an open action usable; changes to the provider, track, action authority or instance lifetime retire its tokens.
- Renderer input is a bounded host-resolved tree of rows, columns, text, icons, progress, sparklines and buttons. Packages cannot supply JavaScript, arbitrary paths, shell commands, URLs or native action targets. Controls require an explicit user action and current host-issued authority.
- Settings offers typed manifest settings, explicit capability choices, stacks and package management in English, Brazilian Portuguese and Spanish. New instances start disabled without grants. Only the exact former built-in Clock reference receives the recognized built-in migration.
- Local package review copies and verifies one bounded archive. An expiring token refers to those exact bytes. Installation does not enable widgets or grant capabilities. Removal first saves disabled, ungranted references and retires runtime admission; failed deletion keeps a retryable inert catalog entry. A committed installation reconciles the catalog even if Preferences closes while its reply is pending.
- Startup revalidates the private installed catalog. Unsupported minimum versions, corrupt packages and missing references never acquire providers. Native chooser cancellation and store shutdown retain ownership until outstanding work has joined.

The [widget guide](../../widgets.md) describes supported extension capabilities and includes a validated example. Package boundaries remain 4 MiB compressed, 8 MiB expanded, 64 archive entries, 128 layout nodes and 120 history values. No weather or calendar provider was added.

## Verification

- Complete Go race/coverage suite passed across all 24 packages after App, configuration, layout, provider and runtime integration. Global Go lint reported zero issues.
- Follow-up package-manager tests passed after injected deletion failures and post-commit cancellation/replacement barriers were added. They verify that old owners and old grants cannot be restored.
- Focused native fixtures cover battery/network mapping, missing data, interface/counter transitions, cancellation, chooser file identity and package bounds. Linux/amd64 platform test cross-compilation passed with CGO disabled.
- Runtime tests cover sharing, required/optional grants, immutable snapshots, bounded history, owner replacement, token authority, stale actions and cancellation during preparation. App tests include native-parent rejection immediately before an action, profile/stack retirement and exact built-in migration.
- The documentation's literal example passed both manifest parsing and real ZIP preview validation through a temporary test overlay.
- Pinned Wails binding generation succeeded: 96 methods and 57 models.
- JavaScript unit tests passed: 329 desktop tests, seven shared tests and four site tests. Workspace lint and production builds passed. The complete desktop Chromium suite passed all 76 tests; the seven launcher/settings scenarios were rerun after the settings fixes.
- Review regressions verify exact package-digest selection when multiple versions share an ID, per-version settings/grants, existing-stack capacity, and retirement of a removed member from a pending stack selection. All exposed failures before the fixes and passed afterwards.
- Native CGO builds passed for arm64 and x86_64. Mach-O metadata confirms a macOS 14.0 minimum on both. These local unsigned binaries were not installed, launched or distributed.


## Acceptance still pending

Injected sources and browser fixtures do not establish real laptop/desktop battery behavior, actual route/VPN changes, device hotplug or audio selection, live player control/seek, visible NSOpenPanel/security-scoped access, or packaged multi-display widget/stack/recovery behavior. Those checks remain distinct from automated implementation evidence. No native device, player, chooser, cursor or Dock action was performed for this checkpoint.

B10 materials for the existing switcher/preview surfaces and the remaining replacement-Dock items are separate retained work. This checkpoint does not complete the full roadmap or authorize a new release.

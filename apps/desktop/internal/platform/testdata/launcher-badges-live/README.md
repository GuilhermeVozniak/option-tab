# Controlled live Dock-badge acceptance

This fixture uses only its own `NSApplication.dockTile.badgeLabel` and `display` APIs. It creates no window, sends no input, requests no permission, calls no application activation method and modifies no Dock preferences. A regular application may add its own temporary Dock icon when explicitly launched. Actual foreground preservation and Dock/AX delivery remain live acceptance facts, not compilation claims.

The app accepts only `count`, `indicator`, `empty`, and `quit` newline commands over its private stdin. Values are fixed to `7`, `!`, and empty. Commands are limited to 8 and each line to 31 bytes. Unknown/oversized input, EOF, quit, or a 45-second native watchdog clears its own badge and terminates. Readiness reports only the fixture's own PID/start/bundle URL/identifier; acknowledgements report only the accepted command. It cannot address another application.

`TestLauncherBadgesLiveOwnedFixture` **skips immediately** unless `OPTION_TAB_LIVE_BADGE_FIXTURE=1`. Ordinary `go test` therefore does not compile or launch this fixture, create NSApplication, set badges, or query remote AX through this test. The live driver creates a unique temporary `.app` and bundle identifier, launches that executable only, verifies its announced PID against the exact child process plus native process-start identity and canonical bundle URL, and revalidates the production running-target resolver before/after each step. Foundation's native path is also resolved to the exact driver-owned filesystem path, accommodating macOS `/var` aliases without weakening identity. One actual `NewLauncherBadgeSource` observes only that immutable fixture target. Every phase requires a newer observation after its command starts. Count 7 and indicator must be known. Removal must clear the prior count/indicator, either to known absence or to an exact-target unavailable entry with no kind/count while the source is ready. The latter is reported explicitly and does not prove known absence or zero. Unsupported data, source unavailability and timeouts fail. No raw badge text or user app identities are logged.

Source observation is canceled and joined before cleanup. Only the launched child receives quit/kill; the driver waits for child and protocol termination. The 40-second driver context also bounds execution. No settings, grants, window actions, external player, network source, or capability implementation is involved.

## Compile only (safe preparation)

From `apps/desktop`:

```sh
fixture_dir=$(mktemp -d /tmp/option-tab-badge-compile.XXXXXX)
sh internal/platform/testdata/launcher-badges-live/build.sh "$fixture_dir/BadgeFixture.app"
go test -c ./internal/platform -o "$fixture_dir/platform.test"
```

These commands do not run either executable. `build.sh` refuses an existing destination, creates only the requested new bundle, and compiles for macOS 14 minimum. SDK declaration checked in `AppKit.framework/Headers/NSDockTile.h`: public copied NSString `badgeLabel` and `display`.

## Explicit live invocation

Reserve the native desktop slot first. This invocation intentionally creates a temporary Dock icon and sets its badge. Do not run it as part of unattended/default CI:

```sh
cd apps/desktop
OPTION_TAB_LIVE_BADGE_FIXTURE=1 go test ./internal/platform \
  -run '^TestLauncherBadgesLiveOwnedFixture$' -count=1 -timeout=60s -v
```

Existing Accessibility access must already permit the actual source. The test never opens a permission dialog. The directly launched packaged executable still needs actual macOS bundle/LaunchServices/Dock compatibility verification; if refused, preserve the failure and coordinate a fixture-only launch adjustment. Do not weaken target validation or substitute injected extraction to manufacture a live pass.

The result establishes only this controlled badge transition on the tested OS, not unread-message meaning, universal third-party badge support, cross-display behavior, permission recovery, or foreground preservation.

## Observed result — 2026-09-08

On macOS 26.6.2 arm64, the actual production source read known count 7 and an indicator from this exact fixture. Clearing its badge produced a ready-source unavailable entry with no kind/count. The test verified that typed transition and joined the observer, owned child and protocol cleanup. Known absence remains unverified. An initial driver refusal exposed the Foundation/Go path alias difference; resolving filesystem identity corrected the test driver. Production source behavior was unchanged. This is source-level evidence; live launcher rendering and the broader acceptance matrix remain separate checks.

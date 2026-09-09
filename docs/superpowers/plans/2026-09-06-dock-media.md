# Dock Media Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Delegation still requires coordinator authorization. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver E04 Spotify/Apple Music details and transport, E05 synchronized lyrics from explicitly supplied timestamped files, and E06 independent pinned media panels.

**Architecture:** Optional typed native AppleEvent adapters feed a bounded Go media owner. Dock hover selects the provider by exact app identity; pinned presentations share its observations but own independent presentation sessions. A pure lyric timeline follows sampled player position, pause, seeks, and track changes. No private MediaRemote, general script evaluator, audio capture, or additional gesture tap.

**Tech Stack:** Go, Objective-C/Foundation AppleEvents, existing Wails nonactivating NSPanel host, React/TypeScript, native file chooser/security-scoped bookmark support.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`, retained roadmap E04–E06 in `docs/dockdoor-roadmap.md`. H12 seek groundwork is included; audio-output switching remains its separate retained milestone.

**Implementation checkpoint, 2026-09-07:** Typed adapters, provider ownership, bounded artwork, local lyrics, hover composition, independent native pins, settings and localization are implemented and automated checks pass. See [evidence and remaining acceptance](../reports/2026-09-07-dock-media.md). The same-hover Windows/Media selector is still a retained implementation follow-up. Task 6's physical/player acceptance is pending; unchecked combined steps below must not be treated as completed native acceptance.

## Global constraints

- Preserve Option Tab's keyboard switcher and existing settings; new media features default off. Do not add tiling, calendar/weather, staging/AirDrop, saved commands, shell execution, or other DragZone behavior.
- Use exact running provider processes; never launch or activate a player to inspect it. No commands, chooser, Automation prompt, or network request solely because an icon is hovered.
- Consent is per provider and explicit. A master enable exposes status; a dedicated Connect button requests Automation consent. Merely loading settings or polling uses no-prompt preflight.
- AppleEvents run off native UI/event-tap callbacks. No Go mutex held across native main dispatch. Limit each provider to one in-flight operation; cancelled work drains before replacement.
- Missing metadata, unsupported seek/artwork, unavailable lyrics, disconnected network and permission denial are visible states, never fabricated playback or synchronized lyrics.
- Local lyric import does not establish copyright ownership. Import only an explicitly selected local file; retain it privately, do not bundle, redistribute, upload, discover, or download third-party lyrics.
- All native acceptance manipulating playback targets a disposable fixture first. Real Spotify/Music commands and permission prompts require a separately coordinated explicit acceptance step; this plan does not authorize those actions.

## Evidence and compatibility boundaries

Read on 2026-09-06: `/Applications/Spotify.app/Contents/Resources/Spotify.sdef` and `/System/Applications/Music.app/Contents/Resources/com.apple.Music.sdef`. Both expose current track, player state, position, and transport. Music exposes persistent ID, artwork raw data, and plain text lyrics; plain lyrics are not evidence of timestamp support. Spotify exposes artwork URL; its old artwork property explicitly never returns data. Installed dictionaries are app-version contracts, not proof every stream/ad implements them.

Use [Foundation's typed event sender](https://developer.apple.com/documentation/foundation/nsappleeventdescriptor/sendevent(options:timeout:)) and [Automation permission preflight](https://developer.apple.com/documentation/coreservices/aedeterminepermissiontoautomatetarget). Confirm the exact declaration and result codes in the local SDK during implementation. Address an existing PID, use noninteraction event options and finite timeouts, and revalidate launch identity; never a bundle-target send that can autolaunch. Package `NSAppleEventsUsageDescription` and the applicable hardened-runtime Automation entitlement; validate the signed fixture app rather than assuming CLI permission carries over.

Spotify's installed dictionary describes duration as seconds, but desktop-version behavior must be measured against a known fixture/real permitted track before enabling seek. Freeze tested unit conversion per adapter/version evidence; do not guess units from track length. Position is seconds in both dictionaries. Music track duration is real seconds.

Spotify's [design guidance](https://developer.spotify.com/documentation/design) requires provider attribution for its metadata/artwork. Its [Web API seek page](https://developer.spotify.com/documentation/web-api/reference/seek-to-position-in-currently-playing-track) also documents restrictions applying to that API; this plan does not use the Web API, OAuth, or its playback SDK. Do not infer a general lyric license or that local AppleEvents waive provider terms. Local user-supplied LRC is the no-service integration baseline. Network lyric service integration requires a separately reviewed agreement/API with synchronization/display/cache rights; no speculative provider settings are added now.

## Frozen port proposal (Task 1 must freeze before parallel integration)

Create `apps/desktop/internal/platform/media.go`, native `darwin_media.go/m/h`, and `media_stub.go`. Keep the platform package independent of the media reducer.

```go
type MediaProvider string // only "music" or "spotify"
type MediaProcess struct { PID int; LaunchID string }
type MediaTrack struct { ID, Title, Artist, Album string; DurationMS int64 }
type MediaCapabilities struct { Play, Pause, Previous, Next, Seek bool }
type MediaSample struct {
    Provider MediaProvider
    Process MediaProcess
    Generation, Sequence, TrackEpoch uint64
    Track MediaTrack
    Playback string // playing | paused | stopped | unknown
    PositionMS int64
    ObservedAt time.Time // monotonic clock retained inside Go
    Status, Reason string
    Capabilities MediaCapabilities
    ArtworkToken string // opaque, never a generic file/URL RPC
}
type MediaScope struct {
    Provider MediaProvider
    Process MediaProcess
    Generation, TrackEpoch uint64
    TrackID string
}
type MediaCommand struct { Scope MediaScope; Kind string; PositionMS int64 }
type MediaPermission struct { Status, Reason string }
type MediaArtwork struct { PNG []byte; Status, Reason string }
type MediaProviderSource interface {
    ObserveMedia(context.Context, MediaProvider, func(MediaSample)) error
    RequestMediaPermission(context.Context, MediaProvider) (MediaPermission, error)
    PerformMediaCommandGuarded(context.Context, MediaCommand, func() error) error
    ReadMediaArtwork(context.Context, MediaScope, string) (MediaArtwork, error)
}
```

Statuses: `ready`, `notRunning`, `permissionRequired`, `denied`, `unsupported`, `unavailable`; preserve the failing provider's reason independently. Artwork adds `networkDisabled`, `offline`, `missing`. Never convert absent track to an actionable empty ID. Track epoch increments whenever observed track identity changes, including A→B→A; process generation changes on restart/replacement. Without a stable track ID, expose readable metadata but refuse seek and track-scoped commands rather than identify by title alone.

`ObserveMedia` holds no UI ownership. The owner starts it only for an enabled provider with a visible hover or pin subscriber, pauses on session inactivity, and cancels/joins at zero subscribers. Poll at 500ms while playing, 1s paused, back off unavailable/permission-denied to 5s; no prompt during retries. Permission request ownership is separate from hover, so hiding the preview does not abandon an accepted OS consent operation. Disable/shutdown retires its result; an already displayed system TCC dialog may not be programmatically dismissible. Do not promise cancellation closes that system dialog.

Commands are only `play`, `pause`, `previous`, `next`, `seek`; no toggle whose meaning depends on stale state. Reject unknown kind, missing scope, NaN/conversion overflow or out-of-range seek. Immediately before send: recheck running PID/launch, current track ID and epoch evidence, then call the Go presentation guard after all native preparation. AppleEvents cannot atomically compare-and-set track plus seek: document the narrow player-autoadvance race between final track read and delivery, never claim atomicity. Serialize commands per provider with bounded pending capacity and explicit busy refusal; never silently lose accepted clicks. Seek coalescing occurs before acceptance during slider movement; send one latest value on release.

### Task 1: Typed adapters and truthful provider state

**Files:** create the platform files above plus `media_test.go`, `darwin_media_test.go`, and `testdata/media-events/` fixture sources/README. Modify packaging's existing macOS plist/entitlement templates only after locating the active build inputs.

- [ ] Write a fake event transport regression: a PID disappears after lookup; sampling and commands return `notRunning`, with zero target-launch calls. Record RED using `go test ./apps/desktop/internal/platform -run Media -count=1`.
- [ ] Write tests for no-prompt denied preflight, provider-independent failure, finite timeout, wrong descriptor types, oversized metadata, track identity changing between metadata reads, and a retired final guard causing zero mutation.
- [ ] Implement fixed property/event descriptors from the installed sdefs: application current track `pTrk`, state `pPlS`, position `pPos`; provider-specific track identity and transport suite codes. Read identity before and after a multi-property sample; discard inconsistent snapshots. Bound strings to 4KiB, reply artwork as below. No generated script text or AppleScript interpolation.
- [ ] Implement explicit permission requests, process lifetime validation and unsupported stubs. Publish copied values off the native main thread; callbacks never contain native references.
- [ ] Run focused race tests. Native fixture implements the exact read/command event subset and records addresses/opcodes; assert unexpected event/property is refused, only the fixture PID receives commands, and cancellation joins bounded in-flight sends. A seam alone does not establish real player compatibility.

### Task 2: Media ownership and command admission

**Files:** create `apps/desktop/internal/media/controller.go`, `controller_test.go`; modify config settings/defaults/normalization tests with `Dock.Media.Enabled`, `MusicEnabled`, `SpotifyEnabled`, `RemoteArtwork` booleans, all false by default. No bookmarks, credentials or lyrics bytes in settings export.

**Interfaces:** `NewController(Deps{Source platform.MediaProviderSource, Changed func(State)})`; `Configure(Settings)`, `Run(context.Context)`, `Subscribe(provider) (subscriptionID uint64)`, `Unsubscribe(subscriptionID)`, `Command(ctx, subscriptionID, scope, kind, positionMS, guard) error`. `State` contains provider sample and monotonically increasing owner revision. Lifecycle commands are nonblocking; command RPC returns a result without holding App locks.

- [ ] Test two subscribers sharing one provider observation, removing one preserving the other, last removal cancelling/joining, disable during blocked callback dropping old state, and reenable not adopting the old generation. Record RED with `go test ./apps/desktop/internal/media -run Controller`.
- [ ] Implement one owner loop with per-provider bounded mailboxes, immutable state copies and callback run-epoch guards. Retain provider failure independently; source failure retries are cancellation-aware and bounded.
- [ ] Test command queue overflow returns busy; stale session, process restart, A→B→A track change and close during native preparation refuse the final guard. Read snapshots never issue commands or permission requests.
- [ ] Run `go test -race ./apps/desktop/internal/media ./apps/desktop/internal/config` and request independent lifecycle review before App wiring.

### Task 3: Bounded artwork without hidden network access

**Files:** create `apps/desktop/internal/media/artwork.go`, `artwork_test.go`; implement the platform artwork method in Task 1's owned adapter with coordinated ownership.

- [ ] Test Music missing/unsupported artwork, Spotify network disabled, redirect to private/local address, oversized transfer, decompression bomb, track replacement and cancelled result. Record RED with `go test ./apps/desktop/internal/media -run Artwork`.
- [ ] Music reads only current track's artwork descriptor; Spotify uses only the provider-returned HTTPS artwork URL after separate `RemoteArtwork` opt-in. Fetch through a native/Go bounded client, never direct WebView remote image tags. Permit validated public HTTPS destinations only, enforce the same policy across redirects and DNS resolution, no credentials/cookies or local network endpoints.
- [ ] Enforce 5MiB response cap, 10s deadline, maximum 4096×4096 decoded pixels before downsampling to 512px; memory-only LRU 20MiB, track-scoped cancellation, no disk/network lyrics cache. Native descriptor memory allocation precedes Go validation, so record this API limitation and avoid repeated oversized replies.
- [ ] Show provider attribution and disclosure before opt-in: fetching cover art contacts the provider's image host and reveals the connection. Network failure leaves metadata/transport usable. Native decode/URL tests and focused race must pass.

### Task 4: Local LRC provider with real synchronization

**Files:** create `apps/desktop/internal/media/lrc.go`, `lrc_test.go`, `lyrics.go`, `lyrics_test.go`; native explicit import helper `platform/media_lyrics.go`, `darwin_media_lyrics.go/m/h`, unsupported stub; app-private `media-lyrics.json` association metadata outside settings export.

**Interfaces:** pure `ParseLRC(data []byte) ([]Cue, error)` with `Cue{AtMS int64, Text string}`; `ActiveCue(cues []Cue, positionMS int64) int`; `LyricsScope{Provider, TrackID string}`; local provider `ChooseLyrics(ctx, scope) (LyricsDocument,error)` and `LoadLyrics(ctx, scope) (LyricsDocument,error)`. `LyricsDocument` contains opaque document ID, scope, copied cues, status/reason. Chooser accepts a captured scope and never remaps its result onto a later track. No frontend path-string read API.

- [ ] Write parser tests with original test text: `[00:01.00]One`, `[00:03.250][00:05.00]Two`, `[offset:-500]`; expected cue times 500,2750,4500ms. Cover duplicate times, CRLF/BOM, stable ordering, hour-long minutes, invalid/overflow timestamps and plain text with no timestamps. Record RED with `go test ./apps/desktop/internal/media -run 'LRC|Lyrics'`.
- [ ] Implement UTF-8 LRC, multi-time tags, signed global millisecond offset; clamp negative adjusted times to zero; preserve equal-time lines together; ignore known metadata tags and reject files without usable timestamps. Bound file 1MiB, 10,000 cues and each line 4KiB. Do not render HTML. Unsupported enhanced word-level tags are explicit line-level handling, not claimed karaoke accuracy.
- [ ] Use a user-triggered file chooser only, capture exact file identity, balance security scope, store an association/bookmark privately with remove/replace UI. Import copies parsed cues into bounded memory; no folder scans or file watcher, no staging/open-file actions. Re-read only explicit reload or next associated track presentation; stale/revoked bookmark is a visible unavailable state.
- [ ] Implement synchronization from sampled position plus monotonic elapsed time while playing. Stop extrapolation when sample age exceeds 2s, freeze on pause/stopped, clamp to duration, binary-search active cue after every fresh sample/seek. Clear cues immediately on track epoch change; explicit matching by provider+stable track ID only, not title guessing. Fake clock tests must cover pause, resume, backward/forward seek, playback drift correction, sleep/resume and A→B→A reload.
- [ ] Show synchronized current line and surrounding lines; auto-follow stops temporarily while the user scrolls and resumes via a Follow button. Selecting/importing a file is not proof that its timestamps match the recording; expose a bounded per-association timing offset adjustment (±30s) and reset. No timestamped file means `No synchronized lyrics`, not unsynchronized text presented as success.

### Task 5: Dock media view and independently pinned native panels

**Files:** create `apps/desktop/app_media.go/test`, `app_media_window.go/test`, frontend `dock/MediaPanel.tsx/test`, `dock/media.css`, `lib/media.ts/test`; modify App lifecycle, Dock content union/controller targeting, frontend route admission and settings/localization in coordinated ownership. Reuse `dockWindow`/`DockPanelHost`; do not share one retained content view between hosts.

**Interfaces:** App DTO `MediaViewState{PanelID string, Session, Revision uint64, Provider, Status, Reason string, Sample, Lyrics, Pinned bool}`; RPCs `MediaCommand(panelID,session,revision,trackEpoch,kind,positionMS)`, `RequestMediaAccess(panelID,session,revision)`, `ImportMediaLyrics(panelID,session,revision,trackEpoch)`, `PinMedia(panelID,session,revision)`, `CloseMediaPanel(panelID,session)`. Provider/track/process are resolved from captured backend state, not trusted caller strings. The frontend receives opaque artwork only.

- [ ] Write App tests for wrong panel/session/revision, hide before command dispatch, permission owner surviving hover hide but not disable, independent pin close, no capture/input-target/window-action paths for media, and old native host-close callback not retiring a replacement. Record RED with `go test ./apps/desktop -run Media`.
- [ ] Add `media` Dock content only for enabled exact Music/Spotify app items; retain an explicit Windows tab when window previews are enabled, so media does not remove existing app-window functionality. Switching content retires action/capture ownership coherently. Folder content remains separate. Never request permission from the hover path.
- [ ] Pin creates a new hidden Wails route/host and independent session subscribed to the same provider. Initial position uses current screen visible logical bounds; clamp after measured content size. At most one pin per provider; repeated pin raises the existing panel without activating it. Close/hide removes only that subscription. App shutdown closes all hosts; session inactivity hides pins and suspends observations, restoring only still-enabled pins on resume. Player exit leaves a truthful not-running pin without launching it.
- [ ] Support explicit panel-header drag, routed only to its own native panel (no global drag tap), and close/unpin. Persist neither live sessions nor automatic startup pin windows. Keep position per live pin by display UUID; display disconnect moves it into main display's visible bounds, never offscreen. Topology changes cancel pending pointer/seek ownership; content resize remains bounded and does not reset user placement. Avoid stealing key focus; import/permission dialogs are the explicit exceptions.
- [ ] Build artwork/title/artist/provider attribution, play/pause/previous/next, lyric import/follow/offset, pin/close and seek slider when advertised. Errors update both rendered revision and event admission; stale events cannot resurrect panels. Expose per-provider permission status and explanation in settings. Light/dark/system theme, reduced motion, text scaling and keyboard-accessible focused controls follow existing appearance policy; do not install global media keys.
- [ ] Browser tests exercise real controls after error revision, track change during slider drag, pause lyric freezing, old queued event after close, network-disabled artwork, and bounded overflowing lyrics. Run focused tests, TypeScript and race checks; regenerate bindings only after RPC freeze under coordinator control.

### Task 6: Acceptance and release gate

**Files:** retain `apps/desktop/testdata/media_smoke/` source/runner/README and a milestone report under `.superpowers/sdd/2026-09-06-dock-media/`. Fixtures use unique bundle IDs and original generated metadata/artwork/LRC; production provider allowlist cannot be widened through an RPC. Test injection is an isolated binary build seam only.

- [ ] Run signed disposable AppleEvent player fixture: verify exact command opcodes/position units, no autolaunch on exit, provider isolation, finite event timeout, refusal after final guard retirement, and no foreground change. Record physical native panel button proof separately from DOM invocation.
- [ ] Exercise actual Wails media hover and two pins while another disposable text app remains foreground: controls dispatch to the intended fixture; close one pin leaves the other; outside clicks pass through; rapid hide/reopen and host destruction cannot target a replacement. Verify no capture streams are opened for media and zero media polling after last panel closes.
- [ ] With fixture-generated LRC, observe line transitions against fixture playback clock, pause freeze, seek backward/forward, track replacement and timestamp offset. Report measured sampling/jitter; do not claim word-perfect sync. Test offline artwork and missing/revoked local file without prompting automatically.
- [ ] Run native geometry seam tests for secondary/Retina display and disconnect. Real physical display detach and real TCC denial/revocation are separate coordinated acceptance cases; mark unexecuted cases pending rather than passing from mocks.
- [ ] Only when explicitly coordinated, verify each installed real provider's metadata, measured duration/position units, available transport/seek and permission grant/denial/revocation. User playback changes are not an automatic acceptance action. Streaming/live/ad restrictions remain explicit per-provider capability failures.
- [ ] Run `go test -race ./apps/desktop/internal/media ./apps/desktop/internal/platform ./apps/desktop`, focused browser tests, existing repository lint/typecheck/build/full regression gate. Inspect lifecycle locks and unlicensed asset/network dependencies independently. Do not mark E04–E06 native acceptance complete from unit seams alone.

## Coverage and remaining decisions

E04 maps to Tasks 1–3 and 5; E05 maps to Task 4's clocked timeline plus native/UI verification, not plain text display; E06 maps to Task 5's independent host sessions and Task 6. H12 seek is supported where verified; output-device selection remains a separate retained implementation. No other retained roadmap item is dropped.

No user decision blocks this implementation plan: providers and remote artwork default off, local lyric import is explicit, pins are session-only. Automatic online lyric coverage is intentionally absent because no licensed service contract has been supplied; adding one requires choosing a provider and approving its actual terms/configuration separately. Real Spotify/Music feature acceptance and Automation prompts need a coordinated user-visible test window, not guessed consent. The installed duration-unit discrepancy and non-atomic track/send boundary must be reported honestly in acceptance evidence.

# Inert widget packages

This package validates and stores local declarative ZIPs. It has no runtime, native provider, enabled state, grant store, network client, chooser or command dispatcher. Author package IDs do not establish trust. Minimum app versions are validated metadata; compatibility admission belongs to the future host.

```go
manifest, err := widgets.ParseManifest(rawJSON)
preview, err := widgets.Preview(ctx, selectedReader) // no files written
store, err := widgets.OpenStore(appOwnedDirectory)
defer store.Close()
installed, err := store.Install(ctx, selectedReader)
copy, err := store.Get(installed.Digest())
metadata := copy.Manifest() // deep copy
pngBytes, err := copy.Asset("icon") // copied bytes by declared opaque asset ID
```

Handle errors in production. `Store.Close` is idempotent. The caller owns and closes supplied readers. Cancellation is checked between bounded reads and before publication; an arbitrary reader blocked inside Read cannot be interrupted by this package. Publication linearizes at the final context check/atomic directory rename. Installation grants nothing, including when an existing author ID requests more capabilities. Package handles remain usable after store closure because they own verified immutable bytes.

`testdata/clock/widget.json` is a valid original example. Put it at ZIP root as `widget.json`; declared assets use portable ASCII `assets/name.png` paths. Explicit directory entries, links, executable modes, alternate Unicode-name/link vendor metadata, undeclared payloads and Windows reserved names are rejected. Only the ordinary extended timestamp ZIP extra is allowed. No ZIP entry is extracted to its name: the app-private store saves one verified `package.zip` inside a digest directory, using a private 0700 staging directory, 0600 file and atomic rename. `Get` revalidates bytes and content digest. Store methods use an `os.Root` directory handle; the app must own the root and protect it from unrelated writers. This is not a sandbox against arbitrary same-UID filesystem manipulation or mount changes.

Limits: 4 MiB archive, 8 MiB expanded, 64 entries, 256 KiB manifest, 512 KiB/4 MP per PNG; tree depth 8, 128 nodes, 16 children; history 1–120; 16 settings. Names/descriptions require `en`, allow `pt-BR` and `es`; 80/1024 characters. IDs are lowercase ASCII, versions strict semantic versions. Unknown/duplicate keys, nulls and unsupported authority combinations fail validation.

## Typed schema

Manifest: `schemaVersion`, `id`, `version`, `minimumAppVersion`, `name`, `description`, `requiredCapabilities`, `optionalCapabilities`, `settings`, `root`, `assets`. Empty optional arrays may be omitted. No grants or enabled fields.

Settings: `id`, `type`, localized `name`, plus exactly the applicable values. `boolean` uses `defaultBool`; `number` uses finite `min`/`max`/`defaultNumber` within ±1e9; `choice` uses 1–16 unique `options` and `defaultText`; `timezone` uses validated IANA `defaultText` (or UTC/Local). Only a clock binding may reference an existing timezone setting.

Nodes: `row`/`column` contain children; `text` has literal text OR one binding; `icon` references a declared asset ID; `progress`/`sparkline` use numeric bindings (`sparkline` adds history); `button` has literal text and a fixed command. Irrelevant combinations are refused. Assets declare `id`, `path`, lowercase hex `sha256`.

Bindings are `{provider, field, formatter, timezoneSetting?}`. Fixed catalog:

| Provider | Fields | Required capability | Formatter |
| --- | --- | --- | --- |
| clock | time | clock.read | shortTime, longTime, date |
| battery | charge; charging; powerSource | battery.read | percent/number; boolean; text |
| network | connected; category | network.status.read | boolean; text |
| network | uploadRate, downloadRate | network.usage.read | bytesPerSecond/number |
| audio | outputName; volume; muted | audio.status.read | text; percent/number; boolean |
| music / spotify | title, artist, album, playback; position, duration | media.music.read / media.spotify.read | text; duration/number |

Commands are `{provider, action}` only. `audio/selectOutput` requires declared `audio.output.select`; `music`/`spotify` support `play`, `pause`, `playPause`, `next`, `previous`, `seek`, requiring the corresponding `media.<provider>.control`. These are declarations, not executable calls or method names. A future trusted host supplies explicit current target/value and full admission scope, and must grant declared authority before use. Packages cannot provide arbitrary target IDs, script text, paths, URLs, expressions or arguments.

Required/optional catalogs contain only the capabilities listed above. Optional declaration is not a grant; missing optional data must remain unavailable in future runtime integration.

## Shared runtime (host integration prerequisite)

`NewRuntime(Deps)` creates a pure owner. `Configure([]Request) error` validates/copies at most 32 visible instance requests; hidden stack instances are omitted. Each request supplies a verified Package, parent controller epoch/display UUID/session/profile ID, instance ID, enabled flag, explicit declared grants, and setting overrides. `ValidateSettings(Manifest,map[string]Value)` returns independent validated defaults/overrides. Unknown settings/grants are refused. A disabled or denied-required instance has no subscriptions/actions. Unused declarations also cause no provider subscription. Allowed optional nodes still render when another optional node is unavailable (`partial` container status).

Run `Run(ctx) error` once; `Snapshot() []InstanceState` returns copies. Configure synchronously replaces changed admission before the owner loop processes callbacks, so revoke→grant/A→B→A cannot revive an old instance. `Deps.Changed` runs only on Run, outside locks; callbacks must return promptly. The host owns a thread-safe `Deps.Now` function (default time.Now). Provider callbacks only validate/copy/coalesce. Five typed slots exist: Battery, Network, Audio, Music, Spotify. A slot implements `Observe(ctx,wantedCapabilities,emit)`, optionally `Perform(ctx,ProviderAction,guard)`. No runtime dispatch accepts package-selected arbitrary source names. One owner is shared for each requested capability union. Replacement waits for Observe and admitted actions to finish after cancellation; last consumer stops the owner. Unexpected source completion retries at most once/second. Sources must honor context and join their work on Observe return; blocked native calls cannot be forcibly terminated by this pure package.

Provider samples use **provider-local** field keys (`charge`, `outputName`, `position`, etc), typed optional Value pointers and fixed action-name keys (`selectOutput`, `next`, `seek`, etc). Media position/duration and seek values are milliseconds. Samples need nonzero monotonic generation/sequence, observation time, and explicit status. The runtime validates/filters field ownership before storage; control-only audio cannot render status. Initial stale/future samples are refused; an owned observation holds its last valid sample until update, refusal or cancellation rather than inventing heartbeat timestamps. Sources must represent track/device identity replacement by generation changes. Host-issued option IDs and numeric ranges stay backend-only; the renderer sees labels, opaque tokens and copied finite ranges.

`Lease` includes parent scope, digest, runtime admission epoch and content revision. `Asset(lease,assetToken)` requires the exact snapshot and returns verified copied bytes. `ActionOptions(ctx,lease,actionToken)` and `Perform(ctx,lease,actionToken,optionToken,*value,finalGuard)` use action authority, so an opened chooser/started action survives unrelated content revisions. The original lease must match every lifetime field; revision must be between the token's first issued revision and current revision. Tokens bind provider owner, generation and a monotonic authority serial updated on every observed ActionSpec/generation/status change **before coalescing**. Option/spec A→B→A cannot revive a token. Current option/range authority, grants and lifetime are checked before preparation, after the App guard and at the provider's final guard. One explicit action runs at a time; another refuses busy. Its context is linked to provider retirement. Progress samples continue reducing while actions prepare. Providers must invoke the supplied guard immediately before native mutation; the runtime itself performs no native command.

Resolved RenderNode contains only key/kind/status, plain text, normalized finite progress or nil, bounded history, opaque asset/action tokens and children. Never insert Text as HTML; trusted React text rendering escapes it. No Binding/Command/native option IDs/provider paths are sent in render nodes. Histories are capped at the manifest's limit (maximum120), copied and reset across provider generations/owners. There is one shared host clock ticker, at most1Hz, only while a granted visible clock binding needs it.

`Builtin(id)` and `Builtins()` return immutable verified packages with IDs `org.optiontab.clock`, `.battery`, `.network`, `.audio`. Built-in manifests/ZIP bytes/digests are deterministic and tested. Clock exposes time format/timezone, Battery charge/charging/source, Network required local status plus optional usage, and Audio required status plus optional output selection. Root/App owns migration of the earlier compiled-clock reference and must never transfer community grants by matching package ID alone.

Binding additionally permits `formatterSetting`, referencing a choice setting whose every option is a fixed valid formatter for that field. This remains a typed enum selection; expressions, format code, wrong-field formatters and unknown setting references fail validation. Existing fixed-formatter manifests are unchanged.

## Installed catalog and removal

`Store.List(ctx) ([]*Package, error)` returns verified immutable packages sorted by digest. It revalidates every archive as `Get` does. Invalid names, symlinks, corrupt archives and digest mismatches are omitted and reported: a non-nil error means the returned catalog may be partial. Only genuine `.stage-<32 lowercase hex>` directories are ignored. Enumeration is bounded to 128 root entries (including abandoned stages) and 64 digest directories; exceeding either refuses the catalog. The app owns cleanup of abandoned installation stages.

`Install` admits at most 64 distinct digest directories; reinstalling an already verified digest remains idempotent at capacity. Serialization and capacity admission apply to one Store owner; the app must use one owner for its private directory, not competing independent Store instances/processes. Invalid catalog entries block new installs until repaired. Installation still grants no capabilities.

`Store.Remove(ctx, digest) error` accepts only the exact lowercase SHA-256 digest directory under its root, refusing symlink/non-directory targets. It can remove a corrupted digest directory for recovery; it does not require a still-valid archive. An absent target returns `os.ErrNotExist`. The App must retire dependent widget instances/actions and drop active references before removal; previously returned immutable Package values retain their owned bytes. Nested links are removed without following them. Cancellation is checked before filesystem admission and again before deletion; admitted filesystem operations are synchronous and cannot promise bounded OS latency or rollback after cancellation. List checks cancellation before/after each bounded archive verification. Nil contexts refuse; closed stores return `os.ErrClosed`.

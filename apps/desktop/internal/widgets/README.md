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

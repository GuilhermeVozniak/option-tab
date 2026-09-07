# Independent widget services implementation

Continue retained H12/H13/H14 alongside launcher profile integration. Worktree `.worktrees/widget-services`, branch `feat/widget-services`, base `4d0dc9c`. These components are prerequisites, not a complete user-visible widget feature. Root integrates them after the current profiles checkpoint. No release, GUI, input, chooser or audio-device action during tests.

The approved `replacement-dock-widgets` specification remains authoritative. No executable widget code, arbitrary provider dispatch, weather/calendar, file staging or saved-command integration. Installation is inert and grants nothing.

## Native audio port — native worker

Own new platform audio_output*.go and darwin_audio_output.{go,m,h} plus native testdata. Avoid edits to unrelated platform behavior. Shared contract:

```go
type AudioOutputDevice struct {
    UID, Name string
    Alive bool
    OutputChannels int
}
type AudioOutputSnapshot struct {
    Generation, Sequence uint64
    ObservedAt time.Time
    Status, Reason string
    Devices []AudioOutputDevice // maximum 64, copies
    DefaultUID string
    Volume *float64 // finite 0..1 or nil (unsupported)
    Muted *bool
}
type AudioOutputSource interface {
    ObserveAudioOutputs(context.Context, func(AudioOutputSnapshot)) error
    SelectAudioOutput(context.Context, uint64, string, func() error) error
}
```

Use public CoreAudio HAL only. Enumerate live output-capable devices with stable UID; numeric device IDs remain backend-only and are resolved freshly. Observe devices/default output and selected-device alive/volume/mute with bounded coalescing. Listeners do not invoke Go/user callbacks; the owned worker delivers copies. Cancel removes listeners and joins queued work before return; old generations cannot select after retirement. Unsupported properties are nil/unavailable, never invented zero readings. No audio I/O streams, microphone access or permission prompts.

Selection is an explicit guarded action against a currently observed source generation and UID. Revalidate UID→ID→UID, alive/output channels, settable default output and final Go guard immediately before setting only DefaultOutputDevice; leave system-alert/input device and sample rate unchanged. Read back the selected UID before reporting success. No automatic fallback or action from observation. Use injected HAL/native seams for hostile races and errors; do not switch a real device.

## Manifest and inert local package store — pure worker

Own new `internal/widgets` package only. This task covers validated immutable manifests/packages and bounded local ZIP installation, not the live widget runtime or App chooser. Constructor takes an app-owned directory. Public API may use `ParseManifest([]byte)`, `OpenStore(root)`, `Preview(ctx, io.Reader)`, `Install(ctx, io.Reader)`, `Get(digest)`, and an opaque package/asset reader; prefer small concrete types and document final names. Root will add explicit native chooser and lifecycle wiring later.

Manifest schemaVersion1 contains package ID, semantic version, minimum app version, localized name/description (EN required; optional pt-BR/es), required/optional capability lists, bounded declared user settings, one root layout tree and declared PNG assets with SHA256. Use the fixed capability catalog in the approved spec. Layout row/column/text/icon/progress/sparkline/button; depth8/nodes128/children16/text bounds/history120. Direct typed provider fields and fixed formatters only; no expressions, paths into application state, raw CSS/HTML/JS, method names or remote resources. A button specifies a supported fixed audio/media command enum, and the corresponding control capability must be declared. Stacks are a host concern and not recursive manifests.

Reject unknown/duplicate fields/keys, unknown capability/field/formatter/command combinations, malformed locales/settings/versions/IDs, missing declared assets or extra undeclared payloads. Package identity is content-digest-bound; matching author package ID conveys no trust. Installation creates no enabled instance or grant.

ZIP limits: compressed4MiB, expanded8MiB, entries64, manifest256KiB, each PNG512KiB encoded and4MP decoded. Bound reads before allocation. Reject traversal/absolute paths, links, path collisions (including case-fold), ambiguous names and expansion beyond limits; allow only widget.json and explicitly declared assets. Restrict asset archive names to portable ASCII components to avoid Unicode path ambiguity. Validate PNG dimensions before full decode. Verify asset digests. Publish verified immutable content under an app-owned digest directory via private staging + atomic rename; cancellation/error removes staging. Never follow archive links or use package-provided destinations. Preview makes no permanent install or grants. Exported package values/read bytes must not alias mutable internal state.

Meaningful tests cover valid package round-trip, digest identity, no grants, duplicate/unknown authority, typed layout/control compatibility, traversal/link/collision/bomb/oversized image/undeclared asset rejection, cancellation cleanup and immutable snapshots. Use owned temporary directories only, no user files/network/process/native permissions. Focused race/lint before release. Root owns git and integration; do not commit or broaden this into live runtime/UI.

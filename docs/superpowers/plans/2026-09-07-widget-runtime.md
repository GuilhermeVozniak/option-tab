# Shared widget runtime and local status providers

Continue H13/H14 in isolated widget-runtime worktree, base d176069, alongside H03 integration. This implements the live engine and battery/network prerequisites. Root subsequently wires App, grants/configuration, installer review, renderer/stacks and existing media owners. Do not claim the complete widget feature from these components alone. Approved widget spec and package limits remain authoritative. No release or native device/desktop action during tests.

## Native status sources — native worker

Own new platform/system_widget_status*.go, darwin_system_widget_status.{go,m,h}, tests/testdata. Add these concrete ports (no changes to audio/launcher):

```go
type BatterySnapshot struct {
    Generation, Sequence uint64
    ObservedAt time.Time
    Status, Reason string
    Charge *float64 // 0..1, nil when absent/unavailable
    Charging *bool
    PowerSource string // battery|external|unknown
}
type NetworkSnapshot struct {
    Generation, Sequence uint64
    ObservedAt time.Time
    Status, Reason string
    Connected *bool
    Category string // none|wifi|ethernet|vpn|other|unknown
    UploadRate, DownloadRate *float64 // bytes/sec, nil unless usage requested + valid deltas
}
type BatterySource interface { ObserveBattery(context.Context, func(BatterySnapshot)) error }
type NetworkSource interface { ObserveNetwork(context.Context, bool /*usage*/, func(NetworkSnapshot)) error }
// Constructors NewBatterySource() / NewNetworkSource(), explicit unsupported !darwin.
```

Use public IOKit power-source and SystemConfiguration/getifaddrs local status/statistics only. No network requests, addresses/SSID/packet contents in snapshots, no device identity dump, no microphone or permissions. Connected describes an active local default route/link, not verified Internet access. Battery absent on desktops is explicit, not zero charge. Battery polling at most once/5s; network status at most once/2s; separately requested usage at most1Hz. Coalesced notification adapters are optional; bounded polling is acceptable. Usage=false does not read byte counters. Usage rates require a previous valid monotonic sample on the same interface identity; first sample, interface change, counter reset/wrap or invalid elapsed time yields nil, not invented traffic. Keep interface identifiers backend-only.

Each source owns and joins its reads/timer before return, copies optional values, rejects nil context/callback and drops results after cancellation. Native calls already entered may outlive cancellation; do not promise a hard OS deadline. Inject every native data read in tests: absent/malformed battery, route/link categories, unsupported counters, interface change/reset, rate bounds, usage-off silence and cancellation. Do not query real power/network state or alter user devices during automated tests.

## Pure shared runtime — pure worker

Own new internal/widgets runtime/types/builtins/tests/README only. Do not import platform, config or App packages: define small typed host-owned provider interfaces here so configuration may later depend on validated widget types without a cycle. Manifest/package validation stays the trust boundary. No raw scripts, package functions or arbitrary provider names can enter runtime dispatch.

Freeze a concrete API around `NewRuntime(Deps)`, `Configure([]Request) error`, `Run(ctx) error`, `Snapshot() []InstanceState`, `ActionOptions(ctx, Lease, actionToken)`, `Perform(ctx, Lease, actionToken, optionToken, value, finalGuard) error`, and `Asset(Lease, assetToken) ([]byte,error)`. Final exported names/fields must be reported before App/UI wiring. A request contains a verified immutable Package, parent controller epoch/display UUID/session/profile ID, stable instance ID, explicit enabled flag/grants, and validated manifest setting values. Request order defines host presentation order; hidden stack members are omitted by App. No native handle or provider data is persisted.

Lease contains controller epoch, display UUID/session, profile ID, instance ID, digest, runtime admission epoch and monotonically increasing instance revision. Configure retires removed/changed admission synchronously before queuing subscription changes; coalescing cannot erase A→B→A or revoke→grant. Parent presentation content revisions are not subscription identities; parent controller epoch/session and exact App final guard protect that boundary. Samples/actions cannot cross digest, instance, profile or display lifetimes. Snapshot and all returned values are independent copies. No locks across source calls, final guards or View callbacks. Run is terminal on cancellation; native/host sources are joined before replacement.

Deps has a typed Providers struct with Battery, Network, Audio, Music and Spotify slots, never a package-selected generic native dispatcher. Slots may implement one common trusted host interface with Observe(ctx, wantedCapabilities, emit), but dispatch only through the closed catalog. Samples contain generation/sequence, timestamp/status/reason, fixed catalog field values and host-owned action options/range metadata. Validate fields/types/finite values and capability ownership before rendering. Missing providers/unsupported fields remain unavailable, never zero. Audio control-only grants may enumerate output choices for that fixed control but cannot render undeclared status fields. Source callbacks copy/coalesce only.

Share one owner per needed provider across all active requests. Reconfigure wanted capabilities by cancelling/joining the previous owner before replacement; disabling/removing one consumer leaves other consumers live. Last eligible request stops the provider. One host clock ticker at most1Hz exists only while granted visible clock bindings need it. Data reduction continues while one serial explicit action prepares; concurrent actions refuse busy. A final action guard must check current runtime lease/grants/provider generation after external preparation and invoke the App guard outside locks. Option tokens refer only to current host-issued options; the package cannot provide target IDs or arguments. No action from Configure, sampling, retries or rendering.

Resolve validated row/column/text/icon/progress/sparkline/button trees into bounded trusted RenderNode values: stable node key, escaped text, finite normalized progress or nil, <=120 numeric history values, opaque asset token and opaque enabled action token. Do not send Binding/Command/provider paths/native identifiers to React. Formatter behavior follows the fixed catalog; validate/copy settings against manifest defaults/ranges/choices/timezones. Denied required grants produce grantRequired and no subscriptions/actions. Denied optional fields produce explicit unavailable nodes while permitted nodes remain usable. Asset access is bound to the active instance/package lease and returns only its verified PNG bytes.

Provide immutable compiled built-in packages for Clock, Battery, Network and Audio using the same manifest/runtime/grants. Their deterministic ZIP bytes/digests must not vary with clock or build metadata. Clock has format/timezone, battery shows charge/charging/source, network separates status and optional usage, audio has status and a fixed choose-output command. Built-in integration/migration of the previous compiled clock reference is root-owned; never inherit grants between arbitrary community digests.

Meaningful tests: no grants/no source work, required vs optional denial, two displays share one provider, last-hide join, blocked source revoke→grant, A→B→A stale data/action rejection, immutable snapshots/assets, one clock timer, bounded histories and data, action busy, option-generation changes, revocation during blocked native preparation, no command/capability spoofing, setting validation and fixed built-in package digests. Use manual clocks/fakes where useful, no native or network action. Focused race/lint, API handoff and bounded report before release. Root owns integration/git/bindings.

## Root integration sequence

First integrate the typed native ports through a small `internal/widgetproviders` package. Provider-local field keys and fixed action names match the runtime catalog. Each adapter admits only capabilities for its own fixed slot, copies optional readings, preserves status/generation/sequence, omits ungranted fields, and drops results after cancellation. The network adapter passes usage=true only for the separate usage grant. Audio control-only requests enumerate host-owned choices without publishing status fields; selection passes the exact generation/UID and runtime guard to the native source. Tests use injected ports and exercise grant separation, absent values, copies, cancellation and guarded identity selection. These adapters do not create owners by themselves.

Then wire one App widget runtime to the admitted replacement-Dock presentations. Runtime requests use parent epoch/display/session/profile identity, not the presentation's clock/content revision. Reconfiguration and native recovery retire widget admission synchronously; provider joins remain outside App locks. Runtime events are accepted only for current presentations, and every asset/action RPC checks both the exact widget lease and parent host at admission/final dispatch. Existing keyboard and independent media-pin owners remain unaffected.

Extend profile widget configuration with bounded typed manifest settings and host-owned stacks. Packages remain identified by immutable digest; only the exact previously compiled clock reference may migrate to the new compiled clock. Keep explicit enabled state/grants for that recognized built-in migration, never copy grants across arbitrary community digests. Validate package settings against the verified manifest before admitting a request. Unknown/missing packages stay unavailable and perform no provider work. Stack selection omits hidden members from runtime requests and retires their pending actions immediately.

Add a preferences catalog/instance editor and explicit local package review/install flow after the live renderer is integrated. Review uses a bounded immutable archive snapshot and an expiring host token; installation consumes that reviewed content and is inert. Enabling an instance and granting its declared capabilities are separate settings actions. Installed package removal retires referencing instances before removing private stored bytes. Existing media reads/controls attach through the shared media controller with both app-level enablement and widget grants; no polling path requests permission.

This sequence is implementation of the approved H scope, not a new release. Component checkpoints must continue to list the remaining App/UI/install/stack/native-acceptance work until it is integrated.

package widgets

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func presentationKey(t *testing.T, node RenderNode) string {
	t.Helper()
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		TextKey string `json:"textKey"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.TextKey
}

func TestRuntimeBuiltinValuePresentationRequiresExactPackage(t *testing.T) {
	yes, no, charge := true, false, .75
	for _, tc := range []struct {
		provider, field string
		value           Value
		child           int
		want            string
	}{
		{"battery", "charging", Value{Boolean: &yes}, 1, "battery.charging"},
		{"battery", "charging", Value{Boolean: &no}, 1, "battery.notCharging"},
		{"battery", "powerSource", Value{Text: builtinText("battery")}, 2, "battery.power"},
		{"battery", "powerSource", Value{Text: builtinText("external")}, 2, "battery.external"},
		{"battery", "powerSource", Value{Text: builtinText("unknown")}, 2, "battery.unknown"},
		{"network", "connected", Value{Boolean: &yes}, 0, "network.connected"},
		{"network", "connected", Value{Boolean: &no}, 0, "network.disconnected"},
		{"network", "category", Value{Text: builtinText("none")}, 1, "network.none"},
		{"network", "category", Value{Text: builtinText("wifi")}, 1, "network.wifi"},
		{"network", "category", Value{Text: builtinText("ethernet")}, 1, "network.ethernet"},
		{"network", "category", Value{Text: builtinText("vpn")}, 1, "network.vpn"},
		{"network", "category", Value{Text: builtinText("other")}, 1, "network.other"},
		{"network", "category", Value{Text: builtinText("unknown")}, 1, "network.unknown"},
		{"audio", "muted", Value{Boolean: &yes}, 2, "audio.muted"},
		{"audio", "muted", Value{Boolean: &no}, 2, "audio.notMuted"},
		{"audio", "outputName", Value{Text: builtinText("Connected")}, 0, ""},
		{"battery", "charge", Value{Number: &charge}, 0, ""},
	} {
		t.Run(tc.provider+"/"+tc.want+"/"+tc.field, func(t *testing.T) {
			builtin, _ := Builtin("org.optiontab." + tc.provider)
			manifest := builtin.Manifest()
			original, _ := json.Marshal(manifest)
			manifest.Name["en"] = "Community literal"
			community, err := Preview(context.Background(), bytes.NewReader(testZIP(t, manifest, nil)))
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []*Package{builtin, community} {
				r := NewRuntime(Deps{})
				q := runtimeRequest(p)
				q.Grants = p.Manifest().RequiredCapabilities
				beforeGrants := append([]string{}, q.Grants...)
				if err := r.Configure([]Request{q}); err != nil {
					t.Fatal(err)
				}
				r.mu.Lock()
				r.owners[tc.provider] = &providerOwner{ctx: context.Background(), sample: Sample{Generation: 1, Sequence: 1, Status: "ready", Fields: map[string]Value{tc.field: tc.value}}}
				r.resolveLocked(r.instances[0])
				r.mu.Unlock()
				state := r.Snapshot()[0]
				node := state.Root.Children[tc.child]
				want := ""
				if p == builtin {
					want = tc.want
				}
				if got := presentationKey(t, node); got != want {
					t.Errorf("builtin=%v: got %q want %q", p == builtin, got, want)
				}
				if node.Status != "ready" {
					t.Fatal("valid reading became unavailable")
				}
				if !reflect.DeepEqual(q.Grants, beforeGrants) {
					t.Fatal("presentation mutated grants")
				}
				// Missing data must never gain a translated false/zero reading.
				r.mu.Lock()
				r.owners[tc.provider].sample.Fields = nil
				r.resolveLocked(r.instances[0])
				r.mu.Unlock()
				node = r.Snapshot()[0].Root.Children[tc.child]
				if node.Status != "unavailable" || presentationKey(t, node) != "" || node.Text != "" {
					t.Fatal("missing reading was invented")
				}
			}
			after, _ := json.Marshal(builtin.Manifest())
			if !bytes.Equal(original, after) {
				t.Fatal("immutable built-in manifest changed")
			}
		})
	}
}

func TestRuntimeBuiltinActionPresentationKeepsAuthority(t *testing.T) {
	p, _ := Builtin("org.optiontab.audio")
	provider := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 2), perform: func(_ context.Context, a ProviderAction, guard func() error) error {
		if a.OptionID != "device" {
			t.Errorf("wrong selected UID %q", a.OptionID)
		}
		return guard()
	}}
	r := NewRuntime(Deps{Providers: Providers{Audio: provider}})
	q := runtimeRequest(p)
	q.Grants = []string{"audio.status.read", "audio.output.select"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, provider.runs)
	t.Cleanup(func() {
		cancel()
		close(run.release)
		if err := runtimeReceive(t, done); err != context.Canceled {
			t.Error(err)
		}
	})
	run.emit(Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "device", Label: "Audio output", LabelKey: "audio.output"}, {ID: "named-device", Label: "Audio output"}}}}})
	first := waitRuntime(t, r, func(s InstanceState) bool { return len(s.Root.Children) == 4 && s.Root.Children[3].ActionToken != "" })
	button := first.Root.Children[3]
	if got := presentationKey(t, button); got != "audio.chooseOutput" {
		t.Errorf("button key %q", got)
	}
	options, err := r.ActionOptions(ctx, first.Lease, button.ActionToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Options) != 2 || options.Options[0].LabelKey != "audio.output" || options.Options[1].LabelKey != "" {
		t.Fatalf("option provenance lost: %+v", options)
	}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	current := r.Snapshot()[0]
	if current.Lease != first.Lease || current.Root.Children[3].ActionToken != button.ActionToken {
		t.Fatal("unchanged presentation retired action")
	}
	if err := r.Perform(ctx, first.Lease, button.ActionToken, options.Options[0].Token, nil, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-provider.runs:
		t.Fatal("presentation restarted provider")
	default:
	}
}

func TestRuntimeRejectsUnknownOptionPresentationKeys(t *testing.T) {
	now := time.Now()
	for _, option := range []ProviderOption{
		{ID: "device", Label: "Audio output", LabelKey: "audio.chooseOutput"},
		{ID: "device", Label: "Speakers", LabelKey: "audio.output"},
	} {
		sample := sanitizeSample("audio", []string{"audio.output.select"}, Sample{Generation: 1, Sequence: 1, ObservedAt: now, Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{option}}}}, now)
		if sample.Status != "unavailable" || len(sample.Actions) != 0 {
			t.Fatalf("invalid option presentation admitted: %+v", sample)
		}
	}
}

func TestRuntimeCommunityButtonTextNeverGainsBuiltinPresentation(t *testing.T) {
	builtin, _ := Builtin("org.optiontab.audio")
	m := builtin.Manifest()
	m.Name["en"] = "Community audio"
	p, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, nil)))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRuntime(Deps{})
	q := runtimeRequest(p)
	q.Grants = []string{"audio.status.read"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	r.resolveLocked(r.instances[0])
	r.mu.Unlock()
	button := r.Snapshot()[0].Root.Children[3]
	if button.Text != "Choose output" || button.TextKey != "" {
		t.Fatalf("community button translated by name: %+v", button)
	}
}

func TestRuntimePartialAudioPresentationWireOmitsEmptyPayloads(t *testing.T) {
	p, _ := Builtin("org.optiontab.audio")
	r := NewRuntime(Deps{})
	q := runtimeRequest(p)
	q.Grants = []string{"audio.status.read"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	muted := false
	now := time.Now()
	sample := sanitizeSample("audio", q.Grants, Sample{
		Generation: 1, Sequence: 1, ObservedAt: now, Status: "ready",
		Fields: map[string]Value{"outputName": {Text: builtinText("")}, "muted": {Boolean: &muted}},
	}, now)
	r.mu.Lock()
	r.owners["audio"] = &providerOwner{ctx: context.Background(), sample: sample}
	r.resolveLocked(r.instances[0])
	r.mu.Unlock()
	raw, err := json.Marshal(r.Snapshot()[0])
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Status string
		Root   struct{ Children []map[string]any }
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Status != "partial" || len(wire.Root.Children) != 4 {
		t.Fatalf("unexpected partial audio wire: %s", raw)
	}
	name, volume, mute, button := wire.Root.Children[0], wire.Root.Children[1], wire.Root.Children[2], wire.Root.Children[3]
	if name["status"] != "ready" || name["text"] != nil || name["textKey"] != nil {
		t.Fatalf("empty device name wire: %+v", name)
	}
	if volume["status"] != "unavailable" || volume["text"] != nil || volume["textKey"] != nil {
		t.Fatalf("absent reading invented a value: %+v", volume)
	}
	if mute["status"] != "ready" || mute["text"] != "false" || mute["textKey"] != "audio.notMuted" {
		t.Fatalf("admitted mute presentation missing: %+v", mute)
	}
	if button["status"] != "unavailable" || button["text"] != "Choose output" || button["textKey"] != "audio.chooseOutput" || button["actionToken"] != nil {
		t.Fatalf("ungranted action wire: %+v", button)
	}
}

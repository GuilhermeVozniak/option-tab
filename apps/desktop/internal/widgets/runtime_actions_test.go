package widgets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"
)

func actionPackage(t *testing.T) *Package {
	t.Helper()
	m := testManifest()
	m.RequiredCapabilities = []string{"audio.output.select"}
	m.Root = Node{Kind: "button", Text: "Output", Command: &Command{Provider: "audio", Action: "selectOutput"}}
	p, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func waitRuntime(t *testing.T, r *Runtime, predicate func(InstanceState) bool) InstanceState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := r.Snapshot()
		if len(s) > 0 && predicate(s[0]) {
			return s[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("runtime state timeout")
	return InstanceState{}
}

func TestRuntimeActionBusyGenerationAndRevocation(t *testing.T) {
	entered := make(chan ProviderAction, 4)
	release := make(chan struct{})
	p := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 4)}
	p.perform = func(_ context.Context, a ProviderAction, g func() error) error { entered <- a; <-release; return g() }
	r := NewRuntime(Deps{Providers: Providers{Audio: p}})
	q := runtimeRequest(actionPackage(t))
	q.Grants = []string{"audio.output.select"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, p.runs)
	run.emit(Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "secret-native-uid", Label: "Speakers"}}}}})
	state := waitRuntime(t, r, func(s InstanceState) bool { return s.Root.ActionToken != "" })
	options, err := r.ActionOptions(ctx, state.Lease, state.Root.ActionToken)
	if err != nil || len(options.Options) != 1 || options.Options[0].Token == "secret-native-uid" {
		t.Fatal("option token boundary", err)
	}
	actionDone := make(chan error, 1)
	go func() {
		actionDone <- r.Perform(ctx, state.Lease, state.Root.ActionToken, options.Options[0].Token, nil, func() error { return nil })
	}()
	sent := runtimeReceive(t, entered)
	if sent.OptionID != "secret-native-uid" || sent.Generation != 1 {
		t.Fatal("wrong exact host option")
	}
	if err = r.Perform(ctx, state.Lease, state.Root.ActionToken, options.Options[0].Token, nil, func() error { return nil }); err != ErrBusy {
		t.Fatalf("parallel action %v", err)
	}
	// Sample reduction must proceed while the provider action is blocked.
	run.emit(Sample{Generation: 2, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "new-native-uid", Label: "Headphones"}}}}})
	fresh := waitRuntime(t, r, func(s InstanceState) bool { return s.Lease.Revision > state.Lease.Revision })
	if fresh.Root.ActionToken == state.Root.ActionToken {
		t.Fatal("generation retained option authority")
	}
	close(release)
	if err = runtimeReceive(t, actionDone); err != ErrRetired {
		t.Fatal("prepared old generation action survived", err)
	}
	if _, err = r.ActionOptions(ctx, state.Lease, state.Root.ActionToken); err != ErrRetired {
		t.Fatal("stale options admitted")
	}
	q.Enabled = false
	if err = r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ActionOptions(ctx, fresh.Lease, fresh.Root.ActionToken); err != ErrRetired {
		t.Fatal("disable retained action")
	}
	cancel()
	close(run.release)
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRuntimeFinalAppGuardCannotRestoreRetiredLease(t *testing.T) {
	p := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 2), perform: func(_ context.Context, _ ProviderAction, g func() error) error { return g() }}
	r := NewRuntime(Deps{Providers: Providers{Audio: p}})
	q := runtimeRequest(actionPackage(t))
	q.Grants = []string{"audio.output.select"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, p.runs)
	run.emit(Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "owned", Label: "Output"}}}}})
	s := waitRuntime(t, r, func(s InstanceState) bool { return s.Root.ActionToken != "" })
	o, err := r.ActionOptions(ctx, s.Lease, s.Root.ActionToken)
	if err != nil {
		t.Fatal(err)
	}
	err = r.Perform(ctx, s.Lease, s.Root.ActionToken, o.Options[0].Token, nil, func() error { q.Enabled = false; return r.Configure([]Request{q}) })
	if !errors.Is(err, ErrRetired) {
		t.Fatal("guard-side revocation admitted", err)
	}
	cancel()
	close(run.release)
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRuntimeFormatterSettingRejectsSpoofedEnums(t *testing.T) {
	p, ok := Builtin("org.optiontab.clock")
	if !ok {
		t.Fatal("missing clock")
	}
	m := p.Manifest()
	value := "longTime"
	settings, err := ValidateSettings(m, map[string]Value{"format": {Text: &value}})
	if err != nil || *settings["format"].Text != value {
		t.Fatal("format choice", err)
	}
	value = "eval"
	if _, err = ValidateSettings(m, map[string]Value{"format": {Text: &value}}); err == nil {
		t.Fatal("runtime formatter expression accepted")
	}
	m.Settings[1].Options = append(m.Settings[1].Options, "bytesPerSecond")
	raw, _ := json.Marshal(m)
	if _, err = ParseManifest(raw); err == nil {
		t.Fatal("wrong-field formatter choice accepted")
	}
}

func TestRuntimeMalformedSamplesCannotInventReadings(t *testing.T) {
	now := time.Now()
	bad := math.NaN()
	s := sanitizeSample("battery", []string{"battery.read"}, Sample{Generation: 1, Sequence: 1, ObservedAt: now, Status: "ready", Fields: map[string]Value{"charge": {Number: &bad}}}, now)
	if s.Status != "unavailable" || len(s.Fields) != 0 {
		t.Fatal("NaN reading admitted")
	}
	text := "private"
	s = sanitizeSample("audio", []string{"audio.output.select"}, Sample{Generation: 1, Sequence: 1, ObservedAt: now, Status: "ready", Fields: map[string]Value{"outputName": {Text: &text}}}, now)
	if len(s.Fields) != 0 {
		t.Fatal("control-only grant exposed status")
	}
}

func TestRuntimeProgressDoesNotRetirePreparedActionOrChooser(t *testing.T) {
	m := testManifest()
	m.RequiredCapabilities = []string{"media.music.read", "media.music.control"}
	m.Root = Node{Kind: "column", Children: []Node{bindingNode("music", "position", "duration"), {Kind: "button", Text: "Next", Command: &Command{Provider: "music", Action: "next"}}}}
	pkg, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, nil)))
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	source := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 2), perform: func(_ context.Context, _ ProviderAction, g func() error) error { close(entered); <-release; return g() }}
	r := NewRuntime(Deps{Providers: Providers{Music: source}})
	q := runtimeRequest(pkg)
	q.Grants = m.RequiredCapabilities
	if err = r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, source.runs)
	position := 1000.0
	sample := Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Fields: map[string]Value{"position": {Number: &position}}, Actions: map[string]ActionSpec{"next": {Enabled: true}}}
	run.emit(sample)
	s := waitRuntime(t, r, func(s InstanceState) bool { return len(s.Root.Children) == 2 && s.Root.Children[1].ActionToken != "" })
	token := s.Root.Children[1].ActionToken
	actionDone := make(chan error, 1)
	go func() { actionDone <- r.Perform(ctx, s.Lease, token, "", nil, func() error { return nil }) }()
	runtimeReceive(t, entered)
	position = 2000
	sample.Sequence++
	sample.Fields["position"] = Value{Number: &position}
	run.emit(sample)
	fresh := waitRuntime(t, r, func(s2 InstanceState) bool { return s2.Lease.Revision > s.Lease.Revision })
	if fresh.Root.Children[1].ActionToken != token {
		t.Error("progress changed action token")
	}
	if _, err = r.ActionOptions(ctx, s.Lease, token); err != nil {
		t.Error("original chooser lease retired", err)
	}
	close(release)
	if err = runtimeReceive(t, actionDone); err != nil {
		t.Error("progress retired prepared action", err)
	}
	cancel()
	close(run.release)
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRuntimeActionAuthorityABADoesNotReviveToken(t *testing.T) {
	p := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 2), perform: func(_ context.Context, _ ProviderAction, g func() error) error { return g() }}
	r := NewRuntime(Deps{Providers: Providers{Audio: p}})
	q := runtimeRequest(actionPackage(t))
	q.Grants = []string{"audio.output.select"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, p.runs)
	a := Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "a", Label: "A"}}}}}
	run.emit(a)
	original := waitRuntime(t, r, func(s InstanceState) bool { return s.Root.ActionToken != "" })
	b := a
	b.Sequence = 2
	b.Actions = map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "b", Label: "B"}}}}
	run.emit(b)
	a.Sequence = 3
	run.emit(a)
	if _, err := r.ActionOptions(ctx, original.Lease, original.Root.ActionToken); err != ErrRetired {
		t.Fatal("ABA revived old action")
	}
	fresh := waitRuntime(t, r, func(s InstanceState) bool {
		return s.Root.ActionToken != "" && s.Root.ActionToken != original.Root.ActionToken
	})
	if _, err := r.ActionOptions(ctx, fresh.Lease, fresh.Root.ActionToken); err != nil {
		t.Fatal(err)
	}
	cancel()
	close(run.release)
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRuntimeProviderReplacementWaitsForCancelledAction(t *testing.T) {
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	source := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 4)}
	source.perform = func(ctx context.Context, _ ProviderAction, g func() error) error {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		<-release
		return g()
	}
	r := NewRuntime(Deps{Providers: Providers{Audio: source}})
	q := runtimeRequest(actionPackage(t))
	q.Grants = []string{"audio.output.select"}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, source.runs)
	run.emit(Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Actions: map[string]ActionSpec{"selectOutput": {Enabled: true, Options: []ProviderOption{{ID: "owned", Label: "Output"}}}}})
	state := waitRuntime(t, r, func(s InstanceState) bool { return s.Root.ActionToken != "" })
	options, err := r.ActionOptions(ctx, state.Lease, state.Root.ActionToken)
	if err != nil {
		t.Fatal(err)
	}
	actionDone := make(chan error, 1)
	go func() {
		actionDone <- r.Perform(ctx, state.Lease, state.Root.ActionToken, options.Options[0].Token, nil, func() error { return nil })
	}()
	runtimeReceive(t, entered)
	r.mu.Lock()
	owner := r.owners["audio"]
	r.mu.Unlock()
	if err = r.Configure(nil); err != nil {
		t.Fatal(err)
	}
	if err = r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	runtimeReceive(t, cancelled)
	close(run.release)
	runtimeReceive(t, owner.done)
	deadline := time.Now().Add(3 * time.Second)
	processed := false
	for time.Now().Before(deadline) {
		r.mu.Lock()
		processed = !owner.endedAt.IsZero() || r.owners["audio"] != owner
		r.mu.Unlock()
		if processed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	r.mu.Lock()
	same := r.owners["audio"] == owner
	r.mu.Unlock()
	if !processed || !same {
		t.Error("provider replaced before preparing action joined")
	}
	close(release)
	if err = runtimeReceive(t, actionDone); err != ErrRetired {
		t.Error("cancelled action admitted", err)
	}
	next := runtimeReceive(t, source.runs)
	cancel()
	close(next.release)
	if err = runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

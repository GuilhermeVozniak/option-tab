package widgets

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestRuntimeBuiltinsDeterministicAndClockSettings(t *testing.T) {
	list := Builtins()
	if len(list) != 4 {
		t.Fatal("builtin count")
	}
	for _, p := range list {
		again, ok := Builtin(p.Manifest().ID)
		if !ok || again.Digest() != p.Digest() {
			t.Fatal("unstable builtin")
		}
		expected := map[string]string{"org.optiontab.clock": "d0420a7f2f42e7a4600de0baa40f5886d927227e0cb6846336bca9944e5702b9", "org.optiontab.battery": "0f2661d5fcd4501af3e7abee68337afe3a0b40fe6b3f53625431e52158b0221b", "org.optiontab.network": "2560b76f1fedcd7a78bedcb5f94973c4e74a5d1a3de1fae0405c7e4e31dcc04b", "org.optiontab.audio": "14413bfa741dd72957c0ead9199e23912c0aff79e0127618be773a22459ae790"}
		if expected[p.Manifest().ID] != p.Digest() {
			t.Fatal("builtin immutable digest changed")
		}
	}
	list[0] = nil
	if Builtins()[0] == nil {
		t.Fatal("builtin list alias")
	}
	clock, _ := Builtin("org.optiontab.clock")
	r := NewRuntime(Deps{Now: func() time.Time { return time.Date(2026, 9, 7, 12, 34, 56, 0, time.UTC) }})
	q := runtimeRequest(clock)
	format, zone := "longTime", "UTC"
	q.Grants = []string{"clock.read"}
	q.Settings = map[string]Value{"format": {Text: &format}, "timezone": {Text: &zone}}
	if err := r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	s := waitRuntime(t, r, func(s InstanceState) bool { return s.Root.Text == "12:34:56" })
	if s.Status != "ready" {
		t.Fatal("clock not ready")
	}
	if len(r.owners) != 0 {
		t.Fatal("clock created provider")
	}
	cancel()
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRuntimeOptionalNodesAndBoundedHistory(t *testing.T) {
	m := testManifest()
	m.RequiredCapabilities = []string{"network.status.read"}
	m.OptionalCapabilities = []string{"network.usage.read"}
	m.Root = Node{Kind: "column", Children: []Node{bindingNode("network", "category", "text"), {Kind: "sparkline", History: 120, Binding: &Binding{Provider: "network", Field: "uploadRate", Formatter: "number"}}}}
	p, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, nil)))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRuntime(Deps{})
	q := runtimeRequest(p)
	q.Grants = []string{"network.status.read"}
	if err = r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o := &providerOwner{name: "network", ctx: ctx, caps: []string{"network.status.read", "network.usage.read"}, serial: 1}
	r.owners["network"] = o
	text := "wifi"
	value := 10.0
	o.sample = Sample{Generation: 1, Sequence: 1, Status: "ready", Fields: map[string]Value{"category": {Text: &text}, "uploadRate": {Number: &value}}}
	r.resolveLocked(r.instances[0])
	s := r.Snapshot()[0]
	if s.Status != "partial" || s.Root.Children[0].Text != "wifi" || s.Root.Children[1].Status != "unavailable" {
		t.Fatal("optional denial hid allowed node")
	}
	// Grant via normal admission, then inject already-validated owner samples into
	// the pure reduction seam to exercise the complete bounded history window.
	q.Grants = append(q.Grants, "network.usage.read")
	o.cancel = func() {}
	if err = r.Configure([]Request{q}); err != nil {
		t.Fatal(err)
	}
	o.retired = false
	for j := 1; j <= 150; j++ {
		v := float64(j)
		o.sample = Sample{Generation: 1, Sequence: uint64(j), Status: "ready", Fields: map[string]Value{"category": {Text: &text}, "uploadRate": {Number: &v}}}
		r.resolveLocked(r.instances[0])
	}
	h := r.Snapshot()[0].Root.Children[1].History
	if len(h) != 120 || h[0] != 31 || h[119] != 150 {
		t.Fatal("history bounds", len(h))
	}
	h[0] = -1
	if r.Snapshot()[0].Root.Children[1].History[0] != 31 {
		t.Fatal("history alias")
	}
	o.sample.Generation = 2
	o.sample.Sequence = 1
	r.resolveLocked(r.instances[0])
	if len(r.Snapshot()[0].Root.Children[1].History) != 1 {
		t.Fatal("history crossed provider generation")
	}
}

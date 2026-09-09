package widgets

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func testManifest() Manifest {
	return Manifest{SchemaVersion: 1, ID: "org.example.clock", Version: "1.0.0", MinimumAppVersion: "0.4.8", Name: Localized{"en": "Clock"}, Description: Localized{"en": "Local clock"}, RequiredCapabilities: []string{"clock.read"}, Root: Node{Kind: "text", Binding: &Binding{Provider: "clock", Field: "time", Formatter: "shortTime"}}}
}

func testZIP(t *testing.T, m Manifest, extra map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	f, err := w.Create("widget.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(raw); err != nil {
		t.Fatal(err)
	}
	for name, data := range extra {
		f, err = w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPackageRoundTripImmutableAndInert(t *testing.T) {
	ctx := context.Background()
	m := testManifest()
	archive := testZIP(t, m, nil)
	p, err := Preview(ctx, bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := p.Manifest()
	snapshot.Name["en"] = "changed"
	snapshot.Root.Binding.Field = "invalid"
	if p.Manifest().Name["en"] != "Clock" || p.Manifest().Root.Binding.Field != "time" {
		t.Fatal("mutable package")
	}
	root := t.TempDir()
	store, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	installed, err := store.Install(ctx, bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(installed.Digest())
	if err != nil || got.Digest() != p.Digest() {
		t.Fatalf("round trip %v", err)
	}
	data, err := json.Marshal(got.Manifest())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("grants")) || bytes.Contains(data, []byte("enabled")) {
		t.Fatal("installation grants authority")
	}
}

func TestManifestRejectsAuthorityAndDuplicateKeys(t *testing.T) {
	raw, _ := json.Marshal(testManifest())
	for _, extra := range []string{`"script":"run",`, `"id":"duplicate",`, `"grants":["clock.read"],`} {
		if _, err := ParseManifest(append([]byte("{"+extra), raw[1:]...)); err == nil {
			t.Fatal("unsafe manifest accepted")
		}
	}
	m := testManifest()
	m.RequiredCapabilities = []string{"process.execute"}
	raw, _ = json.Marshal(m)
	if _, err := ParseManifest(raw); err == nil {
		t.Fatal("unknown capability accepted")
	}
	m = testManifest()
	m.Root = Node{Kind: "button", Text: "Next", Command: &Command{Provider: "music", Action: "next"}}
	raw, _ = json.Marshal(m)
	if _, err := ParseManifest(raw); err == nil {
		t.Fatal("undeclared control accepted")
	}
}

func TestArchiveRejectsUntrustedPayloadsAndCancellation(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "Widget.json", "assets/undeclared.png", "assets/../escape", "assets/ü.png", "assets/CON.png"} {
		if _, err := Preview(context.Background(), bytes.NewReader(testZIP(t, testManifest(), map[string][]byte{name: []byte("payload")}))); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	root := t.TempDir()
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Install(ctx, bytes.NewReader(testZIP(t, testManifest(), nil))); err == nil {
		t.Fatal("cancelled install accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("cancel left staging")
	}
}

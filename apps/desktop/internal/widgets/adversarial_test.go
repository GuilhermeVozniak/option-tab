package widgets

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"sync"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func withAsset(m Manifest, b []byte) Manifest {
	sum := sha256.Sum256(b)
	m.Assets = []Asset{{ID: "icon", Path: "assets/icon.png", SHA256: hex.EncodeToString(sum[:])}}
	m.Root = Node{Kind: "icon", Asset: "icon"}
	return m
}

func TestPNGAssetsDigestIdentityAndReadCopies(t *testing.T) {
	b := pngBytes(t)
	m := withAsset(testManifest(), b)
	p, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, map[string][]byte{"assets/icon.png": b})))
	if err != nil {
		t.Fatal(err)
	}
	copyBytes, err := p.Asset("icon")
	if err != nil {
		t.Fatal(err)
	}
	copyBytes[0] = 0
	again, _ := p.Asset("icon")
	if again[0] == 0 {
		t.Fatal("asset aliases package")
	}
	m.Name["en"] = "Another name"
	other, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, map[string][]byte{"assets/icon.png": b})))
	if err != nil || other.Digest() == p.Digest() {
		t.Fatal("changed manifest retained digest")
	}
	m.Assets[0].SHA256 = string(bytes.Repeat([]byte("0"), 64))
	if _, err = Preview(context.Background(), bytes.NewReader(testZIP(t, m, map[string][]byte{"assets/icon.png": b}))); err == nil {
		t.Fatal("asset digest mismatch accepted")
	}
}

func TestArchiveLinksCollisionAndExpansion(t *testing.T) {
	raw, _ := json.Marshal(testManifest())
	for _, kind := range []string{"link", "collision", "bomb", "many"} {
		var b bytes.Buffer
		w := zip.NewWriter(&b)
		h := &zip.FileHeader{Name: "widget.json", Method: zip.Deflate}
		if kind == "link" {
			h.SetMode(os.ModeSymlink | 0o777)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(raw); err != nil {
			t.Fatal(err)
		}
		names := []string{}
		switch kind {
		case "collision":
			names = []string{"WIDGET.JSON"}
		case "bomb":
			names = []string{"assets/bomb.png"}
		case "many":
			for i := range 65 {
				names = append(names, "assets/a"+string(rune('A'+i))+".png")
			}
		}
		for _, name := range names {
			f, err = w.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			data := []byte("x")
			if kind == "bomb" {
				data = bytes.Repeat([]byte("x"), MaxExpanded+1)
			}
			if _, err = f.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err = w.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err = Preview(context.Background(), bytes.NewReader(b.Bytes())); err == nil {
			t.Fatalf("%s accepted", kind)
		}
	}
}

func TestOversizedPNGRejectedBeforeDecode(t *testing.T) {
	b := pngBytes(t)
	binary.BigEndian.PutUint32(b[16:20], 100000)
	binary.BigEndian.PutUint32(b[20:24], 100000)
	binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
	if _, err := png.DecodeConfig(bytes.NewReader(b)); err != nil {
		t.Fatal("invalid oversized fixture", err)
	}
	m := withAsset(testManifest(), b)
	if _, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, map[string][]byte{"assets/icon.png": b}))); err == nil {
		t.Fatal("oversized decoded image accepted")
	}
}

func TestTypedBindingsSettingsAndTreeBounds(t *testing.T) {
	for _, mutate := range []func(*Manifest){
		func(m *Manifest) { m.Root.Binding.Field = "process.pid" }, func(m *Manifest) { m.Root.Binding.Formatter = "eval" }, func(m *Manifest) { m.Root.Binding.TimezoneSetting = "unknown" }, func(m *Manifest) { m.Name["fr"] = "Clock" }, func(m *Manifest) { m.Version = "1.0.0-01" }, func(m *Manifest) { m.OptionalCapabilities = []string{"clock.read"} },
		func(m *Manifest) {
			for range 9 {
				m.Root = Node{Kind: "row", Children: []Node{m.Root}}
			}
		}, func(m *Manifest) { m.Root = Node{Kind: "row", Children: make([]Node, 17)} }, func(m *Manifest) {
			m.Root = Node{Kind: "sparkline", History: 121, Binding: &Binding{Provider: "clock", Field: "time", Formatter: "shortTime"}}
		},
	} {
		m := testManifest()
		mutate(&m)
		raw, _ := json.Marshal(m)
		if _, err := ParseManifest(raw); err == nil {
			t.Fatal("invalid typed manifest accepted")
		}
	}
	m := testManifest()
	m.RequiredCapabilities = []string{"media.music.control"}
	m.Root = Node{Kind: "button", Text: "Next", Command: &Command{Provider: "music", Action: "next"}}
	raw, _ := json.Marshal(m)
	if _, err := ParseManifest(raw); err != nil {
		t.Fatal("declared fixed command rejected", err)
	}
}

func TestStoreConcurrentInstallCorruptionAndClose(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	archive := testZIP(t, testManifest(), nil)
	var wg sync.WaitGroup
	results := make(chan *Package, 4)
	errs := make(chan error, 4)
	for range 4 {
		wg.Go(func() { p, e := s.Install(context.Background(), bytes.NewReader(archive)); results <- p; errs <- e })
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	digest := ""
	for p := range results {
		if digest != "" && digest != p.Digest() {
			t.Fatal("concurrent identities differ")
		}
		digest = p.Digest()
	}
	if err = s.root.WriteFile(digest+"/package.zip", []byte("corruption"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(digest); err == nil {
		t.Fatal("corrupt installed package accepted")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(digest); err == nil {
		t.Fatal("closed store readable")
	}
}

func TestOriginalClockExampleAndCompressedLimit(t *testing.T) {
	raw, err := os.ReadFile("testdata/clock/widget.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseManifest(raw); err != nil {
		t.Fatal(err)
	}
	if _, err = Preview(context.Background(), bytes.NewReader(make([]byte, MaxCompressed+1))); err == nil {
		t.Fatal("compressed limit ignored")
	}
}

func TestZIPAlternateNamesAndHardLinkMetadataAreRefused(t *testing.T) {
	raw, _ := json.Marshal(testManifest())
	for _, extra := range [][]byte{{0x0d, 0, 0, 0}, {0x75, 0x70, 0, 0}, {1, 0, 0, 0}} {
		var b bytes.Buffer
		w := zip.NewWriter(&b)
		f, err := w.CreateHeader(&zip.FileHeader{Name: "widget.json", Extra: extra})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err = w.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err = Preview(context.Background(), bytes.NewReader(b.Bytes())); err == nil {
			t.Fatal("ambiguous link/name metadata accepted")
		}
	}
}

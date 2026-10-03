package platform

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
)

func TestLauncherItemLifetimeJoinsCanceledWork(t *testing.T) {
	l := newLauncherItemLifetime()
	ctx, end, err := l.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { l.close(); close(done) }()
	<-ctx.Done()
	select {
	case <-done:
		t.Fatal("close returned before owner joined")
	default:
	}
	if _, _, err = l.begin(context.Background()); err == nil {
		t.Fatal("retired source admitted")
	}
	end()
	<-done
}

func TestLauncherItemLinkAndIconBounds(t *testing.T) {
	for _, s := range []string{"file:///tmp/a", "https://user:secret@example.com", "javascript:alert(1)", "https://example.com/\x00"} {
		if validLauncherLink(s) {
			t.Fatalf("accepted %q", s)
		}
	}
	if !validLauncherLink("https://example.com/a?q=1#b") {
		t.Fatal("valid link refused")
	}
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 1025, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeLauncherIcon(context.Background(), b.Bytes()); err == nil {
		t.Fatal("oversize icon accepted")
	}
	b.Reset()
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 512, 256))); err != nil {
		t.Fatal(err)
	}
	data, err := normalizeLauncherIcon(context.Background(), b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 256 || cfg.Height != 128 {
		t.Fatalf("size %#v %v", cfg, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := normalizeLauncherIcon(ctx, b.Bytes()); err == nil {
		t.Fatal("canceled icon returned")
	}
}

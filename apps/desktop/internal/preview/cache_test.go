package preview

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type identifiedSource struct {
	*source
	identity platform.AutomationWindowIdentity
	current  atomic.Bool
}

func (s *identifiedSource) WindowIdentity(id domain.WindowID) (platform.AutomationWindowIdentity, error) {
	return s.identity, nil
}

func (s *identifiedSource) WindowIdentityCurrent(identity platform.AutomationWindowIdentity) bool {
	return identity == s.identity && s.current.Load()
}

func (s *identifiedSource) StreamWindowWithIdentity(ctx context.Context, identity platform.AutomationWindowIdentity, px int, frame func(string)) error {
	if !s.WindowIdentityCurrent(identity) {
		return platform.WindowUnavailableError{}
	}
	return s.StreamWindow(ctx, identity.ID, px, func(url string) {
		if s.WindowIdentityCurrent(identity) {
			frame(url)
		}
	})
}

func (s *identifiedSource) ThumbnailDataURLWithIdentity(ctx context.Context, identity platform.AutomationWindowIdentity, px int) (string, error) {
	if !s.WindowIdentityCurrent(identity) {
		return "", platform.WindowUnavailableError{}
	}
	return s.ThumbnailDataURL(identity.ID, px), nil
}

func TestCachedFramesHaveCopiedCaptureIdentityWithoutStartingWork(t *testing.T) {
	s := &identifiedSource{source: &source{active: map[domain.WindowID]func(string){}}, identity: platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 23, StartSeconds: 456}}}
	s.current.Store(true)
	m := New(s, func(domain.WindowID, string) {})
	defer m.Close()
	if len(m.CachedFrames()) != 0 {
		t.Fatal("empty cache returned frames")
	}
	s.mu.Lock()
	started := len(s.active) + s.snapshots
	s.mu.Unlock()
	if started != 0 {
		t.Fatal("query started capture")
	}
	m.Update([]domain.WindowID{1}, 1, 256)
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.active[1] != nil })
	s.mu.Lock()
	frame := s.active[1]
	s.mu.Unlock()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	frame("data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes()))
	m.Hide()
	got := m.CachedFrames()
	if len(got) != 1 || got[0].Window != s.identity || got[0].CapturedAt.Before(before) || !bytes.Equal(got[0].PNG, data.Bytes()) {
		t.Fatalf("wrong cache attribution: %+v", got)
	}
	got[0].PNG[0] = 0
	if !bytes.Equal(m.CachedFrames()[0].PNG, data.Bytes()) {
		t.Fatal("caller mutated private cached bytes")
	}
	m.Close()
	if len(m.CachedFrames()) != 0 {
		t.Fatal("shutdown retained exported images")
	}
}

type retiringIdentitySource struct {
	*identifiedSource
	retired chan struct{}
}

func (s *retiringIdentitySource) StreamWindow(ctx context.Context, id domain.WindowID, _ int, frame func(string)) error {
	s.mu.Lock()
	s.active[id] = frame
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.retired:
		return unavailableError{}
	}
}

func (s *retiringIdentitySource) StreamWindowWithIdentity(ctx context.Context, identity platform.AutomationWindowIdentity, px int, frame func(string)) error {
	return s.StreamWindow(ctx, identity.ID, px, frame)
}

func TestUnavailableCaptureRetiresExportedCache(t *testing.T) {
	s := &retiringIdentitySource{identifiedSource: &identifiedSource{source: &source{active: map[domain.WindowID]func(string){}}, identity: platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 23, StartSeconds: 456}}}, retired: make(chan struct{})}
	s.current.Store(true)
	m := New(s, func(domain.WindowID, string) {})
	defer m.Close()
	m.Update([]domain.WindowID{1}, 1, 256)
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.active[1] != nil })
	s.mu.Lock()
	frame := s.active[1]
	s.mu.Unlock()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	frame(url)
	if len(m.CachedFrames()) != 1 {
		t.Fatal("initial valid frame was not retained")
	}
	close(s.retired)
	eventually(t, func() bool { return len(m.CachedFrames()) == 0 })
}

func TestIndependentOwnersShareCaptureBudget(t *testing.T) {
	s := &source{active: map[domain.WindowID]func(string){}}
	first := New(s, func(domain.WindowID, string) {})
	second := first.NewPeer(func(domain.WindowID, string) {})
	defer first.Close()
	defer second.Close()
	first.Update([]domain.WindowID{1, 2, 3, 4}, 1, 256)
	second.Update([]domain.WindowID{5, 6, 7, 8}, 5, 256)
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.active[1] != nil && s.active[2] != nil && s.active[5] != nil && s.active[6] != nil
	})
	first.Hide()
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.active) == 2 && s.active[5] != nil && s.active[6] != nil
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.max > 4 {
		t.Fatalf("combined stream budget exceeded: %d", s.max)
	}
}

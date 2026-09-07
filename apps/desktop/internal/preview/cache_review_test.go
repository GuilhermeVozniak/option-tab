package preview

import (
	"context"
	"errors"
	"testing"
	"time"

	"option-tab/internal/domain"
)

type failingPeerStreamSource struct {
	streams   chan struct{}
	snapshots chan struct{}
	release   chan struct{}
}

func (s *failingPeerStreamSource) StreamWindow(context.Context, domain.WindowID, int, func(string)) error {
	s.streams <- struct{}{}
	return errors.New("stream unavailable")
}

func (s *failingPeerStreamSource) ThumbnailDataURL(domain.WindowID, int) string {
	s.snapshots <- struct{}{}
	<-s.release
	return "snapshot"
}

func TestPeerSnapshotFallbackUsesSharedSingleSlot(t *testing.T) {
	s := &failingPeerStreamSource{streams: make(chan struct{}, 4), snapshots: make(chan struct{}, 4), release: make(chan struct{})}
	first := New(s, func(domain.WindowID, string) {})
	second := first.NewPeer(func(domain.WindowID, string) {})
	defer first.Close()
	defer second.Close()
	defer close(s.release)
	first.Update([]domain.WindowID{1, 2}, 1, 256)
	second.Update([]domain.WindowID{3, 4}, 3, 256)
	for i := 0; i < 4; i++ {
		select {
		case <-s.streams:
		case <-time.After(time.Second):
			t.Fatal("live fallback never attempted")
		}
	}
	select {
	case <-s.snapshots:
	case <-time.After(time.Second):
		t.Fatal("missing first snapshot")
	}
	select {
	case <-s.snapshots:
		t.Fatal("second snapshot started while shared snapshot slot was occupied")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestPeerFallbackCancellationReleasesWaitingLeases(t *testing.T) {
	s := &failingPeerStreamSource{streams: make(chan struct{}, 4), snapshots: make(chan struct{}, 4), release: make(chan struct{})}
	first := New(s, func(domain.WindowID, string) {})
	second := first.NewPeer(func(domain.WindowID, string) {})
	first.Update([]domain.WindowID{1, 2}, 1, 256)
	second.Update([]domain.WindowID{3, 4}, 3, 256)
	for i := 0; i < 4; i++ {
		select {
		case <-s.streams:
		case <-time.After(time.Second):
			close(s.release)
			first.Close()
			second.Close()
			t.Fatal("missing stream attempt")
		}
	}
	select {
	case <-s.snapshots:
	case <-time.After(time.Second):
		close(s.release)
		first.Close()
		second.Close()
		t.Fatal("missing snapshot")
	}
	first.Close()
	second.Close()
	close(s.release)
	eventually(t, func() bool { return len(first.streams) == 0 && len(first.snapshots) == 0 })
	if len(s.snapshots) != 0 {
		t.Fatal("cancelled waiters captured after snapshot slot became free")
	}
}

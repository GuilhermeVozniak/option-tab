package platform

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"

	"option-tab/internal/domain"
)

type materialFake struct {
	apply                  func(MaterialStyle, func() bool) error
	calls, closed, retired int
}

func (f *materialFake) Apply(_ uint64, s MaterialStyle, g func() bool) error {
	f.calls++
	if f.apply != nil {
		return f.apply(s, g)
	}
	if !g() {
		return ErrMaterialRetired
	}
	return nil
}

func (f *materialFake) Retire(_ uint64, _ MaterialScope, _ int, g func() bool) error {
	if !g() {
		return ErrMaterialRetired
	}
	f.retired++
	return nil
}
func (f *materialFake) Close(uint64) error { f.closed++; return nil }
func materialStyle() MaterialStyle {
	return MaterialStyle{Scope: MaterialScope{Session: 1, Revision: 1}, Enabled: true, Theme: "system", CornerRadiusPx: 18, Rect: domain.Bounds{X: 10, Y: 20, W: 200, H: 100}}
}

func TestMaterialOwnershipRetireAndClose(t *testing.T) {
	n := &materialFake{}
	p := newMaterialSurface(1, n)
	s := materialStyle()
	if status, err := p.Apply(context.Background(), s); err != nil || status.State != "system" {
		t.Fatal(status, err)
	}
	if err := p.Retire(s.Scope, 180); err != nil {
		t.Fatal(err)
	}
	s.Scope.Revision++
	if _, err := p.Apply(context.Background(), s); !errors.Is(err, ErrMaterialRetired) {
		t.Fatal("retired session reused", err)
	}
	s.Scope.Session++
	s.Scope.Revision++
	if _, err := p.Apply(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Apply(context.Background(), s); err == nil || n.closed != 1 {
		t.Fatal("closed owner accepted or closed twice")
	}
}

func TestMaterialCancelOrReplacementDuringNativePreparation(t *testing.T) {
	for _, replace := range []bool{false, true} {
		n := &materialFake{}
		p := newMaterialSurface(1, n)
		ctx, cancel := context.WithCancel(context.Background())
		s := materialStyle()
		n.apply = func(_ MaterialStyle, guard func() bool) error {
			if replace {
				if err := p.Retire(s.Scope, 0); err != nil {
					t.Fatal(err)
				}
			} else {
				cancel()
			}
			if guard() {
				t.Fatal("stale native preparation admitted")
			}
			return ErrMaterialRetired
		}
		if status, err := p.Apply(ctx, s); err == nil || status.State != "unavailable" {
			t.Fatal(status, err)
		}
		cancel()
	}
}

func TestMaterialValidationAndSolidWithoutRect(t *testing.T) {
	for _, mutate := range []func(*MaterialStyle){func(s *MaterialStyle) { s.Scope.Session = 0 }, func(s *MaterialStyle) { s.Rect.X = math.NaN() }, func(s *MaterialStyle) { s.Rect.W = 65537 }, func(s *MaterialStyle) { s.Rect.Y = -1 }, func(s *MaterialStyle) { s.Theme = "css" }, func(s *MaterialStyle) { s.CornerRadiusPx = 65 }} {
		n := &materialFake{}
		p := newMaterialSurface(1, n)
		s := materialStyle()
		mutate(&s)
		if _, err := p.Apply(context.Background(), s); err == nil || n.calls != 0 {
			t.Fatal("invalid native mutation")
		}
	}
	n := &materialFake{}
	p := newMaterialSurface(1, n)
	s := materialStyle()
	s.Enabled = false
	s.Rect = domain.Bounds{}
	if status, err := p.Apply(context.Background(), s); err != nil || status.State != "solid" {
		t.Fatal(status, err)
	}
}

type blockingMaterialNative struct {
	started, release chan struct{}
	calls, writes    atomic.Int32
}

func (n *blockingMaterialNative) Apply(_ uint64, _ MaterialStyle, guard func() bool) error {
	if n.calls.Add(1) == 1 {
		close(n.started)
		<-n.release
	}
	if !guard() {
		return ErrMaterialRetired
	}
	n.writes.Add(1)
	return nil
}

func (n *blockingMaterialNative) Retire(uint64, MaterialScope, int, func() bool) error { return nil }

func (n *blockingMaterialNative) Close(uint64) error { return nil }

func TestMaterialBlockedOldApplyCannotPublishAfterSuccessor(t *testing.T) {
	n := &blockingMaterialNative{started: make(chan struct{}), release: make(chan struct{})}
	p := newMaterialSurface(1, n)
	first := materialStyle()
	done := make(chan error, 1)
	go func() { _, err := p.Apply(context.Background(), first); done <- err }()
	<-n.started
	next := first
	next.Scope = MaterialScope{Session: 2, Revision: 2}
	if _, err := p.Apply(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	close(n.release)
	if err := <-done; !errors.Is(err, ErrMaterialRetired) {
		t.Fatal("old result admitted", err)
	}
	if n.writes.Load() != 1 {
		t.Fatal("old native operation mutated successor")
	}
}

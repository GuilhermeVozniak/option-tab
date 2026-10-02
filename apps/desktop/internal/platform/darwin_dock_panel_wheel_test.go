//go:build darwin

package platform

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"option-tab/internal/domain"
)

type fakePanelWheel struct {
	mu        sync.Mutex
	policy    DockPanelWheelPolicy
	events    chan DockPanelWheelEvent
	closed    bool
	hook      func()
	closeHook func()
	emptyHook func()
}

func (f *fakePanelWheel) Show(uint64, domain.Bounds) error { return nil }
func (f *fakePanelWheel) Hide(uint64) error                { return nil }
func (f *fakePanelWheel) Close(uint64) error {
	if f.closeHook != nil {
		f.closeHook()
	}
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

func (f *fakePanelWheel) SetPolicy(_ uint64, p DockPanelWheelPolicy) error {
	if f.hook != nil {
		f.hook()
	}
	f.mu.Lock()
	f.policy = p
	f.mu.Unlock()
	return nil
}

func (f *fakePanelWheel) Next(uint64) (DockPanelWheelEvent, int) {
	select {
	case e := <-f.events:
		return e, 1
	default:
	}
	f.mu.Lock()
	closed := f.closed
	f.mu.Unlock()
	if closed {
		return DockPanelWheelEvent{}, -1
	}
	if f.emptyHook != nil {
		f.emptyHook()
	}
	return DockPanelWheelEvent{}, 0
}
func (f *fakePanelWheel) Valid(uint64, uint64, uint64, uint64) bool { return true }
func (f *fakePanelWheel) Complete(uint64, uint64, uint64, uint64)   {}
func TestDarwinPanelWheelCopiesPolicyAndDeliversOffSetter(t *testing.T) {
	f := &fakePanelWheel{events: make(chan DockPanelWheelEvent, 2)}
	p := newDarwinWheelPanel(newDockPanel(1, f), f)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	policy := DockPanelWheelPolicy{Session: 1, Revision: 2, Enabled: true, Regions: []PreviewRegion{{WindowID: 42, AppID: 7, Bounds: domain.Bounds{W: 10, H: 10}}}}
	got := make(chan DockPanelWheelEvent, 1)
	if err := p.SetDockPanelWheelPolicy(policy, func(e DockPanelWheelEvent) { got <- e }); err != nil {
		t.Fatal(err)
	}
	policy.Regions[0].WindowID = 99
	f.mu.Lock()
	id := f.policy.Regions[0].WindowID
	f.mu.Unlock()
	if id != 42 {
		t.Fatal("caller mutated copied policy")
	}
	f.events <- DockPanelWheelEvent{Session: 1, Revision: 2, WindowID: 42, GestureID: 3}
	select {
	case e := <-got:
		if e.WindowID != 42 {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}
}

func TestDarwinPanelWheelCloseDoesNotWaitForBlockedEmission(t *testing.T) {
	f := &fakePanelWheel{events: make(chan DockPanelWheelEvent, 1)}
	p := newDarwinWheelPanel(newDockPanel(1, f), f)
	entered, release := make(chan struct{}), make(chan struct{})
	if e := p.SetDockPanelWheelPolicy(DockPanelWheelPolicy{Session: 1, Revision: 1, Enabled: true}, func(DockPanelWheelEvent) { close(entered); <-release }); e != nil {
		t.Fatal(e)
	}
	f.events <- DockPanelWheelEvent{GestureID: 1}
	<-entered
	done := make(chan struct{})
	go func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close waited on emission")
	}
	close(release)
	select {
	case <-p.wheelDone:
	case <-time.After(time.Second):
		t.Fatal("worker leaked")
	}
}

func TestDarwinPanelWheelNoGoLockAcrossNativePolicy(t *testing.T) {
	f := &fakePanelWheel{events: make(chan DockPanelWheelEvent, 1)}
	p := newDarwinWheelPanel(newDockPanel(1, f), f)
	f.hook = func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- p.SetDockPanelWheelPolicy(DockPanelWheelPolicy{Session: 1, Revision: 1, Enabled: true}, func(DockPanelWheelEvent) {})
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed source admitted policy")
		}
	case <-time.After(time.Second):
		t.Fatal("native reentry deadlocked")
	}
}

func TestDarwinPanelWheelCloseDrainsCancellationAfterEmptyPoll(t *testing.T) {
	closeEntered, finishClose, emptyDuringClose := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce, emptyOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(finishClose) }) }
	t.Cleanup(release)
	f := &fakePanelWheel{events: make(chan DockPanelWheelEvent, 1)}
	f.closeHook = func() {
		close(closeEntered)
		<-finishClose
		f.events <- DockPanelWheelEvent{Session: 1, Revision: 1, GestureID: 3, Phase: "cancelled"}
	}
	f.emptyHook = func() {
		select {
		case <-closeEntered:
			emptyOnce.Do(func() { close(emptyDuringClose) })
		default:
		}
	}
	p := newDarwinWheelPanel(newDockPanel(1, f), f)
	got := make(chan DockPanelWheelEvent, 1)
	if err := p.SetDockPanelWheelPolicy(DockPanelWheelPolicy{Session: 1, Revision: 1, Enabled: true}, func(e DockPanelWheelEvent) { got <- e }); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	select {
	case <-closeEntered:
	case <-time.After(time.Second):
		t.Fatal("native close did not start")
	}
	p.wakeWheel()
	select {
	case <-emptyDuringClose:
	case <-time.After(time.Second):
		t.Fatal("worker did not poll before native close completed")
	}
	select {
	case <-p.Done():
		t.Fatal("drain completed before native cancellation was available")
	case <-time.After(30 * time.Millisecond):
	}
	release()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("native close did not finish")
	}
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("worker did not drain closed mailbox")
	}
	select {
	case e := <-got:
		if e.GestureID != 3 || e.Phase != "cancelled" {
			t.Fatalf("wrong terminal event: %+v", e)
		}
	default:
		t.Fatal("native cancellation was discarded")
	}
}

func TestDarwinPanelWheelFirstPolicyCloseDrainsOwnedEvents(t *testing.T) {
	f := &fakePanelWheel{events: make(chan DockPanelWheelEvent, 2)}
	p := newDarwinWheelPanel(newDockPanel(1, f), f)
	f.hook = func() {
		f.events <- DockPanelWheelEvent{Session: 1, Revision: 1, GestureID: 3, Phase: "began"}
		f.events <- DockPanelWheelEvent{Session: 1, Revision: 1, GestureID: 3, Phase: "cancelled"}
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	}
	got := make(chan DockPanelWheelEvent, 2)
	if err := p.SetDockPanelWheelPolicy(DockPanelWheelPolicy{Session: 1, Revision: 1, Enabled: true}, func(e DockPanelWheelEvent) { got <- e }); err == nil {
		t.Fatal("retired first policy was admitted")
	}
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("closed first policy did not drain")
	}
	for _, phase := range []string{"began", "cancelled"} {
		select {
		case e := <-got:
			if e.Phase != phase {
				t.Fatalf("event order: got %q want %q", e.Phase, phase)
			}
		default:
			t.Fatalf("owned %s event was discarded before worker start", phase)
		}
	}
}

// This invokes only an isolated Objective-C property/queue seam: no windows,
// event taps, pointer movement, foreground changes, or actions.
func TestDarwinPanelWheelNativeSeam(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "panel-wheel")
	compile := exec.CommandContext(ctx, "clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "ApplicationServices", "testdata/panel-wheel/main.m", "-o", binary)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile native seam: %v\n%s", err, output)
	}
	if output, err := exec.CommandContext(ctx, binary).CombinedOutput(); err != nil {
		t.Fatalf("native seam: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

//go:build darwin

package platform

import (
	"runtime"
	"testing"
)

// These invoke an isolated callback directly. No tap is installed, no event is
// posted, and the test handle cannot access or mutate any desktop AX window.
func TestNativePreviewDragCallback(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	n, err := newNativePreviewDrag(true)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	if n.testEvent(5, 3, 4, 0) {
		t.Fatal("unowned mouse move suppressed")
	}
	if !n.testEvent(6, 100, 200, 0) || !n.testEvent(6, 300, 400, 0) {
		t.Fatal("drag not owned")
	}
	s, _ := n.Poll()
	if s.Sequence != 2 || s.X != 300 || s.Y != 400 {
		t.Fatalf("latest sample: %+v", s)
	}
	if !n.testEvent(2, -100, 500, 0) {
		t.Fatal("release not owned")
	}
	if n.testEvent(6, 999, 999, 0) {
		t.Fatal("next drag swallowed after release")
	}
	s, _ = n.Poll()
	if !s.Released || s.X != -100 || s.Y != 500 {
		t.Fatalf("release overwritten: %+v", s)
	}
	if err := n.SetPosition(-300, 200); err != nil {
		t.Fatal(err)
	}
	n.Cancel()
	if err := n.SetPosition(1, 2); err == nil {
		t.Fatal("write after cancellation")
	}
	if n.testWrites() != 1 {
		t.Fatal("late native mutation")
	}
}

func TestNativePreviewDragCancellationAndEarlyRelease(t *testing.T) {
	for _, kind := range []string{"escape", "timeout", "released", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			n, err := newNativePreviewDrag(true)
			if err != nil {
				t.Fatal(err)
			}
			defer n.Close()
			switch kind {
			case "escape":
				if !n.testEvent(10, 0, 0, 53) {
					t.Fatal("escape passed")
				}
			case "timeout":
				n.testEvent(-2, 0, 0, 0)
			case "released":
				n.testButton(false)
			case "cancelled":
				n.Cancel()
			}
			if err := n.Start(); err == nil {
				t.Fatal("capture admitted cancelled or released gesture")
			}
			if n.testWrites() != 0 {
				t.Fatal("mutated window before admission")
			}
		})
	}
}

func TestNativePreviewDragMissingReleaseCancels(t *testing.T) {
	for _, ownedUp := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "owned"}[ownedUp], func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			n, err := newNativePreviewDrag(true)
			if err != nil {
				t.Fatal(err)
			}
			defer n.Close()
			if ownedUp {
				n.testEvent(2, 20, 30, 0)
			} else if n.testSyntheticUp() {
				t.Fatal("synthetic up suppressed")
			}
			n.testButton(false)
			sample, _ := n.Poll()
			if sample.Cancelled == ownedUp {
				t.Fatalf("wrong terminal state %+v", sample)
			}
			err = n.SetPosition(20, 30)
			if (err == nil) != ownedUp {
				t.Fatalf("wrong terminal write admission %v", err)
			}
		})
	}
}

func TestNativePreviewDragTopologyGaps(t *testing.T) {
	for _, phase := range []string{"registration", "poll", "write"} {
		t.Run(phase, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			n, err := newNativePreviewDrag(true)
			if err != nil {
				t.Fatal(err)
			}
			defer n.Close()
			if phase == "registration" {
				n.testScreens(1, 2)
				if n.Start() == nil {
					t.Fatal("topology changed during registration admitted")
				}
				return
			}
			n.testScreens(2, 0)
			if phase == "poll" {
				s, _ := n.Poll()
				if !s.Cancelled {
					t.Fatal("missed topology callback leaves owner alive")
				}
			}
			if n.SetPosition(1, 2) == nil {
				t.Fatal("write admitted after topology change")
			}
		})
	}
}

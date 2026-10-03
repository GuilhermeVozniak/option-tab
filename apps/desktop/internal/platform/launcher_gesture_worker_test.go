package platform

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"option-tab/internal/domain"
)

type gestureWorkerFixture struct {
	events           chan LauncherGestureEvent
	entered, release chan struct{}
	once             sync.Once
}

func (f *gestureWorkerFixture) Policy(uint64, LauncherGesturePolicy, func() bool) error { return nil }

func (f *gestureWorkerFixture) Next(uint64) (LauncherGestureEvent, int) {
	if f.entered != nil {
		f.once.Do(func() { close(f.entered); <-f.release })
	}
	select {
	case e := <-f.events:
		return e, 1
	default:
		return LauncherGestureEvent{}, 0
	}
}

func (*gestureWorkerFixture) Valid(uint64, uint64, uint64, uint64, uint64, uint64) bool { return true }
func (*gestureWorkerFixture) Complete(uint64, uint64, uint64, uint64, uint64, uint64)   {}
func workerPolicy() LauncherGesturePolicy {
	return LauncherGesturePolicy{Epoch: 1, Session: 1, Revision: 1, Admission: 1, DisplayUUID: "fixture", Enabled: true, Scroll: true, Bounds: domain.Bounds{W: 100, H: 100}}
}

func workerEvent() LauncherGestureEvent {
	return LauncherGestureEvent{Epoch: 1, Session: 1, Revision: 1, Admission: 1, DisplayUUID: "fixture", Owned: true, Sequence: 1, GestureID: 1}
}

func TestLauncherGestureWorkerRetireFetched(t *testing.T) {
	f := &gestureWorkerFixture{events: make(chan LauncherGestureEvent, 1), entered: make(chan struct{}), release: make(chan struct{})}
	f.events <- workerEvent()
	w := newLauncherGestureWorker(1, f)
	emits := make(chan struct{}, 1)
	if err := w.set(workerPolicy(), func(LauncherGestureEvent) { emits <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	<-f.entered
	w.retire(true)
	select {
	case <-w.done:
		t.Fatal("closed before native Next joined")
	default:
	}
	close(f.release)
	select {
	case <-w.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not join")
	}
	select {
	case <-emits:
		t.Fatal("retired Next delivered")
	default:
	}
}

func TestLauncherGestureWorkerCallbackDrain(t *testing.T) {
	f := &gestureWorkerFixture{events: make(chan LauncherGestureEvent, 1)}
	f.events <- workerEvent()
	w := newLauncherGestureWorker(1, f)
	entered, release := make(chan struct{}), make(chan struct{})
	if err := w.set(workerPolicy(), func(LauncherGestureEvent) { close(entered); <-release }); err != nil {
		t.Fatal(err)
	}
	<-entered
	w.retire(true)
	select {
	case <-w.done:
		t.Fatal("callback still running")
	default:
	}
	close(release)
	select {
	case <-w.done:
	case <-time.After(time.Second):
		t.Fatal("callback not joined")
	}
	if err := w.set(workerPolicy(), func(LauncherGestureEvent) {}); err == nil {
		t.Fatal("closed accepted")
	}
}

func TestLauncherGestureWorkerInvalidPolicy(t *testing.T) {
	w := newLauncherGestureWorker(1, &gestureWorkerFixture{})
	if err := w.set(workerPolicy(), nil); err == nil {
		t.Fatal("nil callback")
	}
	w.retire(true)
	<-w.done
}

type gesturePolicyFixture struct {
	gestureWorkerFixture
	fail bool
}

func (f *gesturePolicyFixture) Policy(uint64, LauncherGesturePolicy, func() bool) error {
	if f.fail {
		return errors.New("fixture refusal")
	}
	return nil
}

func TestLauncherGestureWorkerPolicyDuringBlockedNext(t *testing.T) {
	for _, mode := range []string{"identical", "failed", "newPacket"} {
		t.Run(mode, func(t *testing.T) {
			f := &gesturePolicyFixture{gestureWorkerFixture: gestureWorkerFixture{events: make(chan LauncherGestureEvent, 1), entered: make(chan struct{}), release: make(chan struct{})}}
			w := newLauncherGestureWorker(1, f)
			got := make(chan LauncherGestureEvent, 2)
			emit := func(e LauncherGestureEvent) { got <- e }
			p := workerPolicy()
			if err := w.set(p, emit); err != nil {
				t.Fatal(err)
			}
			<-f.entered
			event := workerEvent()
			next := p
			switch mode {
			case "identical":
			case "failed":
				f.fail = true
				next.Admission++
			case "newPacket":
				next.Admission++
				event.Admission++
			}
			err := w.set(next, emit)
			if (err != nil) != (mode == "failed") {
				t.Fatal(err)
			}
			f.events <- event
			close(f.release)
			select {
			case delivered := <-got:
				if delivered.Admission != event.Admission {
					t.Fatal(delivered)
				}
			case <-time.After(time.Second):
				w.retire(true)
				<-w.done
				t.Fatal("current owned event dropped")
			}
			w.retire(true)
			<-w.done
			select {
			case <-got:
				t.Fatal("duplicate delivery")
			default:
			}
		})
	}
}

type gestureLatePolicyFixture struct {
	gestureWorkerFixture
	prepared, resume chan struct{}
	installed        bool
}

func (f *gestureLatePolicyFixture) Policy(_ uint64, _ LauncherGesturePolicy, current func() bool) error {
	close(f.prepared)
	<-f.resume
	if !current() {
		return ErrDockPanelClosed
	}
	f.installed = true
	return nil
}

func TestLauncherGestureWorkerRetiredSetterCannotInstallAfterReopen(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint("close=", terminal), func(t *testing.T) {
			f := &gestureLatePolicyFixture{prepared: make(chan struct{}), resume: make(chan struct{})}
			w := newLauncherGestureWorker(1, f)
			result := make(chan error, 1)
			go func() { result <- w.set(workerPolicy(), func(LauncherGestureEvent) {}) }()
			<-f.prepared
			w.retire(terminal)
			close(f.resume)
			if err := <-result; err == nil {
				t.Fatal("retired setter accepted")
			}
			if f.installed {
				t.Fatal("retired setter enabled native suppression after reopen")
			}
			w.retire(true)
			<-w.done
		})
	}
}

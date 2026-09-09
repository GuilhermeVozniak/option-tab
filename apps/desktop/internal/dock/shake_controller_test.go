package dock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type (
	shakeTestSource struct {
		calls    chan *shakeTestCall
		validate func(platform.WindowDragValidation) bool
	}
	shakeTestCall struct {
		emit      func(platform.WindowDragEvent)
		cancelled chan struct{}
		release   chan error
	}
)

func (s *shakeTestSource) ObserveWindowDrags(ctx context.Context, emit func(platform.WindowDragEvent)) error {
	c := &shakeTestCall{emit: emit, cancelled: make(chan struct{}), release: make(chan error, 1)}
	s.calls <- c
	select {
	case err := <-c.release:
		return err
	case <-ctx.Done():
		close(c.cancelled)
		return <-c.release
	}
}

func (s *shakeTestSource) WindowDragGestureCurrent(v platform.WindowDragValidation) bool {
	if s.validate != nil {
		return s.validate(v)
	}
	return true
}

func newShakeTestSource() *shakeTestSource {
	return &shakeTestSource{calls: make(chan *shakeTestCall, 8)}
}

func shakeAwaitCall(t *testing.T, s *shakeTestSource) *shakeTestCall {
	t.Helper()
	select {
	case c := <-s.calls:
		return c
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("source did not start")
		return nil
	}
}

func startShakeOwner(t *testing.T, c *ShakeController) (context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	t.Cleanup(cancel)
	return cancel, done
}

func shakeSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("signal did not arrive")
	}
}

func emitShake(c *shakeTestCall, generation, gesture uint64, app int) {
	base := time.Now()
	for i, x := range []float64{0, 40, 80, 40, 80, 40, 80, 40} {
		kind := "moved"
		if i == 0 {
			kind = "candidate"
		}
		c.emit(platform.WindowDragEvent{Generation: generation, GestureID: gesture, Sequence: gesture*100 + uint64(i) + 1, Timestamp: base.Add(time.Duration(i) * 30 * time.Millisecond), Kind: kind, Window: platform.WindowRole{WindowID: 10, AppID: domain.AppID(app), Role: "AXWindow", Subrole: "AXStandardWindow"}, PointerX: x, WindowX: x})
	}
}

func TestShakeOwnerDisabledJoinedReconfigurationAndOldCallbacks(t *testing.T) {
	s := newShakeTestSource()
	actions := make(chan ShakeIntent, 4)
	c := NewShakeController(ShakeControllerDeps{Source: s, Execute: func(i ShakeIntent, g func() error) error {
		if err := g(); err != nil {
			return err
		}
		actions <- i
		return nil
	}})
	cancel, done := startShakeOwner(t, c)
	c.Configure(true, "none")
	select {
	case <-s.calls:
		t.Fatal("disabled source started")
	case <-time.After(25 * time.Millisecond):
	}
	c.Configure(true, "minimizeOthers")
	first := shakeAwaitCall(t, s)
	c.Configure(true, "closeOthers")
	shakeSignal(t, first.cancelled)
	c.Configure(false, "none")
	c.Configure(true, "closeOthers")
	select {
	case <-s.calls:
		t.Fatal("overlapping native sources")
	case <-time.After(25 * time.Millisecond):
	}
	first.release <- nil
	second := shakeAwaitCall(t, s)
	emitShake(first, 999, 1, 20)
	emitShake(second, 1, 1, 20)
	select {
	case i := <-actions:
		if i.Generation != 1 || i.Kind != "closeOthers" {
			t.Fatalf("stale source/action %+v", i)
		}
	case <-time.After(time.Second):
		t.Fatal("current gesture did not act")
	}
	emitShake(second, 1, 1, 20)
	select {
	case <-actions:
		t.Fatal("gesture fired twice")
	case <-time.After(25 * time.Millisecond):
	}
	cancel()
	shakeSignal(t, second.cancelled)
	second.release <- nil
	shakeSignal(t, done)
}

func TestShakeOwnerValidationAndBlockedLookupCancellation(t *testing.T) {
	for _, blockedValidator := range []bool{false, true} {
		t.Run(map[bool]string{false: "executor", true: "validator"}[blockedValidator], func(t *testing.T) {
			s := newShakeTestSource()
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			dispatched := make(chan struct{}, 1)
			if blockedValidator {
				s.validate = func(platform.WindowDragValidation) bool { once.Do(func() { close(entered) }); <-release; return true }
			}
			c := NewShakeController(ShakeControllerDeps{Source: s, Execute: func(_ ShakeIntent, g func() error) error {
				if !blockedValidator {
					once.Do(func() { close(entered) })
					<-release
				}
				if err := g(); err != nil {
					return err
				}
				dispatched <- struct{}{}
				return nil
			}})
			cancel, done := startShakeOwner(t, c)
			c.Configure(true, "minimizeOthers")
			call := shakeAwaitCall(t, s)
			emitShake(call, 1, 1, 20)
			shakeSignal(t, entered)
			c.Configure(false, "none")
			shakeSignal(t, call.cancelled)
			call.release <- nil
			close(release)
			select {
			case <-dispatched:
				t.Fatal("retired gesture dispatched")
			case <-time.After(30 * time.Millisecond):
			}
			cancel()
			shakeSignal(t, done)
		})
	}
}

func TestShakeOwnerSourceErrorRecoveryAcrossPolicyChange(t *testing.T) {
	s := newShakeTestSource()
	status := make(chan error, 8)
	c := NewShakeController(ShakeControllerDeps{Source: s, SourceFailed: func(e error) { status <- e }})
	cancel, done := startShakeOwner(t, c)
	c.Configure(true, "minimizeOthers")
	a := shakeAwaitCall(t, s)
	a.release <- errors.New("permission")
	select {
	case err := <-status:
		if err == nil || err.Error() != "permission" {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("missing source error")
	}
	c.Configure(true, "closeOthers")
	b := shakeAwaitCall(t, s)
	b.emit(platform.WindowDragEvent{Generation: 2, Sequence: 1, Kind: "cancelled"})
	select {
	case <-status:
		t.Fatal("invalid cancellation claimed recovery")
	case <-time.After(20 * time.Millisecond):
	}
	emitShake(b, 2, 1, 20)
	select {
	case err := <-status:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("missing recovery after policy change")
	}
	cancel()
	shakeSignal(t, b.cancelled)
	b.release <- nil
	shakeSignal(t, done)
}

func TestShakeOwnerSamplingContinuesWithBoundedJobsAndSelfExcluded(t *testing.T) {
	s := newShakeTestSource()
	entered, release := make(chan struct{}), make(chan struct{})
	failures := make(chan error, 8)
	var once sync.Once
	c := NewShakeController(ShakeControllerDeps{Source: s, SelfAppID: 20, Failed: func(_ ShakeIntent, e error) {
		select {
		case failures <- e:
		default:
		}
	}, Execute: func(_ ShakeIntent, g func() error) error {
		once.Do(func() { close(entered) })
		<-release
		if err := g(); err != nil {
			return err
		}
		return errors.New("native action refused")
	}})
	cancel, done := startShakeOwner(t, c)
	c.Configure(true, "closeOthers")
	call := shakeAwaitCall(t, s)
	emitShake(call, 1, 1, 20)
	select {
	case <-entered:
		t.Fatal("self window acted")
	case <-time.After(20 * time.Millisecond):
	}
	emitShake(call, 1, 2, 30)
	shakeSignal(t, entered)
	produced := make(chan struct{})
	go func() {
		for gesture := uint64(3); gesture < 30; gesture++ {
			emitShake(call, 1, gesture, 30)
		}
		close(produced)
	}()
	// Failure callback is bounded/nonblocking too; drain without allowing a slow
	// executor to stop source ingestion.
	observed := 0
	deadline := time.After(time.Second)
drain:
	for {
		select {
		case <-produced:
			break drain
		case <-failures:
			observed++
		case <-deadline:
			t.Fatal("slow action blocked passive sampling")
		}
	}
	if observed == 0 {
		select {
		case <-failures:
		case <-time.After(time.Second):
			t.Fatal("overflow was silent")
		}
	}
	c.Configure(false, "none")
	shakeSignal(t, call.cancelled)
	call.release <- nil
	close(release)
	cancel()
	shakeSignal(t, done)
}

func TestShakeOwnerRetryDistinctErrorsAndReenabledFailure(t *testing.T) {
	s := newShakeTestSource()
	status := make(chan error, 8)
	c := NewShakeController(ShakeControllerDeps{Source: s, SourceFailed: func(e error) { status <- e }})
	cancel, done := startShakeOwner(t, c)
	c.Configure(true, "closeOthers")
	first := shakeAwaitCall(t, s)
	at := time.Now()
	first.release <- errors.New("permission")
	select {
	case <-status:
	case <-time.After(time.Second):
		t.Fatal("missing error")
	}
	second := shakeAwaitCall(t, s)
	if time.Since(at) < 900*time.Millisecond {
		t.Fatal("retry spun")
	}
	second.release <- errors.New("tap unavailable")
	select {
	case err := <-status:
		if err == nil || err.Error() != "tap unavailable" {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("distinct error hidden")
	}
	c.Configure(false, "none")
	c.Configure(true, "closeOthers")
	third := shakeAwaitCall(t, s)
	third.release <- errors.New("tap unavailable")
	select {
	case err := <-status:
		if err == nil || err.Error() != "tap unavailable" {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reenabled failure hidden")
	}
	cancel()
	shakeSignal(t, done)
}

func TestShakeOwnerPassesExactEvidenceAndReportsActionFailure(t *testing.T) {
	s := newShakeTestSource()
	validated := make(chan platform.WindowDragValidation, 8)
	s.validate = func(v platform.WindowDragValidation) bool { validated <- v; return true }
	failed := make(chan ShakeIntent, 1)
	c := NewShakeController(ShakeControllerDeps{Source: s, Execute: func(_ ShakeIntent, guard func() error) error {
		if err := guard(); err != nil {
			return err
		}
		return errors.New("close refused")
	}, Failed: func(i ShakeIntent, err error) {
		if err.Error() == "close refused" {
			failed <- i
		}
	}})
	cancel, done := startShakeOwner(t, c)
	c.Configure(true, "closeOthers")
	call := shakeAwaitCall(t, s)
	emitShake(call, 7, 3, 42)
	select {
	case i := <-failed:
		v := <-validated
		if v.Generation != 7 || v.GestureID != 3 || v.WindowID != 10 || v.AppID != 42 || v.WindowX != i.WindowX || !v.ObservedAt.Equal(i.Timestamp) {
			t.Fatalf("wrong immutable evidence %+v %+v", v, i)
		}
	case <-time.After(time.Second):
		t.Fatal("action refusal was silent")
	}
	cancel()
	shakeSignal(t, call.cancelled)
	call.release <- nil
	shakeSignal(t, done)
}

func TestShakeOwnerRetiredSourceCannotOverwriteDisabledStatus(t *testing.T) {
	s := newShakeTestSource()
	status := make(chan error, 2)
	c := NewShakeController(ShakeControllerDeps{Source: s, SourceFailed: func(e error) { status <- e }})
	cancel, done := startShakeOwner(t, c)
	c.Configure(true, "closeOthers")
	call := shakeAwaitCall(t, s)
	c.Configure(false, "none")
	shakeSignal(t, call.cancelled)
	call.release <- errors.New("retired permission error")
	select {
	case e := <-status:
		t.Errorf("disabled source published stale error: %v", e)
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	shakeSignal(t, done)
}

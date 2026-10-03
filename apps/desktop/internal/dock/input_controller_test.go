package dock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type inputSourceFixture struct {
	mu                        sync.Mutex
	emit                      func(platform.DockInputEvent)
	starts, active, maxActive int
	generation, gesture       uint64
	pending                   uint64
	passed                    int
}

func (s *inputSourceFixture) ObserveDockInput(ctx context.Context, _ platform.DockInputPolicy, targets <-chan platform.DockInputTarget, emit func(platform.DockInputEvent)) error {
	s.mu.Lock()
	s.starts++
	s.active++
	s.maxActive = max(s.maxActive, s.active)
	s.emit = emit
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.emit = nil; s.mu.Unlock() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-targets:
			if !ok {
				return nil
			}
		}
	}
}

func (s *inputSourceFixture) DockInputGestureCurrent(generation, gesture uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active > 0 && s.generation == generation && s.gesture == gesture
}

func (s *inputSourceFixture) send(e platform.DockInputEvent) {
	s.mu.Lock()
	if s.pending != 0 && e.GestureID != s.pending {
		if e.Kind == platform.DockInputLeftDown {
			s.passed++
		}
		s.mu.Unlock()
		return
	}
	if e.Kind == platform.DockInputLeftUp && e.Owned {
		s.pending = e.GestureID
	}
	s.generation = e.Generation
	s.gesture = e.GestureID
	if e.Kind == platform.DockInputCancelled {
		s.gesture = 0
		s.pending = 0
	}
	emit := s.emit
	s.mu.Unlock()
	if emit != nil {
		emit(e)
	}
}

func waitInput(t *testing.T, f func() bool) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		if f() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("input condition timed out")
}

func inputTargetFixture() platform.DockInputTarget {
	return platform.DockInputTarget{Generation: 9, DockPID: 22, ObservedAt: time.Now(), Item: &platform.DockItem{AppID: 10, BundleID: "test.app", Path: "/Test.app", Bounds: domain.Bounds{X: 100, Y: 100, W: 50, H: 50}, ScreenID: 1, Edge: "bottom"}}
}

func sendInputClick(s *inputSourceFixture, target platform.DockInputTarget, gesture uint64) {
	at := time.Now()
	for i, kind := range []platform.DockInputKind{platform.DockInputLeftDown, platform.DockInputLeftUp} {
		s.send(platform.DockInputEvent{Sequence: gesture*2 + uint64(i), Generation: target.Generation, GestureID: gesture, Timestamp: at.Add(time.Duration(i) * time.Millisecond), Kind: kind, Item: *target.Item, PointerX: 110, PointerY: 110, Owned: true, DockPID: target.DockPID})
	}
}

func TestInputControllerDisabledAndJoinedReconfiguration(t *testing.T) {
	source := &inputSourceFixture{}
	c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	target := inputTargetFixture()
	c.Target(target)
	source.mu.Lock()
	starts := source.starts
	source.mu.Unlock()
	if starts != 0 {
		t.Fatal("disabled input installed source")
	}
	c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.starts == 1 })
	c.Configure(true, platform.DockInputPolicy{ScrollShowHide: true})
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.starts == 2 })
	source.mu.Lock()
	peak := source.maxActive
	source.mu.Unlock()
	if peak != 1 {
		t.Fatalf("overlapping sources: %d", peak)
	}
	c.Configure(false, platform.DockInputPolicy{})
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.active == 0 })
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("source did not join")
	}
}

func TestInputControllerExactActionAndNativeCancellationGuard(t *testing.T) {
	for _, invalidate := range []string{"native cancellation", "disable", "new generation", "pointer moved", "none"} {
		t.Run(invalidate, func(t *testing.T) {
			source := &inputSourceFixture{}
			started := make(chan InputAction, 1)
			release := make(chan struct{})
			result := make(chan error, 1)
			c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99, Execute: func(action InputAction, guard func() error) error {
				started <- action
				<-release
				err := guard()
				result <- err
				return err
			}})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go c.Run(ctx)
			target := inputTargetFixture()
			c.Target(target)
			c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
			waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.emit != nil })
			sendInputClick(source, target, 1)
			select {
			case action := <-started:
				if action.Intent.AppID != 10 || action.Intent.Kind != "hide" || action.Item.Path != "/Test.app" {
					t.Fatalf("wrong immutable action: %+v", action)
				}
			case <-time.After(time.Second):
				t.Fatal("missing action")
			}
			switch invalidate {
			case "native cancellation":
				source.send(platform.DockInputEvent{Sequence: 4, Generation: 9, GestureID: 1, Timestamp: time.Now(), Kind: platform.DockInputCancelled})
			case "disable":
				c.Configure(false, platform.DockInputPolicy{})
			case "new generation":
				next := inputTargetFixture()
				next.Generation++
				c.Target(next)
			case "pointer moved":
				next := inputTargetFixture()
				next.Item = nil
				c.Target(next)
			}
			close(release)
			select {
			case err := <-result:
				if (err == nil) != (invalidate == "none" || invalidate == "pointer moved") {
					t.Fatalf("invalidation %s returned %v", invalidate, err)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked executor")
			}
		})
	}
}

func (s *inputSourceFixture) CompleteDockInputGesture(generation, gesture uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generation == generation && s.gesture == gesture {
		s.gesture = 0
		s.pending = 0
	}
}

type failingInputFixture struct {
	inputSourceFixture
	calls atomic.Int32
}

func (s *failingInputFixture) ObserveDockInput(ctx context.Context, p platform.DockInputPolicy, targets <-chan platform.DockInputTarget, emit func(platform.DockInputEvent)) error {
	if s.calls.Add(1) == 1 {
		return errors.New("Accessibility denied")
	}
	return s.inputSourceFixture.ObserveDockInput(ctx, p, targets, emit)
}

func TestInputControllerReportsSourceFailureAndRecovery(t *testing.T) {
	for _, reconfigure := range []bool{false, true} {
		t.Run(map[bool]string{false: "retry", true: "policy change"}[reconfigure], func(t *testing.T) {
			source := &failingInputFixture{}
			notices := make(chan error, 4)
			c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99, SourceFailed: func(err error) { notices <- err }})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			target := inputTargetFixture()
			c.Target(target)
			c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
			go c.Run(ctx)
			select {
			case err := <-notices:
				if err == nil || err.Error() != "Accessibility denied" {
					t.Fatalf("missing native refusal: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("missing native source failure")
			}
			if reconfigure {
				c.Configure(true, platform.DockInputPolicy{ClickToHide: true, ScrollShowHide: true})
			}
			until := time.Now().Add(3 * time.Second)
			for source.calls.Load() < 2 && time.Now().Before(until) {
				time.Sleep(time.Millisecond)
			}
			if source.calls.Load() < 2 {
				t.Fatal("source did not retry")
			}
			waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.emit != nil })
			sendInputClick(&source.inputSourceFixture, target, 1)
			select {
			case err := <-notices:
				if err != nil {
					t.Fatalf("recovery notice: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("source recovery was not reported")
			}
		})
	}
}

type heldInputSource struct {
	starts           atomic.Int32
	leaving, release chan struct{}
}

func (s *heldInputSource) ObserveDockInput(ctx context.Context, _ platform.DockInputPolicy, _ <-chan platform.DockInputTarget, _ func(platform.DockInputEvent)) error {
	s.starts.Add(1)
	<-ctx.Done()
	close(s.leaving)
	<-s.release
	return nil
}

func TestInputControllerCoalescesConfigurationDuringSourceRetirement(t *testing.T) {
	source := &heldInputSource{leaving: make(chan struct{}), release: make(chan struct{})}
	c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
	go func() { c.Run(ctx); close(done) }()
	waitInput(t, func() bool { return source.starts.Load() == 1 })
	c.Configure(true, platform.DockInputPolicy{ScrollShowHide: true})
	select {
	case <-source.leaving:
	case <-time.After(time.Second):
		t.Fatal("source retirement not begun")
	}
	c.Configure(true, platform.DockInputPolicy{ModifiedRightClick: true})
	c.Configure(false, platform.DockInputPolicy{})
	close(source.release)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("source retirement did not finish")
	}
	if source.starts.Load() != 1 {
		t.Fatal("an intermediate configuration installed a source")
	}
}

func TestInputControllerAcknowledgesPendingActionBeforeNewAdmission(t *testing.T) {
	source := &inputSourceFixture{}
	started := make(chan uint64, 3)
	release := make(chan struct{})
	c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99, Execute: func(action InputAction, guard func() error) error {
		started <- action.Intent.GestureID
		<-release
		return guard()
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	target := inputTargetFixture()
	c.Target(target)
	c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
	go c.Run(ctx)
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.emit != nil })
	sendInputClick(source, target, 1)
	select {
	case id := <-started:
		if id != 1 {
			t.Fatal("wrong first gesture")
		}
	case <-time.After(time.Second):
		t.Fatal("missing first action")
	}
	sendInputClick(source, target, 2)
	sendInputClick(source, target, 3)
	source.mu.Lock()
	passed, pending := source.passed, source.pending
	source.mu.Unlock()
	if passed != 2 || pending != 1 {
		t.Fatalf("native backpressure lost pending action: passed=%d pending=%d", passed, pending)
	}
	close(release)
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.pending == 0 })
	sendInputClick(source, target, 4)
	select {
	case id := <-started:
		if id != 4 {
			t.Fatalf("unexpected queued/replaced gesture %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("action completion did not release native admission")
	}
}

type blockingInputValidator struct {
	inputSourceFixture
	entered, release chan struct{}
}

func (s *blockingInputValidator) DockInputGestureCurrent(uint64, uint64) bool {
	close(s.entered)
	<-s.release
	return true
}

func TestInputControllerRechecksAdmissionAfterNativeValidation(t *testing.T) {
	for _, change := range []string{"disable", "generation", "context"} {
		t.Run(change, func(t *testing.T) {
			source := &blockingInputValidator{entered: make(chan struct{}), release: make(chan struct{})}
			c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99})
			target := inputTargetFixture()
			c.Target(target)
			c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
			job := inputJob{configuration: c.snapshot().epoch, generation: target.Generation, dockPID: target.DockPID, action: InputAction{Intent: Intent{GestureID: 1}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- c.guard(ctx, job) }()
			<-source.entered
			switch change {
			case "disable":
				c.Configure(false, platform.DockInputPolicy{})
			case "generation":
				target.Generation++
				c.Target(target)
			case "context":
				cancel()
			}
			close(source.release)
			if err := <-result; !errors.Is(err, ErrInputRetired) {
				t.Fatalf("admitted action after %s during native validation: %v", change, err)
			}
		})
	}
}

func TestInputControllerAcknowledgesRejectedOwnedClick(t *testing.T) {
	source := &inputSourceFixture{}
	var executed atomic.Int32
	c := NewInputController(InputControllerDeps{Source: source, SelfAppID: 99, Execute: func(InputAction, func() error) error {
		executed.Add(1)
		return nil
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	target := inputTargetFixture()
	c.Target(target)
	c.Configure(true, platform.DockInputPolicy{ClickToHide: true})
	go c.Run(ctx)
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.emit != nil })
	at := time.Now()
	source.send(platform.DockInputEvent{Sequence: 1, Generation: 9, GestureID: 1, Timestamp: at, Kind: platform.DockInputLeftDown, Item: *target.Item, PointerX: 149, PointerY: 110, Owned: true, DockPID: 22})
	// Dock magnification may move the current rectangle while the owned event
	// deliberately retains the original target. Rejection still releases input.
	source.send(platform.DockInputEvent{Sequence: 2, Generation: 9, GestureID: 1, Timestamp: at.Add(time.Millisecond), Kind: platform.DockInputLeftUp, Item: *target.Item, PointerX: 151, PointerY: 110, Owned: true, DockPID: 22})
	waitInput(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.pending == 0 })
	if executed.Load() != 0 {
		t.Fatal("rejected click executed an action")
	}
}

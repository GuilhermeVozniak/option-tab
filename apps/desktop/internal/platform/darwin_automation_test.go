//go:build darwin

package platform

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type automationFakeNative struct {
	mu      sync.Mutex
	queue   []automationPacket
	live    map[uint64]bool
	started chan struct{}
	closed  bool
	replies []AutomationReply
}

func (n *automationFakeNative) start() uint64 { close(n.started); return 1 }
func (n *automationFakeNative) status(uint64) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return -1
	}
	return 1
}

func (n *automationFakeNative) pop(uint64) *automationPacket {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.queue) == 0 {
		return nil
	}
	p := n.queue[0]
	n.queue = n.queue[1:]
	return &p
}

func (n *automationFakeNative) current(_, id uint64) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return !n.closed && n.live[id]
}

func (n *automationFakeNative) complete(_, id uint64, r AutomationReply) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.live[id] && !n.closed {
		n.replies = append(n.replies, r)
		delete(n.live, id)
	}
}
func (n *automationFakeNative) stop(uint64)     { n.mu.Lock(); n.closed = true; n.mu.Unlock() }
func (n *automationFakeNative) drain(id uint64) { n.stop(id) }
func automationFake() *automationFakeNative {
	return &automationFakeNative{live: map[uint64]bool{}, started: make(chan struct{})}
}

func automationWait(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

// Dropping the current-token check would allow a handler's final guard after native expiry.
func TestAutomationNativeRetirementImmediatelyInvalidatesHandlerContext(t *testing.T) {
	n := automationFake()
	n.live[7] = true
	n.queue = []automationPacket{{Request: AutomationRequest{ID: 7, Operation: AutomationQueryApps}, RemainingMS: 5000}}
	s := newAutomationServer(n)
	entered := make(chan struct{})
	resume := make(chan struct{})
	checked := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Run(context.Background(), func(ctx context.Context, _ AutomationRequest) AutomationReply {
			close(entered)
			<-resume
			if ctx.Err() == nil {
				t.Error("retired native request remained actionable")
			}
			close(checked)
			return AutomationReply{JSON: []byte(`{}`)}
		})
	}()
	automationWait(t, entered)
	n.mu.Lock()
	n.live[7] = false
	n.mu.Unlock()
	close(resume)
	automationWait(t, checked)
	s.Stop()
	automationWait(t, done)
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.replies) != 0 {
		t.Fatal("late reply published")
	}
}

func TestAutomationStopCancelsAndJoinsAdmittedWork(t *testing.T) {
	n := automationFake()
	n.live[2] = true
	n.queue = []automationPacket{{Request: AutomationRequest{ID: 2}, RemainingMS: 5000}}
	s := newAutomationServer(n)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Run(context.Background(), func(ctx context.Context, _ AutomationRequest) AutomationReply {
			close(entered)
			<-ctx.Done()
			close(cancelled)
			<-release
			return AutomationReply{}
		})
	}()
	automationWait(t, entered)
	s.Stop()
	automationWait(t, cancelled)
	select {
	case <-done:
		t.Fatal("Run returned before handler joined")
	default:
	}
	close(release)
	automationWait(t, done)
	if err := s.Run(context.Background(), nil); err == nil {
		t.Fatal("terminal server restarted")
	}
}

func TestAutomationStopBeforeRunNeverStartsNative(t *testing.T) {
	n := automationFake()
	s := newAutomationServer(n)
	s.Stop()
	if err := s.Run(context.Background(), func(context.Context, AutomationRequest) AutomationReply { return AutomationReply{} }); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	select {
	case <-n.started:
		t.Fatal("native installed after terminal Stop")
	default:
	}
}

func TestAutomationNativeTransportFixture(t *testing.T) {
	for _, fixture := range []string{"main.m", "lease.m"} {
		t.Run(fixture, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "automation-transport")
			cmd := exec.Command("clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa", "-framework", "Carbon", filepath.Join("testdata", "automation-transport", fixture), "-o", binary)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("native fixture compile: %v\n%s", err, out)
			}
			if out, err := exec.Command(binary).CombinedOutput(); err != nil {
				t.Fatalf("native fixture: %v\n%s", err, out)
			} else {
				t.Log(string(out))
			}
		})
	}
}

func TestAutomationDispatchDelayCannotRestartReceiptBudget(t *testing.T) {
	n := automationFake()
	n.live[8] = true
	s := newAutomationServer(n)
	called := false
	s.execute(context.Background(), 1, automationPacket{Request: AutomationRequest{ID: 8}, RemainingMS: 5000, Deadline: time.Now().Add(-time.Millisecond)}, func(context.Context, AutomationRequest) AutomationReply {
		called = true
		return AutomationReply{JSON: []byte(`{}`)}
	})
	if called {
		t.Fatal("expired receipt budget restarted at worker dispatch")
	}
}

func TestAutomationReplyRejectsInvalidUTF8(t *testing.T) {
	n := automationFake()
	n.live[9] = true
	s := newAutomationServer(n)
	s.execute(context.Background(), 1, automationPacket{Request: AutomationRequest{ID: 9}, RemainingMS: 1000}, func(context.Context, AutomationRequest) AutomationReply {
		return AutomationReply{JSON: []byte{'"', 0xff, '"'}}
	})
	if len(n.replies) != 1 || n.replies[0].ErrorCode != "internal" {
		t.Fatalf("invalid native string passed as success: %+v", n.replies)
	}
}

func TestAutomationExpiredRequestPreservesDeadlineAndDone(t *testing.T) {
	n := automationFake()
	n.live[11] = true
	s := newAutomationServer(n)
	s.execute(context.Background(), 1, automationPacket{Request: AutomationRequest{ID: 11}, RemainingMS: 5000, Deadline: time.Now().Add(10 * time.Millisecond)}, func(ctx context.Context, _ AutomationRequest) AutomationReply {
		<-ctx.Done()
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Errorf("deadline became cancellation: %v", ctx.Err())
		}
		return AutomationReply{JSON: []byte(`{}`)}
	})
	if len(n.replies) != 0 {
		t.Fatal("expired handler published a reply")
	}
}

func TestAutomationCancelledParentDoesNotInstall(t *testing.T) {
	n := automationFake()
	s := newAutomationServer(n)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Run(ctx, func(context.Context, AutomationRequest) AutomationReply { return AutomationReply{} }); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	select {
	case <-n.started:
		t.Fatal("cancelled parent installed native automation")
	default:
	}
}

package automation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/platform"
)

// registryContext models a native suspension whose exact registry admission
// can retire before its polling Done bridge observes the change.
type registryContext struct {
	retired atomic.Bool
}

func (*registryContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*registryContext) Done() <-chan struct{}       { return nil }
func (c *registryContext) Err() error {
	if c.retired.Load() {
		return context.Canceled
	}
	return nil
}
func (*registryContext) Value(any) any { return nil }

func TestDerivedDeadlinePreservesExactNativeRequestAdmission(t *testing.T) {
	d, _, performer := fixture()
	registry := &registryContext{}
	performer.before = func() { registry.retired.Store(true) }
	r := request(platform.AutomationWindowAction)
	r.WindowID = 9
	r.Action = "close"
	got := New(d).Handle(registry, r)
	if got.ErrorCode != "cancelled" || performer.calls != 0 {
		t.Fatalf("retired native request dispatched through derived context: reply=%+v calls=%d", got, performer.calls)
	}
}

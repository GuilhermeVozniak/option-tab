package actions

import (
	"context"
	"errors"
	"testing"

	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type automationBackend struct {
	*fake.Fake
	dispatches int
	before     func()
}

func (b *automationBackend) PerformAutomationWindowAction(_ context.Context, kind string, id platform.AutomationWindowIdentity, fullscreen *bool, g func() error) error {
	if b.before != nil {
		b.before()
	}
	if err := g(); err != nil {
		return err
	}
	b.dispatches++
	return nil
}

func TestAutomationRequiresGuardedNativeCapability(t *testing.T) {
	id := platform.AutomationWindowIdentity{ID: 9, Process: platform.ProcessIdentity{PID: 7, StartSeconds: 1}}
	if err := New(fake.New()).PerformAutomationWindowAction(context.Background(), "minimize", id, nil, func() error { return nil }); err == nil {
		t.Fatal("legacy action fallback admitted")
	}
}

func TestAutomationNativePreparationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &automationBackend{Fake: fake.New(), before: cancel}
	id := platform.AutomationWindowIdentity{ID: 9, Process: platform.ProcessIdentity{PID: 7, StartSeconds: 1}}
	err := New(b).PerformAutomationWindowAction(ctx, "close", id, nil, func() error { return nil })
	if !errors.Is(err, context.Canceled) || b.dispatches != 0 {
		t.Fatalf("late native dispatch: %v", err)
	}
}

package switcher

import (
	"context"
	"errors"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type blockedWindows struct {
	source           platform.WindowSource
	entered, release chan struct{}
}

func (b *blockedWindows) Windows() ([]domain.Window, error) {
	close(b.entered)
	<-b.release
	return b.source.Windows()
}

func TestOpenIsIdempotentForRequestedMode(t *testing.T) {
	c, _, view := newController(t, threeWins(), func(s *config.Settings) {
		s.Behavior.HoldToCycle = false
		for i := range s.Shortcuts {
			s.Shortcuts[i].Enabled = false
		}
	})
	first, err := c.Open(config.ModeWindows)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance()
	selected := c.State().Selected
	second, err := c.Open(config.ModeWindows)
	if err != nil {
		t.Fatal(err)
	}
	if second.Session != first.Session || second.Selected != selected || len(view.shows) != 1 {
		t.Fatalf("idempotent open changed presentation: first=%+v second=%+v shows=%d", first, second, len(view.shows))
	}
}

func TestOpenReportsInvalidPausedAndEmpty(t *testing.T) {
	c, _, _ := newController(t, threeWins(), nil)
	if _, err := c.Open(config.SwitcherMode("bogus")); !errors.Is(err, ErrUnsupportedMode) {
		t.Fatalf("invalid mode error=%v", err)
	}
	c.SetPaused(true)
	if _, err := c.Open(config.ModeWindows); !errors.Is(err, ErrPaused) {
		t.Fatalf("paused error=%v", err)
	}
	empty, _, _ := newController(t, nil, nil)
	if _, err := empty.Open(config.ModeWindows); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty error=%v", err)
	}
}

func TestOpenChangesModeWithFreshScopedPresentation(t *testing.T) {
	apps, wins := appFixtures()
	c, _, view := newAppController(t, apps, wins, func(s *config.Settings) {
		s.Shortcuts[1].Scope.AppScope = config.AppScopeAll
	})
	appsState, err := c.Open(config.ModeApps)
	if err != nil {
		t.Fatal(err)
	}
	windowsState, err := c.Open(config.ModeWindows)
	if err != nil {
		t.Fatal(err)
	}
	if windowsState.Mode != config.ModeWindows || windowsState.Session <= appsState.Session || view.hides != 1 {
		t.Fatalf("mode replacement apps=%+v windows=%+v hides=%d", appsState, windowsState, view.hides)
	}
}

func TestOpenGuardedCancellationAfterBlockedInventoryPreservesPresentation(t *testing.T) {
	apps, wins := appFixtures()
	c, native, view := newAppController(t, apps, wins, func(s *config.Settings) {
		s.Shortcuts[1].Scope.AppScope = config.AppScopeAll
	})
	old, err := c.Open(config.ModeApps)
	if err != nil {
		t.Fatal(err)
	}
	barrier := &blockedWindows{source: native, entered: make(chan struct{}), release: make(chan struct{})}
	c.deps.Windows = barrier
	done := make(chan error, 1)
	go func() {
		_, openErr := c.OpenGuarded(config.ModeWindows, func() error { return context.Canceled })
		done <- openErr
	}()
	<-barrier.entered
	close(barrier.release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("guard error=%v", err)
	}
	current := c.State()
	if current.Session != old.Session || current.Mode != config.ModeApps || view.hides != 0 || len(view.shows) != 1 {
		t.Fatalf("cancelled replacement changed presentation: state=%+v hides=%d shows=%d", current, view.hides, len(view.shows))
	}
}

func TestOpenGuardedRevalidatesControllerAfterExternalGuard(t *testing.T) {
	apps, wins := appFixtures()
	c, _, view := newAppController(t, apps, wins, func(s *config.Settings) {
		s.Shortcuts[1].Scope.AppScope = config.AppScopeAll
	})
	old, err := c.Open(config.ModeApps)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.OpenGuarded(config.ModeWindows, func() error {
		c.SetPaused(true)
		return nil
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("changed admission error=%v", err)
	}
	if current := c.State(); current.Session != old.Session || current.Mode != config.ModeApps || view.hides != 0 {
		t.Fatalf("guard-time mutation replaced old presentation: %+v", current)
	}
}

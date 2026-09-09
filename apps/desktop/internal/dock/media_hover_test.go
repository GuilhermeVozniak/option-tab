package dock

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type noMediaWindows struct{}

func (noMediaWindows) Windows() ([]domain.Window, error) { panic("media hover enumerated windows") }
func mediaSettings(s *config.Settings) {
	s.Dock.Enabled = false
	s.Dock.Media = config.DockMediaSettings{Enabled: true, MusicEnabled: true}
	s.Dock.HoverDelayMs = 0
}

func musicIcon() *platform.DockItem {
	i := dockTestItem(0)
	i.Kind = "app"
	i.BundleID = "com.apple.Music"
	i.Title = "Music"
	return i
}

func TestMediaProviderForItemExactOptIn(t *testing.T) {
	s := config.DockMediaSettings{Enabled: true, MusicEnabled: true}
	for _, tc := range []struct {
		item Item
		want platform.MediaProvider
	}{{Item{Kind: "app", BundleID: "com.apple.Music"}, platform.MediaMusic}, {Item{BundleID: "com.apple.Music"}, platform.MediaMusic}, {Item{Kind: "folder", BundleID: "com.apple.Music"}, ""}, {Item{Kind: "app", BundleID: "COM.APPLE.MUSIC"}, ""}, {Item{Kind: "app", BundleID: "com.spotify.client"}, ""}} {
		if got := MediaProviderForItem(tc.item, s); got != tc.want {
			t.Fatalf("selector %+v=%s", tc.item, got)
		}
	}
}

func TestMediaHoverIndependentNoWindowsOrInput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, noMediaWindows{}, mediaSettings)
		h.observe(musicIcon(), 424, 750)
		s := h.view.last()
		if s.ContentKind != "media" || len(s.Windows) != 0 || s.Folder != nil || s.SelectedWindowID != 0 || s.Bounds.W != 360 || s.Bounds.H != 360 {
			t.Fatalf("media state %+v", s)
		}
		if target := h.view.targets[len(h.view.targets)-1]; target.Item != nil || target.Generation != 0 {
			t.Fatal("media icon granted window input")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if h.view.updateCount() != 0 {
			t.Fatal("periodic media window refresh")
		}
		h.c.Configure(config.Default())
		synctest.Wait()
		if h.view.hideCount() == 0 {
			t.Fatal("disable retained media")
		}
	})
}

func TestMediaHoverRejectsOthersAndExcluded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, noMediaWindows{}, mediaSettings)
		h.observe(dockTestItem(10), 424, 750)
		if h.view.showCount() != 0 {
			t.Fatal("ordinary windows enabled by media")
		}
		s := h.settings
		s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "com.apple.Music", Hide: config.HideAlways}}
		h.c.Configure(s)
		synctest.Wait()
		h.observe(musicIcon(), 424, 750)
		if h.view.showCount() != 0 {
			t.Fatal("blacklisted provider shown")
		}
	})
}

func TestMediaHoverLateWindowsAndGeometry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &blockingWindowSource{requests: make(chan chan []domain.Window, 4)}
		h := newDockHarness(t, source, func(s *config.Settings) { mediaSettings(s); s.Dock.Enabled = true })
		h.observe(dockTestItem(10), 424, 750)
		old := <-source.requests
		h.observe(musicIcon(), 424, 750)
		old <- []domain.Window{{ID: 101, AppID: 10}}
		synctest.Wait()
		state := h.view.last()
		if state.ContentKind != "media" || len(state.Windows) != 0 {
			t.Fatalf("late window replaced media %+v", state)
		}
		h.c.SetPanelBounds(state.Session, domain.Bounds{W: 500, H: 260})
		synctest.Wait()
		if s := h.view.last(); s.ContentKind != "media" || s.Bounds.W != 500 || s.Bounds.H != 260 {
			t.Fatalf("measured media %+v", s)
		}
		moved := musicIcon()
		moved.Bounds.X += 100
		h.observe(moved, 524, 750)
		if h.view.last().ContentKind != "media" {
			t.Fatal("icon move lost media")
		}
		h.c.Suspend(true)
		synctest.Wait()
		if h.view.hideCount() == 0 {
			t.Fatal("suspend retained media")
		}
	})
}

type noMediaObserver struct{}

func (noMediaObserver) ObserveDock(context.Context, func(platform.DockObservation)) error {
	panic("master without provider started observation")
}

func TestMediaHoverMasterWithoutProviderDoesNotObserve(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := config.Default()
		s.Dock.Media.Enabled = true
		c := NewController(Deps{Observations: noMediaObserver{}}, s)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { c.Run(ctx); close(done) }()
		synctest.Wait()
		cancel()
		<-done
	})
}

func TestMediaHoverDisplayMigrationAndWindowConditionalExclusion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, noMediaWindows{}, func(s *config.Settings) {
			mediaSettings(s)
			s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "com.apple.Music", Hide: config.HideWhenNoWindow}}
		})
		h.observe(musicIcon(), 424, 750)
		first := h.view.last()
		if first.ContentKind != "media" {
			t.Fatal("window-only exclusion hid media metadata")
		}
		h.env.ScreenList = append(h.env.ScreenList, domain.Screen{ID: 2, Bounds: domain.Bounds{X: 1200, W: 900, H: 800}, Visible: domain.Bounds{X: 1200, W: 900, H: 800}})
		moved := musicIcon()
		moved.ScreenID = 2
		moved.Bounds.X = 1500
		h.observe(moved, 1524, 750)
		next := h.view.last()
		if next.ContentKind != "media" || next.Session == first.Session || next.Bounds.X < 1200 || len(next.Windows) != 0 {
			t.Fatalf("display migration wrong %+v", next)
		}
		settings := h.settings
		settings.Behavior.Paused = true
		h.c.Configure(settings)
		synctest.Wait()
		if h.view.hideCount() < 2 {
			t.Fatal("pause did not retire migrated media")
		}
	})
}

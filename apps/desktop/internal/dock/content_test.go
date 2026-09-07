package dock

import (
	"errors"
	"slices"
	"testing"
	"testing/synctest"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func bothMusicSettings(s *config.Settings) { mediaSettings(s); s.Dock.Enabled = true }

func TestDockContentSelectionKeepsHoverWithFreshSessions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, bothMusicSettings)
		item := musicIcon()
		item.AppID = 10
		h.observe(item, 424, 750)
		initial := h.view.last()
		if initial.ContentKind != "media" || !slices.Equal(initial.ContentOptions, []string{"windows", "media"}) {
			t.Fatalf("initial choices: %+v", initial)
		}
		if err := h.c.SelectContent(initial.Session, "media"); err != nil || h.view.last().Session != initial.Session {
			t.Fatalf("same selection changed owner: %v", err)
		}
		if err := h.c.SelectContent(initial.Session, "windows"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		windows := h.view.last()
		if windows.ContentKind != "windows" || windows.Session <= initial.Session || len(windows.Windows) != 1 {
			t.Fatalf("window content: %+v", windows)
		}
		if err := h.c.SelectContent(initial.Session, "media"); err == nil {
			t.Fatal("stale session admitted")
		}
		if err := h.c.SelectContent(windows.Session, "media"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		next := h.view.last()
		if next.ContentKind != "media" || next.Session <= windows.Session || len(next.Windows) != 0 || h.view.hideCount() != 2 {
			t.Fatalf("media replacement: %+v", next)
		}
		h.c.Dismiss(windows.Session)
		synctest.Wait()
		if h.view.hideCount() != 2 {
			t.Fatal("late hide retired replacement")
		}
	})
}

func TestDockContentSelectionCanReturnDuringBlockedWindowQuery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &blockingWindowSource{requests: make(chan chan []domain.Window, 4)}
		h := newDockHarness(t, source, bothMusicSettings)
		item := musicIcon()
		item.AppID = 10
		h.observe(item, 424, 750)
		if err := h.c.SelectContent(h.view.last().Session, "windows"); err != nil {
			t.Fatal(err)
		}
		pending := <-source.requests
		loading := h.view.last()
		if loading.ContentKind != "windows" || loading.EmptyReason != "loading" {
			t.Fatalf("no selectable loading state: %+v", loading)
		}
		if err := h.c.SelectContent(loading.Session, "media"); err != nil {
			t.Fatal(err)
		}
		current := h.view.last()
		if current.ContentKind != "media" {
			t.Fatal("blocked query prevented returning to media")
		}
		pending <- []domain.Window{{ID: 123, AppID: 10}}
		synctest.Wait()
		if got := h.view.last(); got.Session != current.Session || got.ContentKind != "media" || len(got.Windows) != 0 {
			t.Fatalf("late query replaced media: %+v", got)
		}
		if len(source.requests) != 0 {
			t.Fatal("media selection queried windows")
		}
	})
}

type failedContentWindows struct{}

func (failedContentWindows) Windows() ([]domain.Window, error) {
	return nil, errors.New("fixture inventory unavailable")
}

func TestDockContentInventoryErrorPreservesMediaEscape(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, failedContentWindows{}, bothMusicSettings)
		item := musicIcon()
		item.AppID = 10
		h.observe(item, 424, 750)
		if err := h.c.SelectContent(h.view.last().Session, "windows"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		failed := h.view.last()
		if failed.ContentKind != "windows" || failed.Error == "" || failed.EmptyReason != "unavailable" {
			t.Fatalf("hidden inventory error: %+v", failed)
		}
		if err := h.c.SelectContent(failed.Session, "media"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDockContentSelectionRefusesUnavailableAndRetiredChoices(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, noMediaWindows{}, mediaSettings)
		h.observe(musicIcon(), 424, 750)
		state := h.view.last()
		if !slices.Equal(state.ContentOptions, []string{"media"}) {
			t.Fatalf("media-only choices %+v", state.ContentOptions)
		}
		for _, kind := range []string{"windows", "folder", "", "MEDIA"} {
			if err := h.c.SelectContent(state.Session, kind); err == nil {
				t.Fatalf("admitted %q", kind)
			}
		}
		h.c.Suspend(true)
		if err := h.c.SelectContent(state.Session, "media"); err == nil {
			t.Fatal("suspended choice admitted")
		}
	})
}

func TestDockFilteredWindowsKeepEligibleMediaChoice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) {
			bothMusicSettings(s)
			s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "com.apple.Music", Hide: config.HideWhenNoWindow}}
		})
		h.env.SetWindows(nil)
		item := musicIcon()
		item.AppID = 10
		h.observe(item, 424, 750)
		if err := h.c.SelectContent(h.view.last().Session, "windows"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		state := h.view.last()
		if h.view.hideCount() != 1 || state.ContentKind != "windows" || state.EmptyReason == "loading" {
			t.Fatalf("filtered windows retired media escape: hides=%d state=%+v", h.view.hideCount(), state)
		}
		if err := h.c.SelectContent(state.Session, "media"); err != nil {
			t.Fatalf("cannot return to eligible media: %v", err)
		}
	})
}

func TestDockMediaEscapeKeepsUnconditionalExclusions(t *testing.T) {
	item := Item{Kind: "app", AppID: 10, BundleID: "com.apple.Music", Title: "Music"}
	if mediaChoiceAllowed(item, config.Filters{}, "com.apple.Music") {
		t.Fatal("self allowed media escape")
	}
	for _, hide := range []config.BlacklistHide{config.HideAlways, ""} {
		if mediaChoiceAllowed(item, config.Filters{AppBlacklist: []config.BlacklistEntry{{Match: "com.apple.Music", Hide: hide}}}, "") {
			t.Fatal("unconditionally excluded app allowed media escape")
		}
	}
}

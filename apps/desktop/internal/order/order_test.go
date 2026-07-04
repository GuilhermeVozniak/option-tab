package order

import (
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func ids(ws []domain.Window) []domain.WindowID {
	out := make([]domain.WindowID, len(ws))
	for i, w := range ws {
		out[i] = w.ID
	}
	return out
}

func eq(a []domain.WindowID, b ...domain.WindowID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSort_Recent_MostRecentFirst(t *testing.T) {
	base := time.Unix(1000, 0)
	ws := []domain.Window{
		{ID: 1, LastFocused: base.Add(1 * time.Second)},
		{ID: 2, LastFocused: base.Add(3 * time.Second)},
		{ID: 3, LastFocused: base.Add(2 * time.Second)},
	}
	got := Sort(ws, config.OrderRecent)
	if !eq(ids(got), 2, 3, 1) {
		t.Errorf("recent order = %v, want [2 3 1]", ids(got))
	}
}

func TestSort_Recent_ZeroTimesGoLastStable(t *testing.T) {
	base := time.Unix(1000, 0)
	ws := []domain.Window{
		{ID: 1}, // zero time
		{ID: 2, LastFocused: base},
		{ID: 3}, // zero time
	}
	got := Sort(ws, config.OrderRecent)
	if !eq(ids(got), 2, 1, 3) {
		t.Errorf("zero-time order = %v, want [2 1 3]", ids(got))
	}
}

func TestSort_Alphabetical_ByAppThenTitleCaseInsensitive(t *testing.T) {
	ws := []domain.Window{
		{ID: 1, AppName: "Zed", Title: "a"},
		{ID: 2, AppName: "atom", Title: "b"},
		{ID: 3, AppName: "Atom", Title: "a"},
	}
	got := Sort(ws, config.OrderAlphabetical)
	if !eq(ids(got), 3, 2, 1) {
		t.Errorf("alphabetical order = %v, want [3 2 1]", ids(got))
	}
}

func TestSort_Space_BySpaceThenRecent(t *testing.T) {
	base := time.Unix(1000, 0)
	ws := []domain.Window{
		{ID: 1, SpaceID: 2, LastFocused: base.Add(1 * time.Second)},
		{ID: 2, SpaceID: 1, LastFocused: base.Add(1 * time.Second)},
		{ID: 3, SpaceID: 1, LastFocused: base.Add(2 * time.Second)},
	}
	got := Sort(ws, config.OrderSpace)
	if !eq(ids(got), 3, 2, 1) {
		t.Errorf("space order = %v, want [3 2 1]", ids(got))
	}
}

func TestSort_DoesNotMutateInput(t *testing.T) {
	ws := []domain.Window{
		{ID: 1, LastFocused: time.Unix(1, 0)},
		{ID: 2, LastFocused: time.Unix(2, 0)},
	}
	_ = Sort(ws, config.OrderRecent)
	if ws[0].ID != 1 || ws[1].ID != 2 {
		t.Error("Sort must not mutate the input slice")
	}
}

func TestSort_RecentlyCreated_NewestIDFirst(t *testing.T) {
	ws := []domain.Window{{ID: 10}, {ID: 30}, {ID: 20}}
	got := Sort(ws, config.OrderRecentlyCreated)
	if !eq(ids(got), 30, 20, 10) {
		t.Errorf("recentlyCreated order = %v, want [30 20 10]", ids(got))
	}
}

func TestSendToBack_MovesConfiguredClassesToEnd(t *testing.T) {
	f := config.Default().Filters
	f.ShowMinimized = config.VisShowAtEnd
	f.ShowHiddenApps = config.VisShowAtEnd
	ws := []domain.Window{
		{ID: 1, Minimized: true},
		{ID: 2},
		{ID: 3, Hidden: true},
		{ID: 4},
	}
	got := SendToBack(ws, f)
	if !eq(ids(got), 2, 4, 1, 3) {
		t.Errorf("SendToBack order = %v, want [2 4 1 3]", ids(got))
	}
}

func TestSendToBack_NoopWhenShow(t *testing.T) {
	f := config.Default().Filters // all "show"
	ws := []domain.Window{{ID: 1, Minimized: true}, {ID: 2}}
	got := SendToBack(ws, f)
	if !eq(ids(got), 1, 2) {
		t.Errorf("SendToBack should be a no-op when everything is show, got %v", ids(got))
	}
}

func TestSort_UnknownModeDefaultsToRecent(t *testing.T) {
	base := time.Unix(1000, 0)
	ws := []domain.Window{
		{ID: 1, LastFocused: base},
		{ID: 2, LastFocused: base.Add(time.Second)},
	}
	got := Sort(ws, config.OrderMode("bogus"))
	if !eq(ids(got), 2, 1) {
		t.Errorf("unknown mode should fall back to recent, got %v", ids(got))
	}
}

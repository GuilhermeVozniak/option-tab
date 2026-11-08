package switcher

import (
	"testing"

	"option-tab/internal/platform"
)

func TestSearchRejectsReorderedAndRetiredRequests(t *testing.T) {
	c, _, _ := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	session := c.State().Session
	c.SetSearchForSession("ab", session, 2)
	c.SetSearchForSession("a", session, 1)
	if got := c.State(); got.Search != "ab" || got.SearchSequence != 2 {
		t.Fatalf("older request replaced search: %+v", got)
	}
	c.Cancel()
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.SetSearchForSession("old", session, 3)
	if got := c.State(); got.Search != "" || got.SearchSequence != 0 {
		t.Fatalf("retired search leaked into new session: %+v", got)
	}
	c.SetSearchForSession("new", c.State().Session, 1)
	if c.State().Search != "new" {
		t.Fatal("new session did not admit search")
	}
}

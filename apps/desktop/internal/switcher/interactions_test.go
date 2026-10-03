package switcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func TestInteractionSettingsReachStateWithoutAliasing(t *testing.T) {
	c, _, v := newController(t, threeWins(), func(s *config.Settings) {
		s.Behavior.ActionBindings = map[string]config.ActionKind{"KeyX": config.ActionClose}
		s.Behavior.MiddleClickAction = config.PointerMinimize
		s.Behavior.SwipeUpAction = config.PointerClose
	})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	st := v.last()
	if st.ActionBindings["KeyX"] != config.ActionClose || st.MiddleClickAction != config.PointerMinimize || st.SwipeUpAction != config.PointerClose {
		t.Fatal("interaction preferences did not reach rendered state")
	}
	st.ActionBindings["KeyX"] = config.ActionQuit
	if c.State().ActionBindings["KeyX"] != config.ActionClose {
		t.Fatal("view modified persisted settings")
	}
}

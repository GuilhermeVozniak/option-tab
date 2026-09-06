package config

import (
	"errors"
	"fmt"
)

// SwitcherMode selects whether a shortcut cycles individual windows or one
// entry per running application. It is independent of the visual style.
type SwitcherMode string

const (
	ModeWindows SwitcherMode = "windows"
	ModeApps    SwitcherMode = "apps"
)

func (m SwitcherMode) Valid() bool { return m == ModeWindows || m == ModeApps }

// SwitcherBehavior is the mode-local subset of Behavior. Global lifecycle and
// telemetry settings deliberately remain outside this type.
type SwitcherBehavior struct {
	HoldToCycle       bool                  `json:"holdToCycle"`
	VimKeys           bool                  `json:"vimKeys"`
	ArrowKeys         bool                  `json:"arrowKeys"`
	MouseHoverSelect  bool                  `json:"mouseHoverSelect"`
	CursorFollowFocus bool                  `json:"cursorFollowFocus"`
	HapticFeedback    bool                  `json:"hapticFeedback"`
	ActionBindings    map[string]ActionKind `json:"actionBindings"`
	MiddleClickAction PointerAction         `json:"middleClickAction"`
	SwipeUpAction     PointerAction         `json:"swipeUpAction"`
	SwipeDownAction   PointerAction         `json:"swipeDownAction"`
}

type ModePreferences struct {
	Appearance Appearance       `json:"appearance"`
	Behavior   SwitcherBehavior `json:"behavior"`
	Order      OrderMode        `json:"order"`
	Placement  Placement        `json:"placement"`
}

func cloneActionBindings(in map[string]ActionKind) map[string]ActionKind {
	if in == nil {
		return nil
	}
	out := make(map[string]ActionKind, len(in))
	for code, action := range in {
		out[code] = action
	}
	return out
}

func switcherBehaviorFrom(b Behavior) SwitcherBehavior {
	return SwitcherBehavior{
		HoldToCycle: b.HoldToCycle, VimKeys: b.VimKeys, ArrowKeys: b.ArrowKeys,
		MouseHoverSelect: b.MouseHoverSelect, CursorFollowFocus: b.CursorFollowFocus,
		HapticFeedback: b.HapticFeedback, ActionBindings: cloneActionBindings(b.ActionBindings),
		MiddleClickAction: b.MiddleClickAction, SwipeUpAction: b.SwipeUpAction, SwipeDownAction: b.SwipeDownAction,
	}
}

func cloneSwitcherBehavior(b SwitcherBehavior) SwitcherBehavior {
	b.ActionBindings = cloneActionBindings(b.ActionBindings)
	return b
}

func appPreferencesFrom(a Appearance, b Behavior, order OrderMode, placement Placement) ModePreferences {
	a.Style = StyleAppIcons
	a.PreviewSelected = true
	return ModePreferences{Appearance: a, Behavior: switcherBehaviorFrom(b), Order: order, Placement: placement}
}

// Preferences returns an independent snapshot for the requested mode.
func (s Settings) Preferences(mode SwitcherMode) ModePreferences {
	if mode == ModeApps {
		p := s.AppSwitcher
		p.Behavior = cloneSwitcherBehavior(p.Behavior)
		return p
	}
	return ModePreferences{Appearance: s.Appearance, Behavior: switcherBehaviorFrom(s.Behavior), Order: s.Order, Placement: s.Placement}
}

func validateAppearance(prefix string, a Appearance) error {
	if !a.Style.Valid() {
		return fmt.Errorf("config: invalid %s style %q", prefix, a.Style)
	}
	if !a.Theme.Valid() || !a.SizePreset.Valid() || !a.TitleTruncation.Valid() || !a.LayoutDirection.Valid() {
		return fmt.Errorf("config: invalid %s appearance enum", prefix)
	}
	if a.ThumbnailMaxPx < minThumbnailPx || a.ThumbnailMaxPx > maxThumbnailPx || a.IconSizePx < minIconPx || a.IconSizePx > maxIconPx || a.FontSizePx < minFontPx || a.FontSizePx > maxFontPx || a.MaxRows < 1 || a.MaxRows > 20 || a.MaxColumns < 1 || a.MaxColumns > 20 || a.TitleMaxWidthPx < 60 || a.TitleMaxWidthPx > 1000 || a.CornerRadiusPx < 0 || a.CornerRadiusPx > 64 || a.BackgroundOpacity < 0 || a.BackgroundOpacity > 1 || a.ApparitionDelayMs < 0 || a.ApparitionDelayMs > 2000 || a.CompactThreshold < 0 || a.CompactThreshold > 1000 {
		return fmt.Errorf("config: %s appearance value out of range", prefix)
	}
	return nil
}

func validateSwitcherBehavior(prefix string, b SwitcherBehavior) error {
	if !b.MiddleClickAction.ValidMiddleClick() || !b.SwipeUpAction.ValidSwipe() || !b.SwipeDownAction.ValidSwipe() {
		return fmt.Errorf("config: invalid %s pointer action", prefix)
	}
	for code, action := range b.ActionBindings {
		if !validActionCode(code) || !action.Valid() {
			return fmt.Errorf("config: invalid %s action binding %q=%q", prefix, code, action)
		}
	}
	return nil
}

func normalizeAppearance(a, d Appearance) Appearance {
	if !a.Style.Valid() {
		a.Style = d.Style
	}
	if !a.Theme.Valid() {
		a.Theme = d.Theme
	}
	if !a.SizePreset.Valid() {
		a.SizePreset = d.SizePreset
	}
	if !a.TitleTruncation.Valid() {
		a.TitleTruncation = d.TitleTruncation
	}
	if !a.LayoutDirection.Valid() {
		a.LayoutDirection = d.LayoutDirection
	}
	a.ThumbnailMaxPx = clampInt(a.ThumbnailMaxPx, minThumbnailPx, maxThumbnailPx)
	a.IconSizePx = clampInt(a.IconSizePx, minIconPx, maxIconPx)
	a.FontSizePx = clampInt(a.FontSizePx, minFontPx, maxFontPx)
	a.MaxRows = clampInt(a.MaxRows, 1, 20)
	a.MaxColumns = clampInt(a.MaxColumns, 1, 20)
	a.TitleMaxWidthPx = clampInt(a.TitleMaxWidthPx, 60, 1000)
	a.CornerRadiusPx = clampInt(a.CornerRadiusPx, 0, 64)
	a.BackgroundOpacity = clampFloat(a.BackgroundOpacity, 0, 1)
	a.ApparitionDelayMs = clampInt(a.ApparitionDelayMs, 0, 2000)
	a.CompactThreshold = clampInt(a.CompactThreshold, 0, 1000)
	return a
}

func normalizeSwitcherBehavior(b, d SwitcherBehavior) SwitcherBehavior {
	if !b.MiddleClickAction.ValidMiddleClick() {
		b.MiddleClickAction = d.MiddleClickAction
	}
	if !b.SwipeUpAction.ValidSwipe() {
		b.SwipeUpAction = d.SwipeUpAction
	}
	if !b.SwipeDownAction.ValidSwipe() {
		b.SwipeDownAction = d.SwipeDownAction
	}
	if b.ActionBindings == nil {
		b.ActionBindings = d.ActionBindings
	}
	b.ActionBindings = cloneActionBindings(b.ActionBindings)
	for code, action := range b.ActionBindings {
		if !validActionCode(code) || !action.Valid() {
			delete(b.ActionBindings, code)
		}
	}
	return b
}

func validateModePreferences(prefix string, p ModePreferences) error {
	if err := validateAppearance(prefix, p.Appearance); err != nil {
		return err
	}
	if err := validateSwitcherBehavior(prefix, p.Behavior); err != nil {
		return err
	}
	if !p.Order.Valid() || !p.Placement.Valid() {
		return errors.New("config: invalid mode order or placement")
	}
	return nil
}

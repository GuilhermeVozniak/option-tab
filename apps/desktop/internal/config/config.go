// Package config defines the persisted settings model for the window switcher,
// with sensible AltTab-like defaults, range validation/normalization, and
// versioned JSON load/save. All logic here is pure and unit-tested; callers
// supply io.Reader/io.Writer (or use the *File helpers in paths.go).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// CurrentVersion is the settings schema version. Bump when the shape changes
// and add a migration step in migrate().
const CurrentVersion = 1

// MaxShortcuts is the number of independent shortcuts AltTab supports; we match
// it (all free).
const MaxShortcuts = 9

// Appearance range floors/ceilings used by Normalize.
const (
	minThumbnailPx = 64
	maxThumbnailPx = 1024
	minIconPx      = 16
	maxIconPx      = 256
	minFontPx      = 8
	maxFontPx      = 48
)

// VisualStyle is how the switcher renders each entry.
type VisualStyle string

const (
	StyleThumbnails VisualStyle = "thumbnails"
	StyleAppIcons   VisualStyle = "appIcons"
	StyleTitles     VisualStyle = "titles"
)

// Valid reports whether the style is a known value.
func (v VisualStyle) Valid() bool {
	switch v {
	case StyleThumbnails, StyleAppIcons, StyleTitles:
		return true
	}
	return false
}

// Theme selects the color scheme.
type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

func (t Theme) Valid() bool {
	switch t {
	case ThemeSystem, ThemeLight, ThemeDark:
		return true
	}
	return false
}

// OrderMode is the display order of windows in the switcher.
type OrderMode string

const (
	OrderRecent       OrderMode = "recent"
	OrderAlphabetical OrderMode = "alphabetical"
	OrderSpace        OrderMode = "space"
)

func (o OrderMode) Valid() bool {
	switch o {
	case OrderRecent, OrderAlphabetical, OrderSpace:
		return true
	}
	return false
}

// Placement is where the overlay appears.
type Placement string

const (
	PlaceActiveScreen        Placement = "activeScreen"
	PlaceCursorScreen        Placement = "cursorScreen"
	PlaceFocusedWindowScreen Placement = "focusedWindowScreen"
)

func (p Placement) Valid() bool {
	switch p {
	case PlaceActiveScreen, PlaceCursorScreen, PlaceFocusedWindowScreen:
		return true
	}
	return false
}

// SpaceScope limits which Spaces' windows are shown.
type SpaceScope string

const (
	SpacesActive SpaceScope = "active"
	SpacesAll    SpaceScope = "all"
)

func (s SpaceScope) Valid() bool { return s == SpacesActive || s == SpacesAll }

// ScreenScope limits which screens' windows are shown.
type ScreenScope string

const (
	ScreensActive ScreenScope = "active"
	ScreensAll    ScreenScope = "all"
	ScreensCursor ScreenScope = "cursor"
)

func (s ScreenScope) Valid() bool {
	return s == ScreensActive || s == ScreensAll || s == ScreensCursor
}

// AppScopeMode limits a shortcut to all apps or only the active app's windows.
type AppScopeMode string

const (
	AppScopeAll       AppScopeMode = "all"
	AppScopeActiveApp AppScopeMode = "activeApp"
)

func (a AppScopeMode) Valid() bool { return a == AppScopeAll || a == AppScopeActiveApp }

// Filters control which windows are eligible to be shown.
type Filters struct {
	Spaces                  SpaceScope  `json:"spaces"`
	Screens                 ScreenScope `json:"screens"`
	ShowMinimized           bool        `json:"showMinimized"`
	ShowHiddenApps          bool        `json:"showHiddenApps"`
	ShowFullscreen          bool        `json:"showFullscreen"`
	ShowWindowsWithoutTitle bool        `json:"showWindowsWithoutTitle"`
	AppBlacklist            []string    `json:"appBlacklist"` // bundle ids or app names
}

// Appearance controls the look of the overlay.
type Appearance struct {
	Style              VisualStyle `json:"style"`
	Theme              Theme       `json:"theme"`
	MaxRows            int         `json:"maxRows"`
	MaxColumns         int         `json:"maxColumns"`
	ThumbnailMaxPx     int         `json:"thumbnailMaxPx"`
	IconSizePx         int         `json:"iconSizePx"`
	TitleMaxWidthPx    int         `json:"titleMaxWidthPx"`
	FontSizePx         int         `json:"fontSizePx"`
	AccentColor        string      `json:"accentColor"`
	BackgroundOpacity  float64     `json:"backgroundOpacity"`
	Blur               bool        `json:"blur"`
	CornerRadiusPx     int         `json:"cornerRadiusPx"`
	ShowAppBadge       bool        `json:"showAppBadge"`
	ShowTitle          bool        `json:"showTitle"`
	ShowWindowControls bool        `json:"showWindowControls"`
	AutoSize           bool        `json:"autoSize"`
}

// ShortcutScope narrows the windows a given shortcut shows. Empty SpaceScope/
// ScreenScope mean "inherit the global Filters".
type ShortcutScope struct {
	AppScope AppScopeMode `json:"appScope"`
	Spaces   SpaceScope   `json:"spaces,omitempty"`
	Screens  ScreenScope  `json:"screens,omitempty"`
}

// Shortcut is one of up to MaxShortcuts independent activation chords.
type Shortcut struct {
	ID            int           `json:"id"`
	Chord         string        `json:"chord"`
	Enabled       bool          `json:"enabled"`
	Scope         ShortcutScope `json:"scope"`
	StyleOverride VisualStyle   `json:"styleOverride,omitempty"` // empty = use global style
}

// Behavior controls activation semantics and system integration.
type Behavior struct {
	HoldToCycle     bool `json:"holdToCycle"` // true: hold modifier, release to select
	StartAtLogin    bool `json:"startAtLogin"`
	Paused          bool `json:"paused"`
	ShowMenubarIcon bool `json:"showMenubarIcon"`
}

// Settings is the full persisted configuration.
type Settings struct {
	Version    int        `json:"version"`
	Shortcuts  []Shortcut `json:"shortcuts"`
	Appearance Appearance `json:"appearance"`
	Filters    Filters    `json:"filters"`
	Order      OrderMode  `json:"order"`
	Placement  Placement  `json:"placement"`
	Behavior   Behavior   `json:"behavior"`
}

// Default returns the AltTab-like default settings.
func Default() Settings {
	return Settings{
		Version: CurrentVersion,
		Shortcuts: []Shortcut{
			{
				ID:      1,
				Chord:   "option+tab",
				Enabled: true,
				Scope:   ShortcutScope{AppScope: AppScopeAll},
			},
			{
				ID:      2,
				Chord:   "option+grave",
				Enabled: true,
				Scope:   ShortcutScope{AppScope: AppScopeActiveApp},
			},
		},
		Appearance: Appearance{
			Style:              StyleThumbnails,
			Theme:              ThemeSystem,
			MaxRows:            4,
			MaxColumns:         6,
			ThumbnailMaxPx:     256,
			IconSizePx:         32,
			TitleMaxWidthPx:    240,
			FontSizePx:         13,
			AccentColor:        "#3b82f6",
			BackgroundOpacity:  0.85,
			Blur:               true,
			CornerRadiusPx:     12,
			ShowAppBadge:       true,
			ShowTitle:          true,
			ShowWindowControls: true,
			AutoSize:           true,
		},
		Filters: Filters{
			Spaces:                  SpacesAll,
			Screens:                 ScreensAll,
			ShowMinimized:           true,
			ShowHiddenApps:          true,
			ShowFullscreen:          true,
			ShowWindowsWithoutTitle: false,
			AppBlacklist:            nil,
		},
		Order:     OrderRecent,
		Placement: PlaceCursorScreen,
		Behavior: Behavior{
			HoldToCycle:     true,
			StartAtLogin:    false,
			Paused:          false,
			ShowMenubarIcon: true,
		},
	}
}

// Validate reports whether the settings are internally consistent. It is strict;
// use Normalize to coerce a loaded document into a valid one.
func (s Settings) Validate() error {
	if len(s.Shortcuts) == 0 {
		return errors.New("config: at least one shortcut is required")
	}
	seen := map[int]bool{}
	for _, sc := range s.Shortcuts {
		if sc.ID < 1 || sc.ID > MaxShortcuts {
			return fmt.Errorf("config: shortcut id %d out of range 1..%d", sc.ID, MaxShortcuts)
		}
		if seen[sc.ID] {
			return fmt.Errorf("config: duplicate shortcut id %d", sc.ID)
		}
		seen[sc.ID] = true
		if sc.Chord == "" {
			return fmt.Errorf("config: shortcut %d has empty chord", sc.ID)
		}
	}
	if !s.Appearance.Style.Valid() {
		return fmt.Errorf("config: invalid style %q", s.Appearance.Style)
	}
	if !s.Order.Valid() {
		return fmt.Errorf("config: invalid order %q", s.Order)
	}
	if !s.Placement.Valid() {
		return fmt.Errorf("config: invalid placement %q", s.Placement)
	}
	return nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Normalize returns a copy coerced into a valid, in-range configuration,
// falling back to defaults for any invalid enum and clamping numeric ranges.
func (s Settings) Normalize() Settings {
	d := Default()
	out := s

	// Enums fall back to defaults when invalid.
	if !out.Appearance.Style.Valid() {
		out.Appearance.Style = d.Appearance.Style
	}
	if !out.Appearance.Theme.Valid() {
		out.Appearance.Theme = d.Appearance.Theme
	}
	if !out.Order.Valid() {
		out.Order = d.Order
	}
	if !out.Placement.Valid() {
		out.Placement = d.Placement
	}
	if !out.Filters.Spaces.Valid() {
		out.Filters.Spaces = d.Filters.Spaces
	}
	if !out.Filters.Screens.Valid() {
		out.Filters.Screens = d.Filters.Screens
	}

	// Numeric clamps.
	out.Appearance.ThumbnailMaxPx = clampInt(out.Appearance.ThumbnailMaxPx, minThumbnailPx, maxThumbnailPx)
	out.Appearance.IconSizePx = clampInt(out.Appearance.IconSizePx, minIconPx, maxIconPx)
	out.Appearance.FontSizePx = clampInt(out.Appearance.FontSizePx, minFontPx, maxFontPx)
	out.Appearance.MaxRows = clampInt(out.Appearance.MaxRows, 1, 20)
	out.Appearance.MaxColumns = clampInt(out.Appearance.MaxColumns, 1, 20)
	out.Appearance.TitleMaxWidthPx = clampInt(out.Appearance.TitleMaxWidthPx, 60, 1000)
	out.Appearance.CornerRadiusPx = clampInt(out.Appearance.CornerRadiusPx, 0, 64)
	out.Appearance.BackgroundOpacity = clampFloat(out.Appearance.BackgroundOpacity, 0, 1)

	// Shortcuts: drop empties, clamp/skip out-of-range ids, dedupe by id, cap count.
	var fixed []Shortcut
	seen := map[int]bool{}
	for _, sc := range out.Shortcuts {
		if sc.ID < 1 || sc.ID > MaxShortcuts || sc.Chord == "" || seen[sc.ID] {
			continue
		}
		if !sc.Scope.AppScope.Valid() {
			sc.Scope.AppScope = AppScopeAll
		}
		if sc.StyleOverride != "" && !sc.StyleOverride.Valid() {
			sc.StyleOverride = ""
		}
		seen[sc.ID] = true
		fixed = append(fixed, sc)
	}
	if len(fixed) == 0 {
		fixed = d.Shortcuts
	}
	out.Shortcuts = fixed

	if out.Version == 0 {
		out.Version = CurrentVersion
	}
	return out
}

// migrate upgrades an older document in place to CurrentVersion. New migration
// steps are appended as the schema evolves.
func migrate(s *Settings) {
	if s.Version < 1 {
		// v0 (pre-versioned) -> v1: nothing structural changed; just stamp it.
		s.Version = 1
	}
	s.Version = CurrentVersion
}

// Load reads JSON settings, merging them onto Default so omitted fields keep
// sensible values, then migrates and normalizes the result.
func Load(r io.Reader) (Settings, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Settings{}, fmt.Errorf("config: read: %w", err)
	}
	s := Default()
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, fmt.Errorf("config: parse: %w", err)
	}
	migrate(&s)
	return s.Normalize(), nil
}

// Save writes settings as indented JSON.
func Save(w io.Writer, s Settings) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	return nil
}

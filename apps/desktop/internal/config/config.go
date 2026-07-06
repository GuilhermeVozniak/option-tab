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
// v2: minimized/hidden/fullscreen filters became tristate WindowVisibility
// (legacy bools still parse) and the blacklist became structured entries
// (legacy plain strings still parse).
const CurrentVersion = 2

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
	OrderRecent          OrderMode = "recent"
	OrderRecentlyCreated OrderMode = "recentlyCreated"
	OrderAlphabetical    OrderMode = "alphabetical"
	OrderSpace           OrderMode = "space"
)

func (o OrderMode) Valid() bool {
	switch o {
	case OrderRecent, OrderRecentlyCreated, OrderAlphabetical, OrderSpace:
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

// WindowVisibility is the AltTab-style tristate for a class of windows:
// show them normally, hide them, or show them at the end of the list.
type WindowVisibility string

const (
	VisShow      WindowVisibility = "show"
	VisHide      WindowVisibility = "hide"
	VisShowAtEnd WindowVisibility = "showAtEnd"
)

func (v WindowVisibility) Valid() bool {
	return v == VisShow || v == VisHide || v == VisShowAtEnd
}

// UnmarshalJSON accepts the v1 boolean form (true=show, false=hide) as well as
// the v2 string form, so old settings files keep loading.
func (v *WindowVisibility) UnmarshalJSON(b []byte) error {
	var asBool bool
	if err := json.Unmarshal(b, &asBool); err == nil {
		if asBool {
			*v = VisShow
		} else {
			*v = VisHide
		}
		return nil
	}
	var asStr string
	if err := json.Unmarshal(b, &asStr); err != nil {
		return err
	}
	*v = WindowVisibility(asStr)
	return nil
}

// SizePreset is the coarse switcher size (AltTab's Small/Medium/Large): it
// drives the thumbnail/icon pixel sizes from one user-facing control.
type SizePreset string

const (
	SizeSmall  SizePreset = "small"
	SizeMedium SizePreset = "medium"
	SizeLarge  SizePreset = "large"
)

func (s SizePreset) Valid() bool {
	return s == SizeSmall || s == SizeMedium || s == SizeLarge
}

// TruncationMode is where long window titles get elided.
type TruncationMode string

const (
	TruncateEnd    TruncationMode = "end"
	TruncateMiddle TruncationMode = "middle"
	TruncateStart  TruncationMode = "start"
)

func (t TruncationMode) Valid() bool {
	return t == TruncateEnd || t == TruncateMiddle || t == TruncateStart
}

// ReleaseAction is what happens when the shortcut's modifier is released.
type ReleaseAction string

const (
	ReleaseFocusSelected ReleaseAction = "focusSelected"
	ReleaseDoNothing     ReleaseAction = "doNothing"
)

func (r ReleaseAction) Valid() bool {
	return r == ReleaseFocusSelected || r == ReleaseDoNothing
}

// MenubarIconStyle selects the status-item glyph (AltTab offers several).
type MenubarIconStyle string

const (
	MenubarIconDefault MenubarIconStyle = "default"
	MenubarIconOutline MenubarIconStyle = "outline"
	MenubarIconDot     MenubarIconStyle = "dot"
)

func (m MenubarIconStyle) Valid() bool {
	return m == MenubarIconDefault || m == MenubarIconOutline || m == MenubarIconDot
}

// UpdatePolicy mirrors AltTab's update-check preference.
type UpdatePolicy string

const (
	UpdatesOff   UpdatePolicy = "off"
	UpdatesCheck UpdatePolicy = "check"
	UpdatesAuto  UpdatePolicy = "auto"
)

func (u UpdatePolicy) Valid() bool {
	return u == UpdatesOff || u == UpdatesCheck || u == UpdatesAuto
}

// CrashPolicy mirrors AltTab's crash-report preference. option-tab never
// transmits anything; the choice is persisted for parity and future use.
type CrashPolicy string

const (
	CrashNever  CrashPolicy = "never"
	CrashAsk    CrashPolicy = "ask"
	CrashAlways CrashPolicy = "always"
)

func (c CrashPolicy) Valid() bool {
	return c == CrashNever || c == CrashAsk || c == CrashAlways
}

// BlacklistHide is when a blacklisted app's windows are hidden from the list.
type BlacklistHide string

const (
	// HideAlways removes the app's windows entirely.
	HideAlways BlacklistHide = "always"
	// HideWhenNoWindow only hides the app when it has no open window (relevant
	// to app-icon views); its real windows still show.
	HideWhenNoWindow BlacklistHide = "whenNoWindow"
)

func (h BlacklistHide) Valid() bool { return h == HideAlways || h == HideWhenNoWindow }

// BlacklistEntry is one row of the AltTab-style blacklist table: an app
// matcher plus what to hide and whether shortcuts are ignored while the app
// is active.
type BlacklistEntry struct {
	// Match is a bundle id (com.apple.Safari) or app name.
	Match string `json:"match"`
	// Hide is when the app's windows are hidden (default: always).
	Hide BlacklistHide `json:"hide"`
	// IgnoreShortcuts suppresses switcher activation while this app is active
	// (e.g. games or VMs that need Option+Tab for themselves).
	IgnoreShortcuts bool `json:"ignoreShortcuts"`
}

// UnmarshalJSON accepts the v1 plain-string form ("com.foo" == hide always)
// as well as the v2 object form.
func (e *BlacklistEntry) UnmarshalJSON(b []byte) error {
	var asStr string
	if err := json.Unmarshal(b, &asStr); err == nil {
		*e = BlacklistEntry{Match: asStr, Hide: HideAlways}
		return nil
	}
	type alias BlacklistEntry // avoid recursing into this method
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*e = BlacklistEntry(a)
	return nil
}

// Filters control which windows are eligible to be shown.
type Filters struct {
	Spaces  SpaceScope  `json:"spaces"`
	Screens ScreenScope `json:"screens"`
	// Minimized/hidden/fullscreen windows are tristate: show, hide, or show at
	// the end of the list (AltTab parity). Legacy bools still parse.
	ShowMinimized           WindowVisibility `json:"showMinimized"`
	ShowHiddenApps          WindowVisibility `json:"showHiddenApps"`
	ShowFullscreen          WindowVisibility `json:"showFullscreen"`
	ShowWindowsWithoutTitle bool             `json:"showWindowsWithoutTitle"`
	AppBlacklist            []BlacklistEntry `json:"appBlacklist"`
}

// Appearance controls the look of the overlay.
type Appearance struct {
	Style VisualStyle `json:"style"`
	Theme Theme       `json:"theme"`
	// SizePreset is the coarse overlay size; the preferences UI derives the
	// pixel fields below from it (they remain authoritative for rendering).
	SizePreset         SizePreset `json:"sizePreset"`
	MaxRows            int        `json:"maxRows"`
	MaxColumns         int        `json:"maxColumns"`
	ThumbnailMaxPx     int        `json:"thumbnailMaxPx"`
	IconSizePx         int        `json:"iconSizePx"`
	TitleMaxWidthPx    int        `json:"titleMaxWidthPx"`
	FontSizePx         int        `json:"fontSizePx"`
	AccentColor        string     `json:"accentColor"`
	BackgroundOpacity  float64    `json:"backgroundOpacity"`
	Blur               bool       `json:"blur"`
	CornerRadiusPx     int        `json:"cornerRadiusPx"`
	ShowAppBadge       bool       `json:"showAppBadge"`
	ShowTitle          bool       `json:"showTitle"`
	ShowWindowControls bool       `json:"showWindowControls"`
	AutoSize           bool       `json:"autoSize"`
	// ApparitionDelayMs postpones showing the overlay so quick app switches
	// don't flash it (AltTab's "apparition delay").
	ApparitionDelayMs int `json:"apparitionDelayMs"`
	// FadeOutAnimation animates the overlay's dismissal.
	FadeOutAnimation bool `json:"fadeOutAnimation"`
	// ShowStatusIcons shows the minimized/hidden/fullscreen markers.
	ShowStatusIcons bool `json:"showStatusIcons"`
	// ShowSpaceNumbers shows Space number badges on windows of other Spaces.
	ShowSpaceNumbers bool `json:"showSpaceNumbers"`
	// TitleTruncation is where long titles are elided (end/middle/start).
	TitleTruncation TruncationMode `json:"titleTruncation"`
	// PreviewSelected shows a large preview of the selected window.
	PreviewSelected bool `json:"previewSelected"`
	// PreviewFade fades the selected-window preview in when it changes.
	PreviewFade bool `json:"previewFade"`
}

// ShortcutScope narrows the windows a given shortcut shows. Empty SpaceScope/
// ScreenScope mean "inherit the global Filters".
type ShortcutScope struct {
	AppScope AppScopeMode `json:"appScope"`
	Spaces   SpaceScope   `json:"spaces,omitempty"`
	Screens  ScreenScope  `json:"screens,omitempty"`
	// Order overrides the global display order for this shortcut (AltTab
	// configures order per shortcut). Empty = inherit the global Order.
	Order OrderMode `json:"order,omitempty"`
}

// Shortcut is one of up to MaxShortcuts independent activation chords.
type Shortcut struct {
	ID            int           `json:"id"`
	Chord         string        `json:"chord"`
	Enabled       bool          `json:"enabled"`
	Scope         ShortcutScope `json:"scope"`
	StyleOverride VisualStyle   `json:"styleOverride,omitempty"` // empty = use global style
	// WhenReleased is what releasing the held modifier does: focus the selected
	// window (default) or nothing, leaving the switcher open until Enter/Esc.
	WhenReleased ReleaseAction `json:"whenReleased,omitempty"`
}

// Behavior controls activation semantics and system integration.
type Behavior struct {
	HoldToCycle     bool `json:"holdToCycle"` // true: hold modifier, release to select
	StartAtLogin    bool `json:"startAtLogin"`
	Paused          bool `json:"paused"`
	ShowMenubarIcon bool `json:"showMenubarIcon"`
	VimKeys         bool `json:"vimKeys"`   // h/j/k/l navigate the switcher
	ArrowKeys       bool `json:"arrowKeys"` // arrow keys navigate the switcher
	// MenubarIconStyle picks the status-item glyph when the icon is shown.
	MenubarIconStyle MenubarIconStyle `json:"menubarIconStyle"`
	// Language is a BCP-47 tag for the UI language; empty = system default.
	Language string `json:"language"`
	// UpdatePolicy is the update-check preference (off/check/auto).
	UpdatePolicy UpdatePolicy `json:"updatePolicy"`
	// CrashReports is the crash-report preference (never/ask/always).
	CrashReports CrashPolicy `json:"crashReports"`
	// MouseHoverSelect selects entries by hovering them with the mouse.
	MouseHoverSelect bool `json:"mouseHoverSelect"`
	// CursorFollowFocus warps the cursor to the focused window on commit.
	CursorFollowFocus bool `json:"cursorFollowFocus"`
	// HapticFeedback taps the trackpad when the selection changes.
	HapticFeedback bool `json:"hapticFeedback"`
	// CaptureInBackground refreshes window thumbnails while the switcher is
	// hidden so it opens with fresh previews (shows the macOS screen-recording
	// indicator while enabled).
	CaptureInBackground bool `json:"captureInBackground"`
	// Onboarded records that the first-run permissions wizard was completed,
	// so it is only shown once.
	Onboarded bool `json:"onboarded"`
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
			SizePreset:         SizeMedium,
			MaxRows:            4,
			MaxColumns:         6,
			ThumbnailMaxPx:     280,
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
			ApparitionDelayMs:  0,
			FadeOutAnimation:   true,
			ShowStatusIcons:    true,
			ShowSpaceNumbers:   true,
			TitleTruncation:    TruncateEnd,
			PreviewSelected:    false,
			PreviewFade:        true,
		},
		Filters: Filters{
			Spaces:                  SpacesAll,
			Screens:                 ScreensAll,
			ShowMinimized:           VisShow,
			ShowHiddenApps:          VisShow,
			ShowFullscreen:          VisShow,
			ShowWindowsWithoutTitle: false,
			AppBlacklist:            []BlacklistEntry{},
		},
		Order:     OrderRecent,
		Placement: PlaceCursorScreen,
		Behavior: Behavior{
			HoldToCycle:         true,
			StartAtLogin:        false,
			Paused:              false,
			ShowMenubarIcon:     true,
			ArrowKeys:           true,
			MenubarIconStyle:    MenubarIconDefault,
			Language:            "",
			UpdatePolicy:        UpdatesCheck,
			CrashReports:        CrashAsk,
			MouseHoverSelect:    true,
			CursorFollowFocus:   false,
			HapticFeedback:      true,
			CaptureInBackground: false,
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
	if !out.Appearance.SizePreset.Valid() {
		out.Appearance.SizePreset = d.Appearance.SizePreset
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
	if !out.Filters.ShowMinimized.Valid() {
		out.Filters.ShowMinimized = d.Filters.ShowMinimized
	}
	if !out.Filters.ShowHiddenApps.Valid() {
		out.Filters.ShowHiddenApps = d.Filters.ShowHiddenApps
	}
	if !out.Filters.ShowFullscreen.Valid() {
		out.Filters.ShowFullscreen = d.Filters.ShowFullscreen
	}
	if !out.Appearance.TitleTruncation.Valid() {
		out.Appearance.TitleTruncation = d.Appearance.TitleTruncation
	}
	if !out.Behavior.MenubarIconStyle.Valid() {
		out.Behavior.MenubarIconStyle = d.Behavior.MenubarIconStyle
	}
	if !out.Behavior.UpdatePolicy.Valid() {
		out.Behavior.UpdatePolicy = d.Behavior.UpdatePolicy
	}
	if !out.Behavior.CrashReports.Valid() {
		out.Behavior.CrashReports = d.Behavior.CrashReports
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
	out.Appearance.ApparitionDelayMs = clampInt(out.Appearance.ApparitionDelayMs, 0, 2000)

	// Blacklist: drop empty matchers and default the hide mode. Keep the slice
	// non-nil so it marshals as [] (the preferences UI maps over it).
	bl := []BlacklistEntry{}
	for _, e := range out.Filters.AppBlacklist {
		if e.Match == "" {
			continue
		}
		if !e.Hide.Valid() {
			e.Hide = HideAlways
		}
		bl = append(bl, e)
	}
	out.Filters.AppBlacklist = bl

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
		if sc.Scope.Order != "" && !sc.Scope.Order.Valid() {
			sc.Scope.Order = ""
		}
		if sc.WhenReleased != "" && !sc.WhenReleased.Valid() {
			sc.WhenReleased = ""
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
	if s.Version < 2 {
		// v1 -> v2: filter tristates and structured blacklist entries. Both are
		// handled by the types' UnmarshalJSON, so nothing to do here.
		s.Version = 2
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

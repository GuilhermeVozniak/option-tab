package config

// DockSettings controls the optional Dock hover preview surface. Empty Space/
// Screen scope fields inherit the global filters.
type DockSettings struct {
	Enabled         bool          `json:"enabled"`
	HoverDelayMs    int           `json:"hoverDelayMs"`
	DismissDelayMs  int           `json:"dismissDelayMs"`
	HoverSlopPx     int           `json:"hoverSlopPx"`
	BridgePaddingPx int           `json:"bridgePaddingPx"`
	Scope           ShortcutScope `json:"scope"`
	Appearance      Appearance    `json:"appearance"`
}

func dockDefaults(window Appearance) DockSettings {
	a := defaultAppearance()
	a.Theme = window.Theme
	a.AccentColor = window.AccentColor
	a.Style = StyleThumbnails
	a.ThumbnailMaxPx = 240
	a.MaxRows = 2
	a.MaxColumns = 5
	a.PreviewSelected = false
	a.ShowWindowControls = true
	a.FadeOutAnimation = false
	return DockSettings{
		Enabled: false, HoverDelayMs: 300, DismissDelayMs: 250,
		HoverSlopPx: 8, BridgePaddingPx: 12,
		Scope: ShortcutScope{AppScope: AppScopeAll}, Appearance: a,
	}
}

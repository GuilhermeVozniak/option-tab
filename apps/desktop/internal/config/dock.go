package config

import "regexp"

// DockSettings controls the optional Dock hover preview surface. Empty Space/
// Screen scope fields inherit the global filters.
type DockMediaSettings struct {
	Enabled        bool `json:"enabled"`
	MusicEnabled   bool `json:"musicEnabled"`
	SpotifyEnabled bool `json:"spotifyEnabled"`
	RemoteArtwork  bool `json:"remoteArtwork"`
}

type DockSettings struct {
	Media           DockMediaSettings       `json:"media"`
	Enabled         bool                    `json:"enabled"`
	HoverDelayMs    int                     `json:"hoverDelayMs"`
	DismissDelayMs  int                     `json:"dismissDelayMs"`
	HoverSlopPx     int                     `json:"hoverSlopPx"`
	BridgePaddingPx int                     `json:"bridgePaddingPx"`
	CardSpacingPx   int                     `json:"cardSpacingPx"`
	Scope           ShortcutScope           `json:"scope"`
	Appearance      Appearance              `json:"appearance"`
	Input           DockInputSettings       `json:"input"`
	FolderPop       FolderPopSettings       `json:"folderPop"`
	MonitorLock     DockMonitorLockSettings `json:"monitorLock"`
}

type FolderPopSettings struct {
	Enabled bool `json:"enabled"`
}

type DockMonitorLockSettings struct {
	Enabled        bool   `json:"enabled"`
	Target         string `json:"target"`
	DisplayUUID    string `json:"displayUUID"`
	BypassModifier string `json:"bypassModifier"`
}

var displayUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validDisplayUUID(value string) bool { return displayUUIDPattern.MatchString(value) }

type DockInputSettings struct {
	ClickToHide        bool          `json:"clickToHide"`
	ScrollShowHide     bool          `json:"scrollShowHide"`
	ModifiedRightClick bool          `json:"modifiedRightClick"`
	SwipeTowardDock    PointerAction `json:"swipeTowardDock"`
	SwipeAwayFromDock  PointerAction `json:"swipeAwayFromDock"`
	SwipePrevious      PointerAction `json:"swipePrevious"`
	SwipeNext          PointerAction `json:"swipeNext"`
	PreviewDrag        bool          `json:"previewDrag"`
	AeroShakeAction    string        `json:"aeroShakeAction"`
}

func defaultDockInput() DockInputSettings {
	return DockInputSettings{
		SwipeTowardDock: PointerNone, SwipeAwayFromDock: PointerNone,
		SwipePrevious: PointerNone, SwipeNext: PointerNone, AeroShakeAction: "none",
	}
}

func validAeroShakeAction(action string) bool {
	return action == "none" || action == "minimizeOthers" || action == "closeOthers"
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
		CardSpacingPx: 7,
		Scope:         ShortcutScope{AppScope: AppScopeAll}, Appearance: a,
		Input:       defaultDockInput(),
		FolderPop:   FolderPopSettings{Enabled: false},
		MonitorLock: DockMonitorLockSettings{Target: "main", BypassModifier: "option"},
	}
}

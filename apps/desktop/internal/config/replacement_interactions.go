package config

import "slices"

type LauncherInteractions struct {
	Enabled          bool   `json:"enabled"`
	PreciseScroll    bool   `json:"preciseScroll"`
	Pinch            bool   `json:"pinch"`
	Swipe            bool   `json:"swipe"`
	PrimaryAction    string `json:"primaryAction"`
	TowardAction     string `json:"towardAction"`
	PinchAction      string `json:"pinchAction"`
	Haptics          bool   `json:"haptics"`
	LetterNavigation bool   `json:"letterNavigation"`
	EnterActivates   bool   `json:"enterActivates"`
}

func DefaultLauncherInteractions() *LauncherInteractions {
	return &LauncherInteractions{
		PrimaryAction: "next",
		TowardAction:  "showPreview",
		PinchAction:   "showPreview",
	}
}

func validLauncherInteractions(v *LauncherInteractions) bool {
	return v == nil ||
		slices.Contains([]string{"none", "previous", "next"}, v.PrimaryAction) &&
			slices.Contains([]string{"none", "showPreview", "hidePreview"}, v.TowardAction) &&
			slices.Contains([]string{"none", "showPreview", "hidePreview"}, v.PinchAction)
}

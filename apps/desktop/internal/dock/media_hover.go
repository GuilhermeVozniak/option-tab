package dock

import (
	"errors"
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

// MediaProviderForItem never resolves or launches an app. Only explicit exact
// native application identity and individually enabled providers select media.
func MediaProviderForItem(item Item, settings config.DockMediaSettings) platform.MediaProvider {
	if !settings.Enabled || (item.Kind != "" && item.Kind != "app") {
		return ""
	}
	if item.BundleID == "com.apple.Music" && settings.MusicEnabled {
		return platform.MediaMusic
	}
	if item.BundleID == "com.spotify.client" && settings.SpotifyEnabled {
		return platform.MediaSpotify
	}
	return ""
}

func mediaEnabled(s config.DockMediaSettings) bool {
	return s.Enabled && (s.MusicEnabled || s.SpotifyEnabled)
}

func nativeMediaProvider(item *platform.DockItem, s config.DockMediaSettings) platform.MediaProvider {
	if item == nil {
		return ""
	}
	return MediaProviderForItem(Item{Kind: item.Kind, BundleID: item.BundleID}, s)
}

// queryMedia shares the controller's one serialized environment worker. It
// deliberately does not query windows, app presence, or a media provider.
func queryMedia(deps Deps, settings config.Settings, item Item, provider platform.MediaProvider) windowResult {
	result := windowResult{provider: provider}
	if deps.Env == nil {
		result.err = errors.New("dock: media display unavailable")
		return result
	}
	for _, screen := range deps.Env.Screens() {
		if screen.ID == item.ScreenID {
			result.screen = screen.Visible
			if result.screen.Area() == 0 {
				result.screen = screen.Bounds
			}
			break
		}
	}
	if result.screen.Area() == 0 {
		result.err = errors.New("dock: media display unavailable")
		return result
	}
	result.excluded = deps.SelfBundleID != "" && strings.EqualFold(item.BundleID, deps.SelfBundleID)
	for _, entry := range settings.Filters.AppBlacklist {
		if entry.Hide != config.HideWhenNoWindow && entry.Match != "" && (strings.EqualFold(entry.Match, item.BundleID) || strings.EqualFold(entry.Match, item.Title)) {
			result.excluded = true
		}
	}
	return result
}

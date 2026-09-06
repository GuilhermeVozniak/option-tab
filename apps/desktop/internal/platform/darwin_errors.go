//go:build darwin

package platform

import "errors"

var (
	errNoThumbnail = errors.New("platform: no thumbnail available")
	errUnknownKey  = errors.New("platform: unknown hotkey key")
)

// loginItemResult translates SMAppService's native success/failure result.
func loginItemResult(result int) error {
	if result != 1 {
		return errors.New("platform: could not change start at login; check Login Items in System Settings")
	}
	return nil
}

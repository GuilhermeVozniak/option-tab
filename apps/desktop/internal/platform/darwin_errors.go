//go:build darwin

package platform

import "errors"

var (
	errNoThumbnail = errors.New("platform: no thumbnail available")
	errUnknownKey  = errors.New("platform: unknown hotkey key")
)

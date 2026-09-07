//go:build !darwin

package platform

import (
	"context"
	"errors"
)

type unsupportedMediaSource struct{}

func NewMediaSource() MediaProviderSource { return unsupportedMediaSource{} }
func (unsupportedMediaSource) ObserveMedia(context.Context, MediaProvider, func(MediaSample)) error {
	return errors.New("media integration is unsupported on this platform")
}

func (unsupportedMediaSource) RequestMediaPermission(context.Context, MediaProvider) (MediaPermission, error) {
	return MediaPermission{Status: "unsupported"}, errors.New("media integration is unsupported on this platform")
}

func (unsupportedMediaSource) PerformMediaCommandGuarded(context.Context, MediaCommand, func() error) error {
	return errors.New("media integration is unsupported on this platform")
}

func (unsupportedMediaSource) ReadMediaArtwork(context.Context, MediaScope, string) (MediaArtwork, error) {
	return MediaArtwork{Status: "unsupported"}, errors.New("media integration is unsupported on this platform")
}

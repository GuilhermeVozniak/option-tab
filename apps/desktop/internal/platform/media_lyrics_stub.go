//go:build !darwin

package platform

import (
	"context"
	"errors"
)

type unsupportedMediaLyrics struct{}

func NewMediaLyricsSource(path string) MediaLyricsSource {
	return newMediaLyricsSource(path, unsupportedMediaLyrics{})
}

func (unsupportedMediaLyrics) choose(context.Context) (lyricsSelection, error) {
	return lyricsSelection{}, errors.New("lyric file selection is unsupported on this platform")
}

func (unsupportedMediaLyrics) read(context.Context, lyricsRecord) ([]byte, error) {
	return nil, errors.New("lyric file access is unsupported on this platform")
}

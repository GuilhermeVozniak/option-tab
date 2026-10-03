//go:build !darwin

package platform

import "errors"

func NewAudioOutputSource() AudioOutputSource {
	return newAudioOutputSource(func() (audioOutputNative, error) { return nil, errors.New("audio output unsupported") })
}

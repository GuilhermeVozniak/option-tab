package platform

import (
	"context"
	"time"
)

type AudioOutputDevice struct {
	UID, Name      string
	Alive          bool
	OutputChannels int
}

type AudioOutputSnapshot struct {
	Generation, Sequence uint64
	ObservedAt           time.Time
	Status, Reason       string
	Devices              []AudioOutputDevice
	DefaultUID           string
	Volume               *float64
	Muted                *bool
}

// AudioOutputSource observes outputs without opening an audio stream. Selection
// is a separate explicit action tied to a live observation generation and UID.
type AudioOutputSource interface {
	ObserveAudioOutputs(context.Context, func(AudioOutputSnapshot)) error
	SelectAudioOutput(context.Context, uint64, string, func() error) error
}

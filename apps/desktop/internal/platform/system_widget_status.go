package platform

import (
	"context"
	"time"
)

type (
	BatterySnapshot struct {
		Generation, Sequence uint64
		ObservedAt           time.Time
		Status, Reason       string
		Charge               *float64
		Charging             *bool
		PowerSource          string
	}
	NetworkSnapshot struct {
		Generation, Sequence     uint64
		ObservedAt               time.Time
		Status, Reason           string
		Connected                *bool
		Category                 string
		UploadRate, DownloadRate *float64
	}
	BatterySource interface {
		ObserveBattery(context.Context, func(BatterySnapshot)) error
	}
	NetworkSource interface {
		ObserveNetwork(context.Context, bool, func(NetworkSnapshot)) error
	}
)

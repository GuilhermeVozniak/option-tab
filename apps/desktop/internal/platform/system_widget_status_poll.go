package platform

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync/atomic"
	"time"
)

var systemWidgetGeneration atomic.Uint64

type (
	systemCounter struct {
		Identity         string
		Upload, Download uint64
		Valid            bool
		At               time.Time
	}
	systemNetworkState struct {
		Snapshot  NetworkSnapshot
		Interface string
		Index     uint32
	}
	systemNetworkOps struct {
		Status   func() systemNetworkState
		Counters func(systemNetworkState) systemCounter
		Now      func() time.Time
		Ticks    <-chan time.Time
	}
)

func systemRates(a, b systemCounter) (*float64, *float64) {
	dt := b.At.Sub(a.At).Seconds()
	if !a.Valid || !b.Valid || a.Identity == "" || a.Identity != b.Identity || dt <= 0 || dt > 30 || b.Upload < a.Upload || b.Download < a.Download {
		return nil, nil
	}
	up, down := float64(b.Upload-a.Upload)/dt, float64(b.Download-a.Download)/dt
	if math.IsNaN(up) || math.IsNaN(down) || math.IsInf(up, 0) || math.IsInf(down, 0) || up > 1e12 || down > 1e12 {
		return nil, nil
	}
	return &up, &down
}

func copyBattery(s BatterySnapshot) BatterySnapshot {
	if s.Charge != nil {
		v := *s.Charge
		s.Charge = &v
	}
	if s.Charging != nil {
		v := *s.Charging
		s.Charging = &v
	}
	return s
}

func copyNetwork(s NetworkSnapshot) NetworkSnapshot {
	if s.Connected != nil {
		v := *s.Connected
		s.Connected = &v
	}
	if s.UploadRate != nil {
		v := *s.UploadRate
		s.UploadRate = &v
	}
	if s.DownloadRate != nil {
		v := *s.DownloadRate
		s.DownloadRate = &v
	}
	return s
}

func observeSystemBattery(ctx context.Context, emit func(BatterySnapshot), read func() BatterySnapshot, ticks <-chan time.Time) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid battery observer")
	}
	generation := systemWidgetGeneration.Add(1)
	var sequence uint64
	var previous BatterySnapshot
	first := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := copyBattery(read())
		if err := ctx.Err(); err != nil {
			return err
		}
		s.Generation = 0
		s.Sequence = 0
		s.ObservedAt = time.Time{}
		if s.Charge != nil && (math.IsNaN(*s.Charge) || math.IsInf(*s.Charge, 0) || *s.Charge < 0 || *s.Charge > 1) {
			s.Charge = nil
			s.Status = "unavailable"
			s.Reason = "invalidCharge"
		}
		if first || !reflect.DeepEqual(previous, s) {
			previous = copyBattery(s)
			sequence++
			s.Generation = generation
			s.Sequence = sequence
			s.ObservedAt = time.Now()
			emit(copyBattery(s))
			first = false
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ticks:
			if !ok {
				return nil
			}
		}
	}
}

func observeSystemNetwork(ctx context.Context, usage bool, emit func(NetworkSnapshot), ops systemNetworkOps) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid network observer")
	}
	generation := systemWidgetGeneration.Add(1)
	var sequence uint64
	var state systemNetworkState
	var previous NetworkSnapshot
	var counter systemCounter
	var nextStatus, nextUsage time.Time
	var upload, download *float64
	first := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := ops.Now()
		if first || !now.Before(nextStatus) {
			next := ops.Status()
			if next.Interface != state.Interface || next.Index != state.Index {
				counter = systemCounter{}
				upload = nil
				download = nil
			}
			state = next
			nextStatus = ops.Now().Add(2 * time.Second)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		out := copyNetwork(state.Snapshot)
		out.UploadRate = nil
		out.DownloadRate = nil
		if usage && out.Connected != nil && *out.Connected && state.Interface != "" {
			if !ops.Now().Before(nextUsage) {
				current := ops.Counters(state)
				current.At = ops.Now()
				nextUsage = current.At.Add(time.Second)
				upload, download = systemRates(counter, current)
				counter = current
			}
			out.UploadRate, out.DownloadRate = upload, download
		} else {
			counter = systemCounter{}
			upload = nil
			download = nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		out.Generation = 0
		out.Sequence = 0
		out.ObservedAt = time.Time{}
		if first || !reflect.DeepEqual(previous, out) {
			previous = copyNetwork(out)
			sequence++
			out.Generation = generation
			out.Sequence = sequence
			out.ObservedAt = ops.Now()
			emit(copyNetwork(out))
			first = false
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ops.Ticks:
			if !ok {
				return nil
			}
		}
	}
}

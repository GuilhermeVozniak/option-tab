package platform

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync/atomic"
	"time"
)

var launcherEnvironmentGeneration atomic.Uint64

type launcherEnvironmentOps struct {
	Changed  func() bool
	Snapshot func() LauncherEnvironment
	Pointer  func() (float64, float64, bool)
}

// The owning worker serializes native reads and callbacks. Cancellation waits for
// the current bounded native read; no copied result can escape that join.
func observeLauncherEnvironment(ctx context.Context, emit func(LauncherEnvironment), ops launcherEnvironmentOps, interval time.Duration) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid launcher observer")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var snapshot, previous LauncherEnvironment
	var nextRead time.Time
	var sequence, generation uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := time.Now()
		changed := ops.Changed != nil && ops.Changed()
		if changed || !now.Before(nextRead) {
			snapshot = ops.Snapshot()
			snapshot.Displays = slices.Clone(snapshot.Displays)
			if changed || generation == 0 || !reflect.DeepEqual(snapshot, previous) {
				previous = snapshot
				generation = launcherEnvironmentGeneration.Add(1)
			}
			snapshot.Generation = generation
			nextRead = now.Add(500 * time.Millisecond)
		}
		if snapshot.Generation == 0 {
			snapshot.Generation = launcherEnvironmentGeneration.Add(1)
		}
		snapshot.PointerX, snapshot.PointerY, snapshot.PointerKnown = ops.Pointer()
		if err := ctx.Err(); err != nil {
			return err
		}
		sequence++
		out := snapshot
		out.Displays = slices.Clone(snapshot.Displays)
		out.Sequence = sequence
		out.ObservedAt = now
		emit(out)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

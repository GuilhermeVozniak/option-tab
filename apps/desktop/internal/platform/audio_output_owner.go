package platform

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errAudioBusy          = errors.New("audio output source busy")
	errAudioRetired       = errors.New("audio output generation retired")
	audioOutputGeneration atomic.Uint64
)

type (
	audioOutputNative interface {
		Read() AudioOutputSnapshot
		Dirty() bool
		Select(string, func() error) error
		Close()
	}
	audioOutputOwner struct {
		ctx        context.Context
		native     audioOutputNative
		generation uint64
		state      AudioOutputSnapshot
		busy       bool
		actions    sync.WaitGroup
	}
	audioOutputSource struct {
		mu    sync.Mutex
		owner *audioOutputOwner
		start func() (audioOutputNative, error)
	}
)

func newAudioOutputSource(start func() (audioOutputNative, error)) *audioOutputSource {
	return &audioOutputSource{start: start}
}

func copyAudioSnapshot(s AudioOutputSnapshot) AudioOutputSnapshot {
	s.Devices = slices.Clone(s.Devices)
	if s.Volume != nil {
		v := *s.Volume
		s.Volume = &v
	}
	if s.Muted != nil {
		v := *s.Muted
		s.Muted = &v
	}
	return s
}

func sanitizeAudioSnapshot(s AudioOutputSnapshot) AudioOutputSnapshot {
	s = copyAudioSnapshot(s)
	s.Generation = 0
	s.Sequence = 0
	s.ObservedAt = time.Time{}
	if len(s.Devices) > 64 {
		return AudioOutputSnapshot{Status: "unavailable", Reason: "inventoryLimit"}
	}
	seen := map[string]bool{}
	for _, d := range s.Devices {
		if d.UID == "" || len(d.UID) > 1024 || len(d.Name) > 1024 || strings.ContainsRune(d.UID, 0) || seen[d.UID] || !d.Alive || d.OutputChannels < 1 || d.OutputChannels > 1024 {
			return AudioOutputSnapshot{Status: "unavailable", Reason: "invalidInventory"}
		}
		seen[d.UID] = true
	}
	if s.Volume != nil && (math.IsNaN(*s.Volume) || math.IsInf(*s.Volume, 0) || *s.Volume < 0 || *s.Volume > 1) {
		s.Volume = nil
	}
	if s.DefaultUID != "" && !seen[s.DefaultUID] {
		s.DefaultUID = ""
		s.Volume = nil
		s.Muted = nil
		s.Status = "unavailable"
		s.Reason = "defaultUnavailable"
	}
	return s
}

func (s *audioOutputSource) ObserveAudioOutputs(ctx context.Context, emit func(AudioOutputSnapshot)) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid audio output observer")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	owner := &audioOutputOwner{ctx: ctx}
	s.mu.Lock()
	if s.owner != nil {
		s.mu.Unlock()
		return errAudioBusy
	}
	s.owner = owner
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.owner == owner {
			s.owner = nil
		}
		s.mu.Unlock()
	}()
	native, err := s.start()
	if err != nil {
		return err
	}
	owner.native = native
	defer func() { s.mu.Lock(); owner.generation = 0; s.mu.Unlock(); owner.actions.Wait(); native.Close() }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	first := true
	var previous AudioOutputSnapshot
	var sequence uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if first || native.Dirty() {
			state := sanitizeAudioSnapshot(native.Read())
			if err := ctx.Err(); err != nil {
				return err
			}
			if first || !reflect.DeepEqual(state, previous) {
				s.mu.Lock()
				if owner.generation == 0 || state.Status != owner.state.Status || state.DefaultUID != owner.state.DefaultUID || !reflect.DeepEqual(state.Devices, owner.state.Devices) {
					owner.generation = audioOutputGeneration.Add(1)
				}
				owner.state = copyAudioSnapshot(state)
				state.Generation = owner.generation
				s.mu.Unlock()
				previous = copyAudioSnapshot(state)
				previous.Generation = 0
				sequence++
				state.Sequence = sequence
				state.ObservedAt = time.Now()
				if ctx.Err() != nil {
					return ctx.Err()
				}
				emit(copyAudioSnapshot(state))
			}
			if state.Reason == "listenerUnavailable" {
				return errors.New("audio output listener unavailable")
			}
			first = false
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *audioOutputSource) SelectAudioOutput(ctx context.Context, generation uint64, uid string, guard func() error) error {
	if ctx == nil || guard == nil || generation == 0 || uid == "" || len(uid) > 1024 || strings.ContainsRune(uid, 0) {
		return errors.New("invalid audio output selection")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	owner := s.owner
	if owner == nil || owner.generation != generation || owner.ctx.Err() != nil {
		s.mu.Unlock()
		return errAudioRetired
	}
	if owner.busy {
		s.mu.Unlock()
		return errAudioBusy
	}
	owner.busy = true
	owner.actions.Add(1)
	s.mu.Unlock()
	defer func() { s.mu.Lock(); owner.busy = false; s.mu.Unlock(); owner.actions.Done() }()
	current := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		valid := s.owner == owner && owner.ctx.Err() == nil && owner.generation == generation && owner.state.Status == "ready" && slices.ContainsFunc(owner.state.Devices, func(d AudioOutputDevice) bool { return d.UID == uid && d.Alive && d.OutputChannels > 0 })
		s.mu.Unlock()
		if !valid {
			return errAudioRetired
		}
		if err := guard(); err != nil {
			return err
		}
		s.mu.Lock()
		valid = s.owner == owner && owner.ctx.Err() == nil && owner.generation == generation
		s.mu.Unlock()
		if !valid {
			return errAudioRetired
		}
		return ctx.Err()
	}
	if err := current(); err != nil {
		return err
	}
	return owner.native.Select(uid, current)
}

package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// SessionState invalidates work when the interactive desktop becomes unavailable.
// Generation increases for every state change and across observer invocations.
type SessionState struct {
	Generation uint64
	Inactive   bool
	Reason     string
}

// SessionObservationSource is an optional app-lifetime capability, independent
// of Dock configuration. Callbacks must return promptly. Cancellation joins the
// native worker; no callback occurs after ObserveSession returns.
type SessionObservationSource interface {
	ObserveSession(context.Context, func(SessionState)) error
}

type sessionRaw struct {
	revision                                         uint64 // Any native transition invalidates work, even if coalesced back to active.
	valid, onConsole, loginDone                      bool
	lockState                                        int // -1 unavailable private key, 0 unlocked, 1 locked
	sessionEvent, screenEvent, sleepEvent, lockEvent int // 0 unchanged, 1 inactive, 2 active
}

type sessionReducer struct{ resigned, screensAsleep, sleeping, locked bool }

func (r *sessionReducer) reduce(raw sessionRaw) SessionState {
	if raw.valid {
		if !raw.onConsole || !raw.loginDone {
			r.resigned = true
		}
		if raw.lockState >= 0 {
			r.locked = raw.lockState == 1
		}
	}
	if raw.sessionEvent == 1 {
		r.resigned = true
	} else if raw.sessionEvent == 2 && raw.valid && raw.onConsole && raw.loginDone {
		r.resigned = false
	}
	if raw.screenEvent != 0 {
		r.screensAsleep = raw.screenEvent == 1
	}
	if raw.sleepEvent != 0 {
		r.sleeping = raw.sleepEvent == 1
	}
	if raw.lockEvent == 1 {
		r.locked = true
	} else if raw.lockEvent == 2 && raw.valid && raw.lockState != 1 {
		r.locked = false
	}
	reason := ""
	switch {
	case !raw.valid:
		reason = "unavailable"
	case r.resigned:
		reason = "sessionAway"
	case r.locked:
		reason = "locked"
	case r.sleeping:
		reason = "systemAsleep"
	case r.screensAsleep:
		reason = "screensAsleep"
	}
	return SessionState{Inactive: reason != "", Reason: reason}
}

type sessionPoller interface {
	Poll() (sessionRaw, error)
	Close()
}

var sessionGeneration atomic.Uint64

func runSessionObservation(ctx context.Context, emit func(SessionState), factory func() (sessionPoller, error), interval time.Duration) error {
	if ctx == nil || emit == nil || interval <= 0 {
		return errors.New("session observation requires context, callback, and positive interval")
	}
	if ctx.Err() != nil {
		return nil
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make(chan SessionState, 1)
	done := make(chan error, 1)
	go func() {
		defer close(out)
		p, err := factory()
		if err != nil {
			done <- err
			return
		}
		defer p.Close()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var reducer sessionReducer
		var last SessionState
		var lastRevision uint64
		first := true
		for {
			if child.Err() != nil {
				done <- nil
				return
			}
			raw, err := p.Poll()
			if child.Err() != nil {
				done <- nil
				return
			}
			if err != nil {
				raw = sessionRaw{}
			}
			state := reducer.reduce(raw)
			if first || state.Inactive != last.Inactive || state.Reason != last.Reason || raw.revision != lastRevision {
				lastRevision = raw.revision
				first = false
				state.Generation = sessionGeneration.Add(1)
				last = state
				select {
				case out <- state:
				default:
					select {
					case <-out:
					default:
					}
					select {
					case out <- state:
					default:
					}
				}
			}
			select {
			case <-child.Done():
				done <- nil
				return
			case <-ticker.C:
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			cancel()
			for range out {
			}
			return <-done
		case state, ok := <-out:
			if !ok {
				return <-done
			}
			if ctx.Err() == nil {
				emit(state)
			}
		}
	}
}

package platform

import (
	"errors"
	"math"
	"strings"
	"sync"
	"time"
)

type (
	launcherGestureNative interface {
		Policy(uint64, LauncherGesturePolicy, func() bool) error
		Next(uint64) (LauncherGestureEvent, int)
		Valid(uint64, uint64, uint64, uint64, uint64, uint64) bool
		Complete(uint64, uint64, uint64, uint64, uint64, uint64)
	}
	launcherGestureWorker struct {
		mu              sync.Mutex
		token           uint64
		native          launcherGestureNative
		policy          LauncherGesturePolicy
		emit            func(LauncherGestureEvent)
		serial          uint64
		setters         int
		closed, started bool
		wake, done      chan struct{}
	}
)

func newLauncherGestureWorker(token uint64, n launcherGestureNative) *launcherGestureWorker {
	return &launcherGestureWorker{token: token, native: n, wake: make(chan struct{}, 1), done: make(chan struct{})}
}

func (w *launcherGestureWorker) signal() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *launcherGestureWorker) set(p LauncherGesturePolicy, emit func(LauncherGestureEvent)) error {
	if p.Admission == 0 || p.Epoch == 0 || p.Session == 0 || p.Revision == 0 || len(p.DisplayUUID) == 0 || len(p.DisplayUUID) > 127 || strings.ContainsRune(p.DisplayUUID, 0) || p.Bounds.X != 0 || p.Bounds.Y != 0 || p.Bounds.W <= 0 || p.Bounds.H <= 0 || p.Bounds.W > 16384 || p.Bounds.H > 16384 || math.IsNaN(p.Bounds.W) || math.IsNaN(p.Bounds.H) || (p.Enabled && emit == nil) {
		return errors.New("invalid launcher gesture policy")
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrDockPanelClosed
	}
	w.setters++
	serial := w.serial
	w.mu.Unlock()
	err := w.native.Policy(w.token, p, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return !w.closed && w.serial == serial && p.Admission >= w.policy.Admission
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	w.setters--
	w.signal()
	if err != nil {
		return err
	}
	if w.closed || w.serial != serial || p.Admission < w.policy.Admission {
		return ErrDockPanelClosed
	}
	w.policy, w.emit = p, emit
	if !w.started {
		w.started = true
		go w.run()
	}
	w.signal()
	return nil
}

func (w *launcherGestureWorker) retire(closeOwner bool) {
	w.mu.Lock()
	w.serial++
	w.policy.Enabled = false
	w.emit = nil
	if closeOwner && !w.closed {
		w.closed = true
		if !w.started {
			close(w.done)
		}
	}
	w.mu.Unlock()
	w.signal()
}

func (w *launcherGestureWorker) run() {
	defer close(w.done)
	for {
		w.mu.Lock()
		closed := w.closed
		w.mu.Unlock()
		if closed {
			return
		}
		e, status := w.native.Next(w.token)
		if status < 0 {
			return
		}
		if status > 0 {
			w.deliver(e)
			continue
		}
		timer := time.NewTimer(8 * time.Millisecond)
		select {
		case <-w.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

// Next is token-scoped, so it may fetch the native setter's new carrier before
// that setter publishes its Go policy. Keep just that one packet while setters
// finish; retirement wakes this wait and no lock crosses native/user code.
func (w *launcherGestureWorker) deliver(e LauncherGestureEvent) {
	for {
		w.mu.Lock()
		p, emit := w.policy, w.emit
		current := !w.closed && p.Enabled && e.Epoch == p.Epoch && e.Session == p.Session && e.Revision == p.Revision && e.Admission == p.Admission && e.DisplayUUID == p.DisplayUUID
		pending := !w.closed && w.setters > 0 && e.Admission > p.Admission
		w.mu.Unlock()
		if current {
			if emit != nil {
				emit(e)
			}
			return
		}
		if !pending {
			return
		}
		timer := time.NewTimer(8 * time.Millisecond)
		select {
		case <-w.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

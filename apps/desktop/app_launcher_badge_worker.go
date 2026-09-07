package main

import (
	"context"
	"sync"
)

// One observer owns preparation, native reads and publication until it returns.
// Updating retires its publication immediately but joins it before a successor.
type launcherBadgeWorker struct {
	mu         sync.Mutex
	once       sync.Once
	generation uint64
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
	kick       chan struct{}
	done       chan struct{}
	observe    func(context.Context, uint64)
}

func newLauncherBadgeWorker(observe func(context.Context, uint64)) *launcherBadgeWorker {
	return &launcherBadgeWorker{kick: make(chan struct{}, 1), done: make(chan struct{}), observe: observe}
}

func (w *launcherBadgeWorker) update() uint64 {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return 0
	}
	w.generation++
	generation, cancel := w.generation, w.cancel
	w.cancel = nil
	select {
	case w.kick <- struct{}{}:
	default:
	}
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return generation
}

func (w *launcherBadgeWorker) current(generation uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return generation != 0 && !w.closed && w.ctx != nil && w.ctx.Err() == nil && generation == w.generation
}

func (w *launcherBadgeWorker) run(ctx context.Context) {
	w.once.Do(func() {
		w.mu.Lock()
		w.ctx = ctx
		w.mu.Unlock()
		defer func() {
			w.mu.Lock()
			w.closed = true
			cancel := w.cancel
			w.cancel = nil
			w.mu.Unlock()
			if cancel != nil {
				cancel()
			}
			close(w.done)
		}()
		if ctx == nil || w.observe == nil {
			return
		}
		var lastGeneration uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.kick:
			}
			if ctx.Err() != nil {
				return
			}
			w.mu.Lock()
			generation := w.generation
			if generation == lastGeneration {
				w.mu.Unlock()
				continue
			}
			lastGeneration = generation
			child, cancel := context.WithCancel(ctx)
			w.cancel = cancel
			w.mu.Unlock()
			w.observe(child, generation)
			cancel()
			w.mu.Lock()
			if w.generation == generation {
				w.cancel = nil
			}
			w.mu.Unlock()
		}
	})
}

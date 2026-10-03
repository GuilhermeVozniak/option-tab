//go:build darwin

package platform

/*
#cgo darwin LDFLAGS: -framework CoreMedia -framework CoreVideo -framework CoreImage
#include "darwin_stream.h"
#include "darwin.h"
*/
import "C"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"option-tab/internal/domain"
)

type streamDelivery struct {
	frames      chan string
	done        chan struct{}
	once        sync.Once
	unavailable atomic.Bool
	observation windowObservation
}

var (
	streamRegistry sync.Map
	streamToken    atomic.Uint64
)

//export goStreamFrame
func goStreamFrame(token C.uint64_t, data *C.char) {
	if v, ok := streamRegistry.Load(uint64(token)); ok {
		s := v.(*streamDelivery)
		select {
		case s.frames <- C.GoString(data):
		default:
		}
	}
}

//export goStreamDestroyed
func goStreamDestroyed(token C.uint64_t) {
	if v, ok := streamRegistry.Load(uint64(token)); ok {
		s := v.(*streamDelivery)
		retiredWindows.destroyed(s.observation)
		s.unavailable.Store(true)
	}
}

//export goStreamDone
func goStreamDone(token C.uint64_t) {
	if v, ok := streamRegistry.Load(uint64(token)); ok {
		s := v.(*streamDelivery)
		s.once.Do(func() { close(s.done) })
	}
}

func (p *darwinPlatform) StreamWindow(ctx context.Context, id domain.WindowID, px int, frame func(string)) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	owner := C.ot_window_pid(C.uint32_t(id))
	if owner <= 0 {
		return WindowUnavailableError{}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	token := streamToken.Add(1)
	identity := nativeWindowIdentity(id)
	s := &streamDelivery{frames: make(chan string, 1), done: make(chan struct{}), observation: retiredWindows.observe(identity)}
	defer retiredWindows.finish(s.observation)
	streamRegistry.Store(token, s)
	defer streamRegistry.Delete(token)
	C.ot_stream_start(C.uint64_t(token), C.uint32_t(id), C.int(px), C.int(identity.PID), C.uint64_t(identity.StartSec), C.uint64_t(identity.StartUsec))
	for {
		select {
		case url := <-s.frames:
			if ctx.Err() == nil {
				frame(url)
			}
		case <-ticker.C:
			if C.ot_window_pid(C.uint32_t(id)) != owner {
				C.ot_stream_stop(C.uint64_t(token))
				<-s.done
				return WindowUnavailableError{}
			}
		case <-s.done:
			if s.unavailable.Load() {
				return WindowUnavailableError{}
			}
			if C.ot_window_pid(C.uint32_t(id)) != owner {
				return WindowUnavailableError{}
			}
			return errors.New("ScreenCaptureKit stream stopped")
		case <-ctx.Done():
			C.ot_stream_stop(C.uint64_t(token))
			<-s.done
			return ctx.Err()
		}
	}
}

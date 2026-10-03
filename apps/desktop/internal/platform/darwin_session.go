//go:build darwin

package platform

/*
#include "darwin_session.h"
*/
import "C"

import (
	"context"
	"errors"
	"time"
	"unsafe"
)

type nativeSessionPoller struct{ ptr unsafe.Pointer }

func newNativeSessionPoller() (sessionPoller, error) {
	ptr := C.ot_session_observer_create()
	if ptr == nil {
		return nil, errors.New("native session observation unavailable")
	}
	return &nativeSessionPoller{ptr: ptr}, nil
}

func (p *nativeSessionPoller) Poll() (sessionRaw, error) {
	s := C.ot_session_observer_poll(p.ptr)
	return sessionRaw{revision: uint64(s.revision), valid: s.valid != 0, onConsole: s.on_console != 0, loginDone: s.login_done != 0, lockState: int(s.lock_state), sessionEvent: int(s.session_event), screenEvent: int(s.screen_event), sleepEvent: int(s.sleep_event), lockEvent: int(s.lock_event)}, nil
}

func (p *nativeSessionPoller) Close() {
	if p.ptr != nil {
		C.ot_session_observer_destroy(p.ptr)
		p.ptr = nil
	}
}

func (p *darwinPlatform) ObserveSession(ctx context.Context, emit func(SessionState)) error {
	return runSessionObservation(ctx, emit, newNativeSessionPoller, 200*time.Millisecond)
}

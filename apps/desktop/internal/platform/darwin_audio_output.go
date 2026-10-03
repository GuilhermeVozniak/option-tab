//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework CoreAudio
#include <stdlib.h>
#include "darwin_audio_output.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"runtime/cgo"
	"unsafe"
)

type darwinAudioOutput struct{ owner unsafe.Pointer }

func NewAudioOutputSource() AudioOutputSource {
	return newAudioOutputSource(func() (audioOutputNative, error) {
		p := C.ot_audio_output_start()
		if p == nil {
			return nil, errors.New("audio output listener unavailable")
		}
		return &darwinAudioOutput{p}, nil
	})
}

func (n *darwinAudioOutput) Read() AudioOutputSnapshot {
	raw := C.ot_audio_output_read(n.owner)
	if raw == nil {
		return AudioOutputSnapshot{Status: "unavailable", Reason: "nativeReadFailed"}
	}
	defer C.free(unsafe.Pointer(raw))
	var state AudioOutputSnapshot
	if json.Unmarshal([]byte(C.GoString(raw)), &state) != nil {
		return AudioOutputSnapshot{Status: "unavailable", Reason: "invalidNativeSnapshot"}
	}
	return state
}
func (n *darwinAudioOutput) Dirty() bool { return C.ot_audio_output_dirty(n.owner) != 0 }
func (n *darwinAudioOutput) Close()      { C.ot_audio_output_close(n.owner) }

type audioOutputGuard struct {
	guard func() error
	err   error
}

//export goAudioOutputFinalGuard
func goAudioOutputFinalGuard(token C.uintptr_t) C.int {
	g := cgo.Handle(token).Value().(*audioOutputGuard)
	g.err = g.guard()
	if g.err != nil {
		return 0
	}
	return 1
}

func (n *darwinAudioOutput) Select(uid string, guard func() error) error {
	g := &audioOutputGuard{guard: guard}
	h := cgo.NewHandle(g)
	defer h.Delete()
	text := C.CString(uid)
	defer C.free(unsafe.Pointer(text))
	status := int(C.ot_audio_output_select(text, C.uintptr_t(h)))
	if g.err != nil {
		return g.err
	}
	switch status {
	case 0:
		return nil
	case 2:
		return errors.New("audio output target changed")
	case 3:
		return errors.New("audio output selection unsupported")
	case 4:
		return errAudioRetired
	case 5:
		return errors.New("audio output change rejected")
	case 6:
		return errors.New("audio output readback mismatch")
	}
	return errors.New("invalid audio output target")
}

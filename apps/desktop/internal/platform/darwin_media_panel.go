//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_media_panel.h"
*/
import "C"

import (
	"encoding/json"
	"unsafe"
)

type nativeMediaPanelEvents struct{}

func (p *darwinPlatform) CreateMediaPanel(host unsafe.Pointer, session uint64, emit func(MediaPanelEvent)) (DockPanel, error) {
	if host == nil || session == 0 || emit == nil {
		return nil, ErrDockPanelHostClosed
	}
	token := uint64(C.ot_media_panel_create(host, C.uint64_t(session)))
	if token == 0 {
		return nil, ErrDockPanelHostClosed
	}
	return newMediaPanel(newDockPanel(token, mediaPanelNative{}), nativeMediaPanelEvents{}, token, session, emit), nil
}

func (nativeMediaPanelEvents) Next(token uint64) (MediaPanelEvent, int) {
	data := C.ot_media_panel_next(C.uint64_t(token))
	if data == nil {
		return MediaPanelEvent{}, 0
	}
	defer C.free(unsafe.Pointer(data))
	var event MediaPanelEvent
	if err := json.Unmarshal([]byte(C.GoString(data)), &event); err != nil {
		return MediaPanelEvent{}, 0
	}
	return event, 1
}

type mediaPanelNative struct{ darwinDockPanelNative }

func (mediaPanelNative) Close(token uint64) error {
	err := darwinDockPanelNative{}.Close(token)
	C.ot_media_panel_forget(C.uint64_t(token))
	return err
}

//go:build darwin

package platform

/*
#include "darwin_preview_drag.h"
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
	"unsafe"
)

type nativePreviewDrag struct {
	handle     unsafe.Pointer
	stop, done chan struct{}
}

func (p *darwinPlatform) DragPreview(ctx context.Context, request PreviewDragRequest) (PreviewDragResult, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return runPreviewDrag(ctx, request, func() (previewDragNative, error) { return newNativePreviewDrag(false) })
}

func newNativePreviewDrag(test bool) (*nativePreviewDrag, error) {
	mode := C.int(0)
	if test {
		mode = 1
	}
	h := C.ot_preview_drag_create(mode)
	if h == nil {
		return nil, fmt.Errorf("preview drag allocation failed")
	}
	return &nativePreviewDrag{handle: h}, nil
}

func previewDragError(code C.int) error {
	if code == 0 {
		return nil
	}
	return fmt.Errorf("native preview drag refused (%d)", int(code))
}

func (n *nativePreviewDrag) Prepare(r PreviewDragRequest) (previewDragSize, error) {
	var size C.OTPreviewDragSize
	err := previewDragError(C.ot_preview_drag_prepare(n.handle, C.uint32_t(r.WindowID), C.int(r.AppID), &size))
	return previewDragSize{W: float64(size.width), H: float64(size.height)}, err
}

func (n *nativePreviewDrag) Start() error {
	n.stop = make(chan struct{})
	n.done = make(chan struct{})
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(n.done)
		defer C.ot_preview_drag_stop_tap(n.handle)
		err := previewDragError(C.ot_preview_drag_start(n.handle))
		ready <- err
		if err != nil {
			return
		}
		for {
			select {
			case <-n.stop:
				return
			default:
				C.ot_preview_drag_pump(n.handle)
			}
		}
	}()
	return <-ready
}

func (n *nativePreviewDrag) Poll() (previewDragSample, error) {
	C.ot_preview_drag_check(n.handle)
	s := C.ot_preview_drag_sample(n.handle)
	return previewDragSample{Sequence: uint64(s.sequence), X: float64(s.x), Y: float64(s.y), Released: s.released != 0, Cancelled: s.cancelled != 0}, nil
}

func (n *nativePreviewDrag) SetPosition(x, y float64) error {
	return previewDragError(C.ot_preview_drag_position(n.handle, C.double(x), C.double(y)))
}
func (n *nativePreviewDrag) Cancel() { C.ot_preview_drag_cancel(n.handle) }
func (n *nativePreviewDrag) Close() {
	n.Cancel()
	if n.stop != nil {
		close(n.stop)
		<-n.done
	}
	C.ot_preview_drag_destroy(n.handle)
}

func (n *nativePreviewDrag) testEvent(kind int, x, y float64, key int) bool {
	return C.ot_preview_drag_test_event(n.handle, C.int(kind), C.double(x), C.double(y), C.int(key)) == 1
}

func (n *nativePreviewDrag) testButton(down bool) {
	v := C.int(0)
	if down {
		v = 1
	}
	C.ot_preview_drag_test_button(n.handle, v)
}
func (n *nativePreviewDrag) testWrites() int { return int(C.ot_preview_drag_test_writes(n.handle)) }

func (n *nativePreviewDrag) testScreens(current, registration uint64) {
	C.ot_preview_drag_test_screens(n.handle, C.uint64_t(current), C.uint64_t(registration))
}

func (n *nativePreviewDrag) testSyntheticUp() bool {
	return C.ot_preview_drag_test_synthetic_up(n.handle) == 1
}

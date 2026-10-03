package platform

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"

	"option-tab/internal/domain"
)

// PreviewDragRequest identifies an already admitted preview gesture. App owns
// cancellation of the exact Session/GestureID pair, including early release.
// Pointer coordinates are global top-left logical points. Grab coordinates are
// normalized inside the preview and clamp to 0...1; they are not pixel offsets.
type PreviewDragRequest struct {
	Session, GestureID uint64
	WindowID           domain.WindowID
	AppID              domain.AppID
	PointerX, PointerY float64
	GrabX, GrabY       float64
}
type PreviewDragResult struct{ FinalX, FinalY float64 }

// PreviewDragger blocks until release/cancellation and native resource cleanup.
// Ordinary Dock hover hide must not cancel an already owned off-panel drag.
type PreviewDragger interface {
	DragPreview(context.Context, PreviewDragRequest) (PreviewDragResult, error)
}

type (
	previewDragSize   struct{ W, H float64 }
	previewDragSample struct {
		Sequence            uint64
		X, Y                float64
		Released, Cancelled bool
	}
	previewDragNative interface {
		Prepare(PreviewDragRequest) (previewDragSize, error)
		Start() error
		Poll() (previewDragSample, error)
		SetPosition(float64, float64) error
		Cancel()
		Close()
	}
)

var previewDragBusy atomic.Bool

func runPreviewDrag(ctx context.Context, request PreviewDragRequest, factory func() (previewDragNative, error)) (PreviewDragResult, error) {
	if ctx == nil || factory == nil || request.Session == 0 || request.GestureID == 0 || request.WindowID == 0 || request.AppID <= 0 || !finitePreviewPoint(request.PointerX, request.PointerY, request.GrabX, request.GrabY) {
		return PreviewDragResult{}, errors.New("invalid preview drag request")
	}
	if err := ctx.Err(); err != nil {
		return PreviewDragResult{}, err
	}
	if !previewDragBusy.CompareAndSwap(false, true) {
		return PreviewDragResult{}, errors.New("another preview drag is active")
	}
	defer previewDragBusy.Store(false)
	native, err := factory()
	if err != nil {
		return PreviewDragResult{}, err
	}
	stopWatch, watchDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			native.Cancel()
		case <-stopWatch:
		}
	}()
	defer func() { close(stopWatch); <-watchDone; native.Close() }()
	size, err := native.Prepare(request)
	if err != nil {
		return PreviewDragResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return PreviewDragResult{}, err
	}
	if !finitePreviewPoint(size.W, size.H) || size.W <= 0 || size.H <= 0 {
		return PreviewDragResult{}, errors.New("invalid native window size")
	}
	if err := native.Start(); err != nil {
		return PreviewDragResult{}, err
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var last PreviewDragResult
	var sequence uint64
	for {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		sample, err := native.Poll()
		if err != nil {
			return last, err
		}
		if sample.Cancelled {
			return last, errors.New("preview drag cancelled")
		}
		if sample.Sequence > sequence {
			sequence = sample.Sequence
			destination, err := previewDragDestination(request, size, sample.X, sample.Y)
			if err != nil {
				return last, err
			}
			if err := ctx.Err(); err != nil {
				return last, err
			}
			// Native rechecks its atomic cancellation flag after any AX lookups and
			// immediately before the sole position write as well.
			if err := native.SetPosition(destination.FinalX, destination.FinalY); err != nil {
				return last, err
			}
			last = destination
			if sample.Released {
				return last, nil
			}
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-ticker.C:
		}
	}
}

func previewDragDestination(request PreviewDragRequest, size previewDragSize, x, y float64) (PreviewDragResult, error) {
	if !finitePreviewPoint(x, y, size.W, size.H, request.GrabX, request.GrabY) || size.W <= 0 || size.H <= 0 {
		return PreviewDragResult{}, errors.New("invalid preview drag geometry")
	}
	destination := PreviewDragResult{FinalX: x - max(0, min(1, request.GrabX))*size.W, FinalY: y - max(0, min(1, request.GrabY))*size.H}
	if !finitePreviewPoint(destination.FinalX, destination.FinalY) {
		return PreviewDragResult{}, errors.New("nonfinite preview drag destination")
	}
	return destination, nil
}

func finitePreviewPoint(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

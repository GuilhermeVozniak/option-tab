package platform

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakePreviewDrag struct {
	mu                         sync.Mutex
	entered, release           chan struct{}
	samples                    []previewDragSample
	positions                  []PreviewDragResult
	started, closed, cancelled atomic.Bool
	startErr, moveErr          error
}

func (f *fakePreviewDrag) Prepare(PreviewDragRequest) (previewDragSize, error) {
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	return previewDragSize{W: 800, H: 600}, nil
}
func (f *fakePreviewDrag) Start() error { f.started.Store(true); return f.startErr }
func (f *fakePreviewDrag) Poll() (previewDragSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.samples) == 0 {
		return previewDragSample{}, nil
	}
	s := f.samples[0]
	f.samples = f.samples[1:]
	return s, nil
}

func (f *fakePreviewDrag) SetPosition(x, y float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.positions = append(f.positions, PreviewDragResult{x, y})
	return f.moveErr
}
func (f *fakePreviewDrag) Cancel() { f.cancelled.Store(true) }
func (f *fakePreviewDrag) Close()  { f.closed.Store(true) }
func dragRequest() PreviewDragRequest {
	return PreviewDragRequest{Session: 1, GestureID: 2, WindowID: 101, AppID: 10, PointerX: 50, PointerY: 60, GrabX: 0.25, GrabY: 0.5}
}

func TestPreviewDragUsesNormalizedDesktopAnchorAndLatestRelease(t *testing.T) {
	f := &fakePreviewDrag{samples: []previewDragSample{{Sequence: 1, X: 900, Y: 700}, {Sequence: 2, X: -100, Y: 300, Released: true}}}
	result, err := runPreviewDrag(context.Background(), dragRequest(), func() (previewDragNative, error) { return f, nil })
	if err != nil || result.FinalX != -300 || result.FinalY != 0 {
		t.Fatalf("wrong desktop destination %+v %v", result, err)
	}
	if !f.started.Load() || !f.closed.Load() || len(f.positions) != 2 {
		t.Fatalf("wrong lifecycle %+v", f)
	}
}

func TestPreviewDragCancellationDuringLookupNeverInstallsCapture(t *testing.T) {
	f := &fakePreviewDrag{entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runPreviewDrag(ctx, dragRequest(), func() (previewDragNative, error) { return f, nil })
		done <- err
	}()
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		t.Fatal("prepare not reached")
	}
	cancel()
	close(f.release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("wrong cancellation %v", err)
	}
	if f.started.Load() || len(f.positions) != 0 || !f.closed.Load() {
		t.Fatal("late capture/write after cancellation")
	}
}

func TestPreviewDragRejectsSecondConcurrentOwner(t *testing.T) {
	f := &fakePreviewDrag{entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := runPreviewDrag(ctx, dragRequest(), func() (previewDragNative, error) { return f, nil })
		done <- err
	}()
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		t.Fatal("prepare not reached")
	}
	called := false
	_, err := runPreviewDrag(context.Background(), dragRequest(), func() (previewDragNative, error) { called = true; return &fakePreviewDrag{}, nil })
	cancel()
	close(f.release)
	<-done
	if err == nil || called {
		t.Fatal("second owner was admitted")
	}
}

func TestPreviewDragInvalidGeometryAndNativeRefusal(t *testing.T) {
	for _, change := range []func(*PreviewDragRequest){func(r *PreviewDragRequest) { r.Session = 0 }, func(r *PreviewDragRequest) { r.GestureID = 0 }, func(r *PreviewDragRequest) { r.WindowID = 0 }, func(r *PreviewDragRequest) { r.AppID = 0 }, func(r *PreviewDragRequest) { r.PointerX = math.NaN() }, func(r *PreviewDragRequest) { r.GrabY = math.Inf(1) }} {
		r := dragRequest()
		change(&r)
		called := false
		_, err := runPreviewDrag(context.Background(), r, func() (previewDragNative, error) { called = true; return &fakePreviewDrag{}, nil })
		if err == nil || called {
			t.Fatal("invalid request reached native source")
		}
	}
	for _, f := range []*fakePreviewDrag{{startErr: errors.New("button already up")}, {samples: []previewDragSample{{Sequence: 1, X: 1, Y: 2}}, moveErr: errors.New("AX position refused")}, {samples: []previewDragSample{{Sequence: 1, Cancelled: true}}}} {
		_, err := runPreviewDrag(context.Background(), dragRequest(), func() (previewDragNative, error) { return f, nil })
		if err == nil || !f.closed.Load() {
			t.Fatal("native refusal not propagated/cleaned")
		}
	}
}

func TestPreviewDragClampsGrabWithoutResizing(t *testing.T) {
	r := dragRequest()
	r.GrabX = -1
	r.GrabY = 2
	got, err := previewDragDestination(r, previewDragSize{W: 800, H: 600}, 100, 200)
	if err != nil || got.FinalX != 100 || got.FinalY != -400 {
		t.Fatalf("wrong clamped anchor %+v %v", got, err)
	}
}

func TestPreviewDragRejectsOverflowedDestination(t *testing.T) {
	r := dragRequest()
	r.GrabX = 1
	if _, err := previewDragDestination(r, previewDragSize{W: math.MaxFloat64, H: 600}, -math.MaxFloat64, 0); err == nil {
		t.Fatal("nonfinite destination accepted")
	}
}

//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_dock.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"option-tab/internal/domain"
)

type (
	dockApp struct {
		PID      int    `json:"pid"`
		Path     string `json:"path"`
		BundleID string `json:"bundleId"`
	}
	dockScreen struct {
		ID     domain.ScreenID `json:"id"`
		Bounds domain.Bounds   `json:"bounds"`
		Scale  float64         `json:"scale"`
	}
	dockRawItem struct {
		PID       int           `json:"pid"`
		Role      string        `json:"role"`
		Subrole   string        `json:"subrole"`
		URL       string        `json:"url"`
		Path      string        `json:"path"`
		BundleID  string        `json:"bundleId"`
		Title     string        `json:"title"`
		Bounds    domain.Bounds `json:"bounds"`
		Container domain.Bounds `json:"container"`
		Apps      []dockApp     `json:"apps"`
	}
)

type dockRaw struct {
	Generation  uint64       `json:"generation"`
	DockPID     int          `json:"dockPid"`
	PointerX    float64      `json:"pointerX"`
	PointerY    float64      `json:"pointerY"`
	Status      string       `json:"status"`
	Orientation string       `json:"orientation"`
	Screens     []dockScreen `json:"screens"`
	Item        *dockRawItem `json:"item"`
	Diagnostic  string       `json:"diagnostic"`
}

type (
	dockPoller interface {
		Poll() (dockRaw, error)
		Close()
	}
	nativeDockPoller struct{ ptr unsafe.Pointer }
)

func newNativeDockPoller() (dockPoller, error) {
	p := C.ot_dock_observer_create()
	if p == nil {
		return nil, errors.New("could not create Dock observer")
	}
	return &nativeDockPoller{ptr: p}, nil
}

func (p *nativeDockPoller) Poll() (dockRaw, error) {
	data := C.ot_dock_observer_poll(p.ptr)
	if data == nil {
		return dockRaw{}, errors.New("dock observation unavailable")
	}
	defer C.free(unsafe.Pointer(data))
	var raw dockRaw
	err := json.Unmarshal([]byte(C.GoString(data)), &raw)
	return raw, err
}

func (p *nativeDockPoller) Close() {
	if p.ptr != nil {
		C.ot_dock_observer_destroy(p.ptr)
		p.ptr = nil
	}
}

var dockObservationSession atomic.Uint64

// ObserveDock blocks until cancellation. The callback must return promptly;
// polling never waits on it, and no callback occurs after this method returns.
func (p *darwinPlatform) ObserveDock(ctx context.Context, emit func(DockObservation)) error {
	return runDockObservation(ctx, emit, newNativeDockPoller, 50*time.Millisecond)
}

func runDockObservation(ctx context.Context, emit func(DockObservation), factory func() (dockPoller, error), interval time.Duration) error {
	if ctx == nil || emit == nil {
		return errors.New("dock observation requires context and callback")
	}
	if ctx.Err() != nil {
		return nil
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	observations := make(chan DockObservation, 1)
	done := make(chan error, 1)
	epoch := dockObservationSession.Add(1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(observations)
		poller, err := factory()
		if err != nil {
			done <- err
			return
		}
		defer poller.Close()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var sequence uint64
		lastAmbiguousPath := ""
		lastDiagnostic := ""
		trace := os.Getenv("OPTION_TAB_DOCK_OBSERVER_DIAGNOSTICS") == "1"
		for {
			if child.Err() != nil {
				done <- nil
				return
			}
			observedAt := time.Now()
			raw, err := poller.Poll()
			if child.Err() != nil {
				done <- nil
				return
			}
			if err != nil {
				raw = dockRaw{Generation: sequence + 1, Status: "dockUnavailable"}
			}
			sequence++
			if trace && raw.Diagnostic != lastDiagnostic {
				log.Printf("Dock observer: %s (status=%s pointer=%.1f,%.1f screens=%+v)", raw.Diagnostic, raw.Status, raw.PointerX, raw.PointerY, raw.Screens)
				lastDiagnostic = raw.Diagnostic
			}
			observation := mapDockObservation(raw, sequence, epoch)
			observation.ObservedAt = observedAt
			if raw.Item != nil && len(dockMatchingApps(raw.Item)) > 1 {
				if raw.Item.Path != lastAmbiguousPath {
					log.Printf("Dock observer: ambiguous live application identity for %q; ignoring icon", raw.Item.Path)
				}
				lastAmbiguousPath = raw.Item.Path
			} else {
				lastAmbiguousPath = ""
			}
			select {
			case observations <- observation:
			default:
				select {
				case <-observations:
				default:
				}
				select {
				case observations <- observation:
				default:
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
			for range observations {
			}
			return <-done
		case observation, ok := <-observations:
			if !ok {
				return <-done
			}
			if ctx.Err() == nil {
				emit(observation)
			}
		}
	}
}

func mapDockObservation(raw dockRaw, sequence, epoch uint64) DockObservation {
	out := DockObservation{DockPID: raw.DockPID, Sequence: sequence, Generation: (epoch << 32) | raw.Generation, PointerX: raw.PointerX, PointerY: raw.PointerY, Status: raw.Status}
	if raw.Status != "ready" || raw.Item == nil {
		return out
	}
	item := raw.Item
	if raw.DockPID <= 0 || item.PID != raw.DockPID || item.Role != "AXDockItem" || item.Subrole != "AXApplicationDockItem" || !validDockBounds(item.Bounds) {
		return out
	}
	u, err := url.Parse(item.URL)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || !filepath.IsAbs(item.Path) || !strings.HasSuffix(strings.ToLower(strings.TrimRight(item.Path, "/")), ".app") {
		return out
	}
	matches := dockMatchingApps(item)
	if len(matches) > 1 {
		return out
	} // ambiguous icon cannot select an arbitrary PID
	var pid domain.AppID
	if len(matches) == 1 {
		pid = domain.AppID(matches[0].PID)
	}
	screen, ok := dockItemScreen(item.Bounds, raw.Screens)
	if !ok {
		return out
	}
	out.Item = &DockItem{AppID: pid, BundleID: item.BundleID, Path: item.Path, Title: item.Title, Bounds: item.Bounds, ScreenID: screen.ID, Edge: dockItemEdge(item.Bounds, item.Container, screen.Bounds, raw.Orientation)}
	return out
}

func dockMatchingApps(item *dockRawItem) []dockApp {
	var exact, bundle []dockApp
	for _, app := range item.Apps {
		if app.PID <= 0 {
			continue
		}
		if filepath.Clean(app.Path) == filepath.Clean(item.Path) {
			exact = append(exact, app)
		}
		if item.BundleID != "" && app.BundleID == item.BundleID {
			bundle = append(bundle, app)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = bundle
	}
	return matches
}

func validDockBounds(b domain.Bounds) bool {
	return b.W > 0 && b.H > 0 && !math.IsNaN(b.X) && !math.IsNaN(b.Y) && !math.IsNaN(b.W) && !math.IsNaN(b.H) && !math.IsInf(b.X, 0) && !math.IsInf(b.Y, 0) && !math.IsInf(b.W, 0) && !math.IsInf(b.H, 0)
}

func dockItemScreen(icon domain.Bounds, screens []dockScreen) (dockScreen, bool) {
	x, y := icon.Center()
	var best dockScreen
	area := 0.0
	for _, s := range screens {
		if !validDockBounds(s.Bounds) {
			continue
		}
		if s.Bounds.ContainsPoint(x, y) {
			return s, true
		}
		intersection := math.Max(0, math.Min(icon.X+icon.W, s.Bounds.X+s.Bounds.W)-math.Max(icon.X, s.Bounds.X)) * math.Max(0, math.Min(icon.Y+icon.H, s.Bounds.Y+s.Bounds.H)-math.Max(icon.Y, s.Bounds.Y))
		if intersection > area {
			best = s
			area = intersection
		}
	}
	return best, area > 0
}

func dockItemEdge(icon, container, screen domain.Bounds, preference string) string {
	b := icon
	if validDockBounds(container) {
		b = container
	}
	x, y := b.Center()
	distances := map[string]float64{"left": math.Abs(x - screen.X), "right": math.Abs(screen.X + screen.W - x), "bottom": math.Abs(screen.Y + screen.H - y)}
	best := "bottom"
	for _, edge := range []string{"left", "right"} {
		if distances[edge] < distances[best] {
			best = edge
		}
	}
	if d, ok := distances[preference]; ok && math.Abs(d-distances[best]) <= 2 {
		return preference
	}
	return best
}

var _ DockObservationSource = (*darwinPlatform)(nil)

//go:build darwin

package platform

/*
#include "darwin_dock_panel.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"sync"
	"time"
	"unsafe"

	"option-tab/internal/domain"
)

type darwinDockPanelNative struct{}

func (p *darwinPlatform) CreateDockPanel(host unsafe.Pointer) (DockPanel, error) {
	if host == nil {
		return nil, ErrDockPanelHostClosed
	}
	token := uint64(C.ot_dock_panel_create(host))
	if token == 0 {
		return nil, ErrDockPanelHostClosed
	}
	return newDarwinWheelPanel(newDockPanel(token, darwinDockPanelNative{}), darwinPanelWheelNative{}), nil
}

func (darwinDockPanelNative) Show(token uint64, b domain.Bounds) error {
	if C.ot_dock_panel_show(C.uint64_t(token), C.double(b.X), C.double(b.Y), C.double(b.W), C.double(b.H)) == 0 {
		return ErrDockPanelHostClosed
	}
	return nil
}

func (darwinDockPanelNative) Hide(token uint64) error {
	if C.ot_dock_panel_hide(C.uint64_t(token)) == 0 {
		return ErrDockPanelHostClosed
	}
	return nil
}

func (darwinDockPanelNative) Close(token uint64) error {
	C.ot_dock_panel_close(C.uint64_t(token))
	return nil
}

type panelWheelNative interface {
	SetPolicy(uint64, DockPanelWheelPolicy) error
	Next(uint64) (DockPanelWheelEvent, int)
	Valid(uint64, uint64, uint64, uint64) bool
	Complete(uint64, uint64, uint64, uint64)
}
type darwinWheelPanel struct {
	*dockPanel
	wheel                                   panelWheelNative
	wheelMu                                 sync.Mutex
	wheelEmit                               func(DockPanelWheelEvent)
	wheelEnabled, wheelStarted, wheelClosed bool
	wheelWake                               chan struct{}
	wheelDone                               chan struct{}
	wheelDoneOnce                           sync.Once
}

func newDarwinWheelPanel(base *dockPanel, native panelWheelNative) *darwinWheelPanel {
	return &darwinWheelPanel{dockPanel: base, wheel: native, wheelWake: make(chan struct{}, 1), wheelDone: make(chan struct{})}
}

func (p *darwinWheelPanel) SetDockPanelWheelPolicy(policy DockPanelWheelPolicy, emit func(DockPanelWheelEvent)) error {
	if (policy.Enabled && (policy.Session == 0 || policy.Revision == 0 || emit == nil)) || len(policy.Regions) > 256 {
		return errors.New("invalid Dock panel wheel policy")
	}
	policy.Regions = slices.Clone(policy.Regions)
	for _, region := range policy.Regions {
		if region.WindowID == 0 || region.AppID <= 0 || region.Bounds.W <= 0 || region.Bounds.H <= 0 {
			return ErrDockPanelBounds
		}
		for _, v := range []float64{region.Bounds.X, region.Bounds.Y, region.Bounds.W, region.Bounds.H} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return ErrDockPanelBounds
			}
		}
	}
	p.wheelMu.Lock()
	closed := p.wheelClosed
	p.wheelMu.Unlock()
	if closed {
		return ErrDockPanelClosed
	}
	// No Go mutex crosses synchronous AppKit dispatch.
	if err := p.wheel.SetPolicy(p.token, policy); err != nil {
		return p.recordHostLoss(err)
	}
	p.wheelMu.Lock()
	if p.wheelClosed {
		p.wheelMu.Unlock()
		return ErrDockPanelClosed
	}
	if emit != nil {
		p.wheelEmit = emit
	}
	p.wheelEnabled = policy.Enabled
	if !p.wheelStarted {
		p.wheelStarted = true
		go p.runWheel()
	}
	p.wheelMu.Unlock()
	p.wakeWheel()
	return nil
}

func (p *darwinWheelPanel) wakeWheel() {
	select {
	case p.wheelWake <- struct{}{}:
	default:
	}
}

func (p *darwinWheelPanel) runWheel() {
	defer p.wheelDoneOnce.Do(func() { close(p.wheelDone) })
	for {
		for {
			event, status := p.wheel.Next(p.token)
			if status < 0 {
				return
			}
			if status == 0 {
				break
			}
			p.wheelMu.Lock()
			emit := p.wheelEmit
			p.wheelMu.Unlock()
			// The native UI callback only enqueues copied values. User code runs here.
			if emit != nil {
				emit(event)
			}
		}
		p.wheelMu.Lock()
		enabled, closed := p.wheelEnabled, p.wheelClosed
		p.wheelMu.Unlock()
		if closed {
			return
		}
		delay := time.Second
		if enabled {
			delay = 8 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-p.wheelWake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
	}
}

func (p *darwinWheelPanel) Close() error {
	p.wheelMu.Lock()
	p.wheelClosed = true
	started := p.wheelStarted
	p.wheelMu.Unlock()
	err := p.dockPanel.Close()
	p.wakeWheel()
	if !started {
		p.wheelDoneOnce.Do(func() { close(p.wheelDone) })
	}
	return err
}

func (p *darwinWheelPanel) ValidateDockPanelWheelGesture(session, revision, gesture uint64) bool {
	p.wheelMu.Lock()
	closed := p.wheelClosed
	p.wheelMu.Unlock()
	return !closed && p.wheel.Valid(p.token, session, revision, gesture)
}

func (p *darwinWheelPanel) CompleteDockPanelWheelGesture(session, revision, gesture uint64) {
	p.wheel.Complete(p.token, session, revision, gesture)
}

type darwinPanelWheelNative struct{}

func (darwinPanelWheelNative) SetPolicy(token uint64, policy DockPanelWheelPolicy) error {
	regions := make([]map[string]any, 0, len(policy.Regions))
	for _, r := range policy.Regions {
		regions = append(regions, map[string]any{"window": uint64(r.WindowID), "app": int(r.AppID), "x": r.Bounds.X, "y": r.Bounds.Y, "w": r.Bounds.W, "h": r.Bounds.H})
	}
	payload, err := json.Marshal(map[string]any{"session": policy.Session, "revision": policy.Revision, "enabled": policy.Enabled, "regions": regions})
	if err != nil {
		return err
	}
	text := C.CString(string(payload))
	defer C.free(unsafe.Pointer(text))
	switch C.ot_dock_panel_wheel_policy(C.uint64_t(token), text) {
	case -1:
		return ErrDockPanelHostClosed
	case 0:
		return errors.New("dock panel wheel policy rejected")
	}
	return nil
}

func (darwinPanelWheelNative) Next(token uint64) (DockPanelWheelEvent, int) {
	var value C.OTDockPanelWheelEvent
	status := int(C.ot_dock_panel_wheel_next(C.uint64_t(token), &value))
	if status != 1 {
		return DockPanelWheelEvent{}, status
	}
	phase := func(v C.int) string {
		switch int(v) {
		case 1:
			return "began"
		case 2:
			return "changed"
		case 3:
			return "ended"
		case 4:
			return "cancelled"
		}
		return "none"
	}
	reason := ""
	switch int(value.reason) {
	case 1:
		reason = "policy changed"
	case 2:
		reason = "panel hidden"
	case 3:
		reason = "panel closed"
	case 4:
		reason = "native wheel queue overflow"
	}
	seconds, nanos := math.Modf(float64(value.unixTime))
	return DockPanelWheelEvent{Session: uint64(value.session), Revision: uint64(value.revision), Sequence: uint64(value.sequence), GestureID: uint64(value.gesture), Timestamp: time.Unix(int64(seconds), int64(nanos*1e9)), WindowID: domain.WindowID(value.window), AppID: domain.AppID(value.app), PanelX: float64(value.x), PanelY: float64(value.y), DeltaX: float64(value.dx), DeltaY: float64(value.dy), Owned: value.owned != 0, Precise: value.precise != 0, DirectionInverted: value.inverted != 0, Phase: phase(value.phase), MomentumPhase: phase(value.momentum), Reason: reason}, status
}

func (darwinPanelWheelNative) Valid(token, session, revision, gesture uint64) bool {
	return C.ot_dock_panel_wheel_valid(C.uint64_t(token), C.uint64_t(session), C.uint64_t(revision), C.uint64_t(gesture)) != 0
}

func (darwinPanelWheelNative) Complete(token, session, revision, gesture uint64) {
	C.ot_dock_panel_wheel_complete(C.uint64_t(token), C.uint64_t(session), C.uint64_t(revision), C.uint64_t(gesture))
}

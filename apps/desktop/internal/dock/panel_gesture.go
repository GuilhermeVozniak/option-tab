package dock

import (
	"math"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

const (
	panelGestureLockDistance   = 8.0
	panelGestureActionDistance = 80.0
	panelGestureIdle           = 250 * time.Millisecond
)

type PanelGestureIntent struct {
	Session, Revision, GestureID uint64
	Action                       config.PointerAction
	WindowID                     domain.WindowID
	AppID                        domain.AppID
}

type PanelGestureRecognizer struct {
	edge  string
	input config.DockInputSettings

	session, revision, lastSequence uint64
	gestureID                       uint64
	windowID                        domain.WindowID
	appID                           domain.AppID
	lastAt                          time.Time
	x, y                            float64
	axis                            byte
	active                          bool
	fired                           bool
}

func NewPanelGestureRecognizer(edge string, input config.DockInputSettings) *PanelGestureRecognizer {
	return &PanelGestureRecognizer{edge: edge, input: input}
}

func (r *PanelGestureRecognizer) SetPresentation(session, revision uint64) {
	if r.session == session && r.revision == revision {
		return
	}
	r.Reset()
	r.session, r.revision = session, revision
	r.lastSequence, r.gestureID = 0, 0
	r.lastAt = time.Time{}
}

// Reset retires ownership without allowing delayed events to revive its ID.
func (r *PanelGestureRecognizer) Reset() {
	r.resetGesture()
}

func (r *PanelGestureRecognizer) resetGesture() {
	r.windowID, r.appID = 0, 0
	r.x, r.y, r.axis = 0, 0, 0
	r.active, r.fired = false, false
}

func (r *PanelGestureRecognizer) Step(event platform.DockPanelWheelEvent) *PanelGestureIntent {
	if event.Session == 0 || event.Revision == 0 || event.Session != r.session || event.Revision != r.revision ||
		event.Sequence == 0 || event.Sequence <= r.lastSequence {
		return nil
	}
	r.lastSequence = event.Sequence
	if event.GestureID == 0 || event.GestureID < r.gestureID {
		return nil
	}
	fresh := event.GestureID > r.gestureID
	if fresh {
		r.resetGesture()
		r.gestureID = event.GestureID
	}
	if !event.Owned || !event.Precise || event.WindowID == 0 || event.AppID <= 0 ||
		!finiteInput(event.DeltaX, event.DeltaY) || event.Timestamp.IsZero() ||
		(!r.lastAt.IsZero() && event.Timestamp.Before(r.lastAt)) ||
		(event.MomentumPhase != "" && event.MomentumPhase != "none") ||
		(event.Phase != "began" && event.Phase != "changed") {
		r.resetGesture()
		return nil
	}
	if !fresh && !r.lastAt.IsZero() && event.Timestamp.Sub(r.lastAt) > panelGestureIdle {
		r.resetGesture()
	}
	r.lastAt = event.Timestamp
	if fresh {
		r.windowID, r.appID = event.WindowID, event.AppID
		r.active = true
	}
	if !r.active {
		return nil
	}
	if event.WindowID != r.windowID || event.AppID != r.appID {
		r.resetGesture()
		return nil
	}
	if r.fired {
		return nil
	}
	switch r.axis {
	case 0:
		r.x = accumulateSigned(r.x, event.DeltaX)
		r.y = accumulateSigned(r.y, event.DeltaY)
		if math.Max(math.Abs(r.x), math.Abs(r.y)) < panelGestureLockDistance {
			return nil
		}
		if math.Abs(r.x) > math.Abs(r.y) {
			r.axis = 'x'
		} else {
			r.axis = 'y'
		}
	case 'x':
		r.x = accumulateSigned(r.x, event.DeltaX)
	default:
		r.y = accumulateSigned(r.y, event.DeltaY)
	}
	value := r.y
	if r.axis == 'x' {
		value = r.x
	}
	if math.Abs(value) < panelGestureActionDistance {
		return nil
	}
	r.fired = true
	action := r.action(r.axis, value)
	if action == config.PointerNone {
		return nil
	}
	return &PanelGestureIntent{
		Session: r.session, Revision: r.revision, GestureID: r.gestureID,
		Action: action, WindowID: r.windowID, AppID: r.appID,
	}
}

func accumulateSigned(total, delta float64) float64 {
	if delta == 0 {
		return total
	}
	if total != 0 && math.Signbit(total) != math.Signbit(delta) {
		return delta
	}
	return total + delta
}

func (r *PanelGestureRecognizer) action(axis byte, value float64) config.PointerAction {
	if r.edge == "left" || r.edge == "right" {
		if axis == 'y' {
			if value > 0 {
				return r.input.SwipePrevious
			}
			return r.input.SwipeNext
		}
		toward := value < 0
		if r.edge == "right" {
			toward = value > 0
		}
		if toward {
			return r.input.SwipeTowardDock
		}
		return r.input.SwipeAwayFromDock
	}
	if axis == 'x' {
		if value < 0 {
			return r.input.SwipePrevious
		}
		return r.input.SwipeNext
	}
	if value < 0 {
		return r.input.SwipeTowardDock
	}
	return r.input.SwipeAwayFromDock
}

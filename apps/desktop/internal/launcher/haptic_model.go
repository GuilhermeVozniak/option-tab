package launcher

import "time"

const HapticInterval = 40 * time.Millisecond

type HapticLimiter struct {
	scope  Scope
	target string
	last   time.Time
}

// Allow consumes observed transitions even when rate-limited; there is no delayed tick.
func (r *HapticLimiter) Allow(scope Scope, enabled, interactionEnabled bool, target string, at time.Time) bool {
	if r.scope != scope {
		r.scope = scope
		r.target = ""
	}
	if !enabled || !interactionEnabled {
		r.target = ""
		return false
	}
	changed := r.target != target
	r.target = target
	if !enabled || !interactionEnabled || !validGestureScope(scope) || target == "" || !changed || at.IsZero() || (!r.last.IsZero() && at.Sub(r.last) < HapticInterval) {
		return false
	}
	r.last = at
	return true
}

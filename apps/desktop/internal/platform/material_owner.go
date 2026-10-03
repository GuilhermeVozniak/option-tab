package platform

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
)

var (
	ErrMaterialInvalid     = errors.New("invalid native material style")
	ErrMaterialRetired     = errors.New("native material owner retired")
	ErrMaterialUnavailable = errors.New("native material unavailable")
)

type materialNative interface {
	Apply(uint64, MaterialStyle, func() bool) error
	Retire(uint64, MaterialScope, int, func() bool) error
	Close(uint64) error
}
type materialSurface struct {
	mu               sync.Mutex
	token, operation uint64
	scope            MaterialScope
	retired, closed  bool
	native           materialNative
}

func newMaterialSurface(token uint64, n materialNative) *materialSurface {
	return &materialSurface{token: token, native: n}
}

func validMaterial(s MaterialStyle) bool {
	if s.Scope.Session == 0 || s.Scope.Revision == 0 || s.CornerRadiusPx < 0 || s.CornerRadiusPx > 64 || !slices.Contains([]string{"system", "light", "dark"}, s.Theme) {
		return false
	}
	if !s.Enabled {
		return true
	}
	for _, v := range []float64{s.Rect.X, s.Rect.Y, s.Rect.W, s.Rect.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 65536 {
			return false
		}
	}
	return s.Rect.W > 0 && s.Rect.H > 0 && s.Rect.X+s.Rect.W <= 65536 && s.Rect.Y+s.Rect.H <= 65536
}

func materialResult(scope MaterialScope, state string, err error) MaterialStatus {
	r := MaterialStatus{Session: scope.Session, Revision: scope.Revision, State: state}
	if err != nil {
		r.State = "unavailable"
		switch {
		case errors.Is(err, ErrMaterialInvalid):
			r.Reason = "invalidStyle"
		case errors.Is(err, ErrMaterialRetired):
			r.Reason = "retired"
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			r.Reason = "cancelled"
		default:
			r.Reason = "nativeUnavailable"
		}
	}
	return r
}

func (p *materialSurface) current(op uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.closed && p.operation == op
}

func (p *materialSurface) Apply(ctx context.Context, s MaterialStyle) (MaterialStatus, error) {
	fail := func(err error) (MaterialStatus, error) { return materialResult(s.Scope, "unavailable", err), err }
	if ctx == nil || !validMaterial(s) {
		return fail(ErrMaterialInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	p.mu.Lock()
	if p.closed || s.Scope.Session < p.scope.Session || s.Scope.Revision <= p.scope.Revision || (s.Scope.Session == p.scope.Session && p.retired) {
		p.mu.Unlock()
		return fail(ErrMaterialRetired)
	}
	p.scope = s.Scope
	p.retired = false
	p.operation++
	op := p.operation
	p.mu.Unlock()
	guard := func() bool { return ctx.Err() == nil && p.current(op) }
	err := p.native.Apply(p.token, s, guard)
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	if !p.current(op) {
		return fail(ErrMaterialRetired)
	}
	if err != nil {
		return fail(err)
	}
	state := "solid"
	if s.Enabled {
		state = "system"
	}
	return materialResult(s.Scope, state, nil), nil
}

func (p *materialSurface) Retire(scope MaterialScope, fade int) error {
	if scope.Session == 0 || scope.Revision == 0 || (fade != 0 && fade != 180) {
		return ErrMaterialInvalid
	}
	p.mu.Lock()
	if p.closed || scope.Session < p.scope.Session || scope.Revision < p.scope.Revision {
		p.mu.Unlock()
		return ErrMaterialRetired
	}
	if p.retired && scope == p.scope {
		p.mu.Unlock()
		return nil
	}
	p.scope = scope
	p.retired = true
	p.operation++
	op := p.operation
	p.mu.Unlock()
	return p.native.Retire(p.token, scope, fade, func() bool { return p.current(op) })
}

func (p *materialSurface) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.operation++
	p.mu.Unlock()
	return p.native.Close(p.token)
}

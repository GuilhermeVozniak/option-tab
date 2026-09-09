package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/switcher"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type appMaterialOwner struct {
	mu      sync.Mutex
	create  func() (platform.MaterialSurface, error)
	publish func(platform.MaterialStatus)
	wake    chan struct{}
	done    chan struct{}
	style   platform.MaterialStyle
	reason  string
	ctx     context.Context
	cancel  context.CancelFunc
	pending platform.MaterialScope
	fade    int
	serial  uint64
	closed  bool
}

func newAppMaterialOwner(create func() (platform.MaterialSurface, error), publish func(platform.MaterialStatus)) *appMaterialOwner {
	o := &appMaterialOwner{create: create, publish: publish, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go o.run()
	return o
}

func (o *appMaterialOwner) signalLocked() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func (o *appMaterialOwner) apply(style platform.MaterialStyle, reason string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	if o.cancel != nil {
		o.cancel()
	}
	o.ctx, o.cancel = context.WithCancel(context.Background())
	o.style, o.reason = style, reason
	o.serial++
	o.signalLocked()
}

func (o *appMaterialOwner) retire(scope platform.MaterialScope, fade int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	if o.cancel != nil {
		o.cancel()
	}
	o.style = platform.MaterialStyle{}
	if scope.Session >= o.pending.Session {
		o.pending = scope
		o.fade = fade
	}
	o.serial++
	o.signalLocked()
}

func (o *appMaterialOwner) close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.closed = true
	if o.cancel != nil {
		o.cancel()
	}
	o.serial++
	o.signalLocked()
}

func (o *appMaterialOwner) current(serial uint64) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.closed && o.serial == serial && o.ctx != nil && o.ctx.Err() == nil
}

func materialResult(style platform.MaterialStyle, reason string, status platform.MaterialStatus, err error) platform.MaterialStatus {
	result := platform.MaterialStatus{Session: style.Scope.Session, Revision: style.Scope.Revision, State: "unavailable", Reason: "unsupported"}
	if reason != "" {
		result.Reason = reason
		return result
	}
	if err != nil {
		result.Reason = "nativeFailure"
		return result
	}
	if !style.Enabled {
		result.State = "solid"
		result.Reason = ""
		return result
	}
	if status.Session == result.Session && status.Revision == result.Revision && (status.State == "system" || status.State == "solid" || status.State == "unavailable") {
		return status
	}
	return result
}

func (o *appMaterialOwner) run() {
	defer close(o.done)
	var surface platform.MaterialSurface
	for range o.wake {
		o.mu.Lock()
		closed, serial, style, reason, ctx, pending, fade := o.closed, o.serial, o.style, o.reason, o.ctx, o.pending, o.fade
		o.pending = platform.MaterialScope{}
		o.mu.Unlock()
		if closed {
			if surface != nil {
				_ = surface.Close()
			}
			return
		}
		if pending.Session != 0 && surface != nil {
			_ = surface.Retire(pending, fade)
		}
		if style.Scope.Session == 0 || !o.current(serial) {
			continue
		}
		var err error
		if surface == nil && style.Enabled {
			if o.create != nil {
				surface, err = o.create()
			} else {
				err = errors.New("unsupported")
			}
		}
		if !o.current(serial) {
			continue
		}
		status := platform.MaterialStatus{}
		if surface != nil {
			status, err = surface.Apply(ctx, style)
		} else if style.Enabled && err == nil {
			err = errors.New("unsupported")
		}
		if o.current(serial) && o.publish != nil {
			o.publish(materialResult(style, reason, status, err))
		}
	}
}

func validMaterialRect(b domain.Bounds) bool {
	for _, v := range []float64{b.X, b.Y, b.W, b.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 65536 {
			return false
		}
	}
	return b.X >= 0 && b.Y >= 0 && b.W > 0 && b.H > 0 && b.X+b.W <= 65536 && b.Y+b.H <= 65536
}

type dockMaterialRequest struct {
	style   platform.MaterialStyle
	ctx     context.Context
	publish func(platform.MaterialStatus, func() bool)
}

func (d *dockWindow) setMaterial(style platform.MaterialStyle, publish func(platform.MaterialStatus, func() bool)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.setMaterialLocked(style, publish)
}

func (d *dockWindow) showMaterial(bounds domain.Bounds, style platform.MaterialStyle, publish func(platform.MaterialStatus, func() bool)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if !d.visible {
		d.visibility++
	}
	d.visible = true
	d.bounds = bounds
	d.revision++
	d.setMaterialLocked(style, publish)
	d.scheduleLocked()
}

func (d *dockWindow) setMaterialLocked(style platform.MaterialStyle, publish func(platform.MaterialStatus, func() bool)) {
	if d.closed {
		return
	}
	if d.material != nil && d.material.style == style {
		return
	}
	if d.materialCancel != nil {
		d.materialCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	if d.materialDelivery == nil {
		d.materialDelivery = newDockMaterialDelivery()
	}
	d.materialCancel = cancel
	d.material = &dockMaterialRequest{style: style, ctx: ctx, publish: publish}
	d.revision++
	d.scheduleLocked()
}

func (d *dockWindow) clearMaterial() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.clearMaterialLocked()
	d.revision++
	d.scheduleLocked()
}

func (d *dockWindow) clearMaterialLocked() {
	if d.materialCancel != nil {
		d.materialCancel()
		d.materialCancel = nil
	}
	if d.material != nil {
		scope := d.material.style.Scope
		if scope.Session >= d.materialRetire.Session {
			d.materialRetire = scope
		}
		d.material = nil
	}
}

func (d *dockWindow) refreshMaterialLocked() {
	if d.material == nil {
		return
	}
	if d.materialCancel != nil {
		d.materialCancel()
	}
	next := *d.material
	next.ctx, d.materialCancel = context.WithCancel(context.Background())
	d.material = &next
}

func (d *dockWindow) materialCurrent(r *dockWindowResources, request *dockMaterialRequest) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return !d.closed && d.visible && d.current == r && r.window.alive() && d.material == request && request.ctx.Err() == nil
}

// Called only by serialized dockWindow reconcile after Show has established
// native content bounds. Native calls and App publication never hold d.mu.
func (d *dockWindow) reconcileMaterial(r *dockWindowResources, visible bool, revision uint64) {
	d.mu.Lock()
	if d.revision != revision {
		d.mu.Unlock()
		return
	}
	request, retire := d.material, d.materialRetire
	d.materialRetire = platform.MaterialScope{}
	d.mu.Unlock()
	if retire.Session != 0 && r.material != nil {
		_ = r.material.Retire(retire, 0)
		_ = r.material.Close()
		r.material = nil
		d.mu.Lock()
		r.materialApplied = platform.MaterialScope{}
		d.mu.Unlock()
	}
	if !visible || request == nil || !d.materialCurrent(r, request) || r.materialApplied == request.style.Scope {
		return
	}
	var err error
	if r.material == nil {
		if request.publish != nil {
			d.materialDelivery.post(request.publish, platform.MaterialStatus{Session: request.style.Scope.Session, Revision: request.style.Scope.Revision, State: "unavailable", Reason: "preparing"}, func() bool { return d.materialCurrent(r, request) })
		}
		if host, ok := d.host.(platform.PreviewMaterialHost); ok {
			r.material, err = host.CreatePreviewMaterial(r.panel)
		} else {
			err = errors.New("unsupported")
		}
	}
	if !d.materialCurrent(r, request) {
		return
	}
	status := platform.MaterialStatus{}
	if r.material != nil {
		status, err = r.material.Apply(request.ctx, request.style)
	} else if err == nil {
		err = errors.New("unsupported")
	}
	if d.materialCurrent(r, request) {
		d.mu.Lock()
		r.materialApplied = request.style.Scope
		d.mu.Unlock()
		if request.publish != nil {
			d.materialDelivery.post(request.publish, materialResult(request.style, "", status, err), func() bool { return d.materialCurrent(r, request) })
		}
	}
}

func (o *appMaterialOwner) invalidate() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancel != nil {
		o.cancel()
	}
	o.serial++
}

type appSwitcherMaterial struct {
	resizeQueued      bool
	scope             platform.MaterialScope
	owner             *appMaterialOwner
	host              *liveWindow
	hostEpoch         uint64
	session, sequence uint64
	appearance        config.Appearance
	mode              config.SwitcherMode
	visual            config.VisualStyle
	screen            domain.ScreenID
	rect              domain.Bounds
	reported          bool
	status            platform.MaterialStatus
}
type appDockMaterial struct {
	window *dockWindow
	style  platform.MaterialStyle
	status platform.MaterialStatus
}

func (a *App) nextMaterialScopeLocked(session uint64) platform.MaterialScope {
	a.materialRevision++
	return platform.MaterialScope{Session: session, Revision: a.materialRevision}
}

func appearanceMaterial(scope platform.MaterialScope, appearance config.Appearance, rect domain.Bounds) platform.MaterialStyle {
	return platform.MaterialStyle{Scope: scope, Enabled: appearance.Blur, Theme: string(appearance.Theme), CornerRadiusPx: appearance.CornerRadiusPx, Rect: rect}
}

func (a *App) syncSwitcherMaterialLocked(st switcher.State) {
	if !a.switcherVisible || st.Session == 0 || a.visibleSwitcherSession != st.Session {
		return
	}
	r := a.switcherMaterial
	if r == nil || r.host != a.overlay {
		if r != nil {
			r.owner.close()
		}
		r = &appSwitcherMaterial{host: a.overlay}
		a.switcherMaterial = r
		r.owner = newAppMaterialOwner(func() (platform.MaterialSurface, error) {
			host, ok := a.platform.(platform.OverlayMaterialHost)
			if !ok || r.host == nil || !r.host.alive() {
				return nil, errors.New("unsupported")
			}
			native := r.host.native()
			if native == nil {
				return nil, errors.New("hostClosed")
			}
			return host.CreateOverlayMaterial(native)
		}, func(status platform.MaterialStatus) {
			a.viewMu.Lock()
			defer a.viewMu.Unlock()
			if a.switcherMaterial != r || !a.switcherVisible || a.visibleSwitcherSession != status.Session || a.controller.PresentationSession() != status.Session || r.scope.Session != status.Session || r.scope.Revision != status.Revision || !r.host.alive() || r.host.materialVersion() != r.hostEpoch {
				return
			}
			if r.status.State == status.State && r.status.Reason == status.Reason {
				return
			}
			status.Revision = a.nextMaterialScopeLocked(status.Session).Revision
			r.status = status
			a.emit("switcher:material", status)
		})
		r.hostEpoch = r.host.bindMaterial(r.owner)
	}
	changed := r.session != st.Session || r.mode != st.Mode || r.visual != st.Style || r.screen != st.PlacementScreenID || !reflect.DeepEqual(r.appearance, st.Appearance) || r.hostEpoch != r.host.materialVersion()
	if !changed {
		return
	}
	if r.session != 0 && r.session != st.Session {
		r.owner.retire(platform.MaterialScope{Session: r.session, Revision: r.status.Revision}, 0)
	}
	if r.session != st.Session {
		r.sequence = 0
	}
	r.session = st.Session
	r.appearance = st.Appearance
	r.mode = st.Mode
	r.visual = st.Style
	r.screen = st.PlacementScreenID
	r.hostEpoch = r.host.materialVersion()
	r.reported = false
	r.rect = domain.Bounds{}
	a.requestSwitcherMaterialLocked(r)
}

func (a *App) requestSwitcherMaterialLocked(r *appSwitcherMaterial) {
	scope := a.nextMaterialScopeLocked(r.session)
	r.scope = scope
	style := appearanceMaterial(scope, r.appearance, r.rect)
	reason := ""
	state := "solid"
	if r.appearance.Blur {
		state = "unavailable"
		reason = "preparing"
		if !r.reported {
			style.Enabled = false
			reason = "missingReporter"
		}
	}
	r.status = platform.MaterialStatus{Session: scope.Session, Revision: scope.Revision, State: state, Reason: reason}
	a.emit("switcher:material", r.status)
	fallback := ""
	if r.appearance.Blur && !r.reported {
		fallback = "missingReporter"
	}
	r.owner.apply(style, fallback)
}

func (a *App) SetSwitcherMaterialRect(session, stateRevision, sequence uint64, x, y, width, height float64) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	r := a.switcherMaterial
	if r == nil || !a.switcherVisible || session == 0 || session != a.visibleSwitcherSession || session != r.session || a.controller.PresentationSession() != session || stateRevision != a.switcherRevision || sequence == 0 || sequence <= r.sequence || !r.host.alive() {
		return errors.New("material: retired")
	}
	if r.hostEpoch != r.host.materialVersion() {
		return errors.New("material: resized")
	}
	rect := domain.Bounds{X: x, Y: y, W: width, H: height}
	r.sequence = sequence
	if !validMaterialRect(rect) {
		r.reported = false
		r.rect = domain.Bounds{}
		a.requestSwitcherMaterialLocked(r)
		return errors.New("material: invalidRect")
	}
	if r.reported && r.rect == rect {
		return nil
	}
	r.reported = true
	r.rect = rect
	a.requestSwitcherMaterialLocked(r)
	return nil
}

func (a *App) GetSwitcherMaterialStatus(session uint64) platform.MaterialStatus {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	r := a.switcherMaterial
	if r != nil && a.switcherVisible && session == r.session && session == a.visibleSwitcherSession && r.host.alive() && r.host.materialVersion() == r.hostEpoch {
		return r.status
	}
	return platform.MaterialStatus{Session: session, Revision: a.materialRevision, State: "unavailable", Reason: "missingReporter"}
}

func (a *App) GetDockMaterialStatus(session uint64) platform.MaterialStatus {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	r := a.dockMaterial
	if r != nil && r.window == a.dockWindow && a.dockState.Session == session && r.status.Session == session && a.dockState.ContentKind == "windows" {
		if r.status.State == "system" && !r.window.materialInstalled(r.style.Scope) {
			return platform.MaterialStatus{Session: session, Revision: r.status.Revision, State: "unavailable", Reason: "hostClosed"}
		}
		return r.status
	}
	return platform.MaterialStatus{Session: session, Revision: a.materialRevision, State: "unavailable", Reason: "retired"}
}

func (a *App) syncDockMaterialLocked(st dock.State) {
	if a.dockWindow == nil {
		return
	}
	if st.ContentKind != "windows" {
		a.dockWindow.clearMaterial()
		a.dockMaterial = nil
		return
	}
	rect := domain.Bounds{W: st.Bounds.W, H: st.Bounds.H}
	style := appearanceMaterial(platform.MaterialScope{Session: st.Session}, st.Appearance, rect)
	if old := a.dockMaterial; old != nil && old.window == a.dockWindow {
		candidate := old.style
		candidate.Scope.Revision = 0
		if candidate == style {
			return
		}
	}
	style.Scope = a.nextMaterialScopeLocked(st.Session)
	r := &appDockMaterial{window: a.dockWindow, style: style, status: platform.MaterialStatus{Session: st.Session, Revision: style.Scope.Revision, State: "solid"}}
	if style.Enabled {
		r.status.State = "unavailable"
		r.status.Reason = "preparing"
	}
	a.dockMaterial = r
	a.emit("dock:material", r.status)
	r.window.showMaterial(st.Bounds, style, func(status platform.MaterialStatus, current func() bool) {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if !current() || a.dockMaterial != r || a.dockWindow != r.window || a.dockState.Session != status.Session || a.dockState.ContentKind != "windows" || status.Revision != r.style.Scope.Revision {
			return
		}
		if r.status.State == status.State && r.status.Reason == status.Reason {
			return
		}
		status.Revision = a.nextMaterialScopeLocked(status.Session).Revision
		r.status = status
		a.emit("dock:material", status)
	})
}

func (a *App) retireSwitcherMaterialLocked(fade int) {
	if r := a.switcherMaterial; r != nil && r.session != 0 {
		scope := a.nextMaterialScopeLocked(r.session)
		r.owner.retire(scope, fade)
		r.status = platform.MaterialStatus{Session: r.session, Revision: scope.Revision, State: "unavailable", Reason: "retired"}
		r.reported = false
		r.session = 0
	}
}

func (a *App) stopMaterialsLocked() {
	if r := a.switcherMaterial; r != nil {
		r.owner.close()
		a.switcherMaterial = nil
	}
	if a.dockWindow != nil {
		a.dockWindow.clearMaterial()
	}
	a.dockMaterial = nil
}

func (a *App) materialHostResized(host *liveWindow, epoch uint64) {
	a.viewMu.Lock()
	r := a.switcherMaterial
	if r == nil || r.host != host || host.materialVersion() != epoch || r.hostEpoch == epoch || !a.switcherVisible || r.resizeQueued {
		a.viewMu.Unlock()
		return
	}
	r.resizeQueued = true
	r.reported = false
	r.rect = domain.Bounds{}
	a.requestSwitcherMaterialLocked(r)
	a.viewMu.Unlock()
	for {
		attemptedEpoch := host.materialVersion()
		a.controller.Refresh()
		a.viewMu.Lock()
		if a.switcherMaterial != r || !a.switcherVisible || !host.alive() || r.hostEpoch == host.materialVersion() || host.materialVersion() == attemptedEpoch {
			r.resizeQueued = false
			a.viewMu.Unlock()
			return
		}
		a.viewMu.Unlock()
	}
}

func (a *App) materialHostClosed(host *liveWindow) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if r := a.switcherMaterial; r != nil && r.host == host {
		r.owner.close()
		a.switcherMaterial = nil
	}
}

func (d *dockWindow) materialInstalled(scope platform.MaterialScope) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := d.current
	return !d.closed && d.visible && r != nil && r.window.alive() && d.material != nil && d.material.style.Scope == scope && r.materialApplied == scope
}

// One bounded delivery owner keeps the App callback off AppKit. Pending results
// coalesce to the latest native result; an already-delivering result precedes
// the next result and still requires its exact resource guard at App admission.
type dockMaterialDelivery struct {
	mu      sync.Mutex
	wake    chan struct{}
	pending func()
	closed  bool
}

func newDockMaterialDelivery() *dockMaterialDelivery {
	d := &dockMaterialDelivery{wake: make(chan struct{}, 1)}
	go func() {
		for range d.wake {
			d.mu.Lock()
			if d.closed {
				d.mu.Unlock()
				return
			}
			pending := d.pending
			d.pending = nil
			d.mu.Unlock()
			if pending != nil {
				pending()
			}
		}
	}()
	return d
}

func (d *dockMaterialDelivery) post(publish func(platform.MaterialStatus, func() bool), status platform.MaterialStatus, current func() bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.pending = func() { publish(status, current) }
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *dockMaterialDelivery) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	d.pending = nil
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

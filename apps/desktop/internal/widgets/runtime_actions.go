package widgets

import (
	"context"
	"math"
	"strconv"
)

func (r *Runtime) instanceLocked(lease Lease) *runtimeInstance {
	if r.closed {
		return nil
	}
	for _, i := range r.instances {
		if i.state.Lease == lease && eligible(i) {
			return i
		}
	}
	return nil
}

func (r *Runtime) actionLocked(lease Lease, token string) (runtimeAction, bool) {
	var i *runtimeInstance
	if !r.closed {
		for _, candidate := range r.instances {
			identity := candidate.state.Lease
			identity.Revision = lease.Revision
			if identity == lease && eligible(candidate) {
				i = candidate
				break
			}
		}
	}
	if i == nil {
		return runtimeAction{}, false
	}
	a, ok := i.actions[token]
	if !ok || a.owner.retired || a.owner.ctx.Err() != nil || r.owners[a.owner.name] != a.owner || a.owner.sample.Generation != a.generation || a.owner.authorities[a.action] != a.authority || lease.Revision < a.issued || lease.Revision > i.state.Lease.Revision || a.owner.sample.Status != "ready" {
		return runtimeAction{}, false
	}
	return a, true
}

func (r *Runtime) Asset(lease Lease, token string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.instanceLocked(lease)
	if i == nil {
		return nil, ErrRetired
	}
	id, ok := i.assets[token]
	if !ok {
		return nil, ErrRetired
	}
	return i.request.Package.Asset(id)
}

func (r *Runtime) ActionOptions(ctx context.Context, lease Lease, token string) (ActionOptions, error) {
	if ctx == nil {
		return ActionOptions{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return ActionOptions{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.actionLocked(lease, token)
	if !ok {
		return ActionOptions{}, ErrRetired
	}
	out := ActionOptions{Options: []ActionOption{}, Range: copyRange(a.spec.Range)}
	for j, o := range a.spec.Options {
		out.Options = append(out.Options, ActionOption{Token: token + "/option" + strconv.Itoa(j), Label: o.Label})
	}
	return out, nil
}

func (r *Runtime) Perform(ctx context.Context, lease Lease, token, optionToken string, value *float64, finalGuard func() error) error {
	if ctx == nil || finalGuard == nil {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	a, ok := r.actionLocked(lease, token)
	if !ok {
		r.mu.Unlock()
		return ErrRetired
	}
	if r.busy {
		r.mu.Unlock()
		return ErrBusy
	}
	action := ProviderAction{Generation: a.generation, Action: a.action}
	if a.action == "selectOutput" {
		id, ok := a.options[optionToken]
		if !ok || value != nil {
			r.mu.Unlock()
			return ErrInvalid
		}
		action.OptionID = id
	} else if a.action == "seek" {
		if optionToken != "" || value == nil || a.spec.Range == nil || !finite(*value) || *value < a.spec.Range.Min || *value > a.spec.Range.Max {
			r.mu.Unlock()
			return ErrInvalid
		}
		steps := (*value - a.spec.Range.Min) / a.spec.Range.Step
		if math.Abs(steps-math.Round(steps)) > 1e-6 {
			r.mu.Unlock()
			return ErrInvalid
		}
		v := *value
		action.Value = &v
	} else if optionToken != "" || value != nil {
		r.mu.Unlock()
		return ErrInvalid
	}
	source, ok := a.owner.source.(ActionProvider)
	if !ok {
		r.mu.Unlock()
		return ErrUnavailable
	}
	r.busy = true
	r.actions.Add(1)
	a.owner.actionCount++
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.busy = false
		a.owner.actionCount--
		r.mu.Unlock()
		r.actions.Done()
		r.notify()
	}()
	actionCtx, cancel := context.WithCancel(ctx)
	unlink := context.AfterFunc(a.owner.ctx, cancel)
	defer func() { unlink(); cancel() }()
	current := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.mu.Lock()
		_, ok := r.actionLocked(lease, token)
		r.mu.Unlock()
		if !ok {
			return ErrRetired
		}
		return nil
	}
	guard := func() error {
		if err := current(); err != nil {
			return err
		}
		if err := finalGuard(); err != nil {
			return err
		}
		return current()
	}
	if err := guard(); err != nil {
		return err
	}
	if err := source.Perform(actionCtx, action, guard); err != nil {
		return err
	}
	return current()
}

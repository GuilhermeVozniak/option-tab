package widgets

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

type Runtime struct {
	mu                    sync.Mutex
	deps                  Deps
	instances             []*runtimeInstance
	owners                map[string]*providerOwner
	next                  uint64
	running, closed, busy bool
	wake                  chan struct{}
	actions               sync.WaitGroup
	clockTime             time.Time
}
type runtimeInstance struct {
	previousActions map[string]runtimeAction
	request         Request
	manifest        Manifest
	state           InstanceState
	actions         map[string]runtimeAction
	assets          map[string]string
	histories       map[string]runtimeHistory
}
type (
	runtimeHistory struct {
		generation string
		version    string
		values     []float64
	}
	providerOwner struct {
		actionCount   int
		nextAuthority uint64
		authorities   map[string]uint64
		name          string
		source        Provider
		caps          []string
		ctx           context.Context
		cancel        context.CancelFunc
		done          chan struct{}
		retired       bool
		sample        Sample
		serial        uint64
		endedAt       time.Time
	}
)

type runtimeAction struct {
	owner                         *providerOwner
	generation, authority, issued uint64
	action                        string
	spec                          ActionSpec
	options                       map[string]string
}

var providerNames = []string{"battery", "network", "audio", "music", "spotify"}

func NewRuntime(d Deps) *Runtime {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Runtime{deps: d, owners: map[string]*providerOwner{}, wake: make(chan struct{}, 1)}
}

func (r *Runtime) notify() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func requestKey(q Request) string {
	return fmt.Sprintf("%d/%s/%d/%s/%s", q.ControllerEpoch, q.DisplayUUID, q.Session, q.ProfileID, q.InstanceID)
}

var (
	runtimeDisplayID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	runtimeID        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

func allowedRequest(q Request) bool {
	if q.Package == nil || !hash.MatchString(q.Package.digest) || len(q.Package.manifest) == 0 || q.Package.assets == nil || q.ControllerEpoch == 0 || q.Session == 0 || len(q.DisplayUUID) == 0 || len(q.DisplayUUID) > 128 || !runtimeID.MatchString(q.InstanceID) || !runtimeID.MatchString(q.ProfileID) {
		return false
	}
	if !runtimeDisplayID.MatchString(q.DisplayUUID) {
		return false
	}
	return true
}

func copyValue(v Value) Value {
	if v.Text != nil {
		x := *v.Text
		v.Text = &x
	}
	if v.Number != nil {
		x := *v.Number
		v.Number = &x
	}
	if v.Boolean != nil {
		x := *v.Boolean
		v.Boolean = &x
	}
	return v
}

func ValidateSettings(m Manifest, values map[string]Value) (map[string]Value, error) {
	if validate(m) != nil || len(values) > len(m.Settings) {
		return nil, ErrInvalid
	}
	out := map[string]Value{}
	for _, s := range m.Settings {
		v := Value{Text: s.DefaultText, Number: s.DefaultNumber, Boolean: s.DefaultBool}
		if supplied, ok := values[s.ID]; ok {
			v = supplied
		}
		check := s
		check.DefaultText = v.Text
		check.DefaultNumber = v.Number
		check.DefaultBool = v.Boolean
		candidate := m
		candidate.Settings = slices.Clone(m.Settings)
		for j := range candidate.Settings {
			if candidate.Settings[j].ID == s.ID {
				candidate.Settings[j] = check
			}
		}
		if validate(candidate) != nil {
			return nil, ErrInvalid
		}
		out[s.ID] = copyValue(v)
	}
	for k := range values {
		if _, ok := out[k]; !ok {
			return nil, ErrInvalid
		}
	}
	return out, nil
}

func requiredGranted(i *runtimeInstance) bool {
	for _, cap := range i.manifest.RequiredCapabilities {
		if !slices.Contains(i.request.Grants, cap) {
			return false
		}
	}
	return true
}
func eligible(i *runtimeInstance) bool { return i.request.Enabled && requiredGranted(i) }
func (r *Runtime) Configure(requests []Request) error {
	if len(requests) > 32 {
		return ErrInvalid
	}
	normalized := make([]Request, len(requests))
	seen := map[string]bool{}
	for j, q := range requests {
		if !allowedRequest(q) || seen[requestKey(q)] || len(q.Grants) > len(capabilities) {
			return ErrInvalid
		}
		seen[requestKey(q)] = true
		m := q.Package.Manifest()
		caps := append(slices.Clone(m.RequiredCapabilities), m.OptionalCapabilities...)
		grants := map[string]bool{}
		for _, g := range q.Grants {
			if !slices.Contains(caps, g) || grants[g] {
				return ErrInvalid
			}
			grants[g] = true
		}
		settings, err := ValidateSettings(m, q.Settings)
		if err != nil {
			return err
		}
		q.Settings = settings
		q.Grants = slices.Clone(q.Grants)
		slices.Sort(q.Grants)
		normalized[j] = q
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ErrRetired
	}
	old := map[string]*runtimeInstance{}
	for _, i := range r.instances {
		old[requestKey(i.request)] = i
	}
	next := make([]*runtimeInstance, 0, len(normalized))
	for _, q := range normalized {
		i := old[requestKey(q)]
		if i == nil || !reflect.DeepEqual(i.request, q) {
			r.next++
			i = &runtimeInstance{request: q, manifest: q.Package.Manifest(), actions: map[string]runtimeAction{}, assets: map[string]string{}, histories: map[string]runtimeHistory{}}
			status := "preparing"
			if !q.Enabled {
				status = "disabled"
			} else if !requiredGranted(i) {
				status = "grantRequired"
			}
			i.state = InstanceState{Lease: Lease{ControllerEpoch: q.ControllerEpoch, DisplayUUID: q.DisplayUUID, Session: q.Session, ProfileID: q.ProfileID, InstanceID: q.InstanceID, Digest: q.Package.Digest(), AdmissionEpoch: r.next, Revision: 1}, Status: status, Root: RenderNode{Kind: "row", Key: "root", Status: status}}
		}
		next = append(next, i)
	}
	r.instances = next
	wanted := r.wantedLocked()
	var stops []context.CancelFunc
	for name, o := range r.owners {
		if !slices.Equal(o.caps, wanted[name]) && !o.retired {
			o.retired = true
			o.sample = Sample{}
			stops = append(stops, o.cancel)
		}
	}
	r.mu.Unlock()
	for _, stop := range stops {
		stop()
	}
	r.notify()
	return nil
}

func (r *Runtime) wantedLocked() map[string][]string {
	sets := map[string]map[string]bool{}
	for _, i := range r.instances {
		if !eligible(i) {
			continue
		}
		for _, cap := range usedCapabilities(i) {
			name := strings.Split(cap, ".")[0]
			if name == "media" {
				name = strings.Split(cap, ".")[1]
			}
			if name == "clock" {
				continue
			}
			if sets[name] == nil {
				sets[name] = map[string]bool{}
			}
			sets[name][cap] = true
		}
	}
	out := map[string][]string{}
	for name, caps := range sets {
		for cap := range caps {
			out[name] = append(out[name], cap)
		}
		slices.Sort(out[name])
	}
	return out
}

func (r *Runtime) provider(name string) Provider {
	switch name {
	case "battery":
		return r.deps.Providers.Battery
	case "network":
		return r.deps.Providers.Network
	case "audio":
		return r.deps.Providers.Audio
	case "music":
		return r.deps.Providers.Music
	case "spotify":
		return r.deps.Providers.Spotify
	}
	return nil
}

func cloneRender(n RenderNode) RenderNode {
	if n.Progress != nil {
		v := *n.Progress
		n.Progress = &v
	}
	n.History = slices.Clone(n.History)
	n.Children = slices.Clone(n.Children)
	for j := range n.Children {
		n.Children[j] = cloneRender(n.Children[j])
	}
	return n
}

func (r *Runtime) snapshotLocked() []InstanceState {
	out := make([]InstanceState, len(r.instances))
	for j, i := range r.instances {
		out[j] = i.state
		out[j].Root = cloneRender(i.state.Root)
	}
	return out
}

func (r *Runtime) Snapshot() []InstanceState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func (r *Runtime) Run(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalid
	}
	r.mu.Lock()
	if r.running || r.closed {
		r.mu.Unlock()
		return ErrRetired
	}
	r.running = true
	r.clockTime = r.deps.Now()
	r.mu.Unlock()
	maintenance := time.NewTicker(100 * time.Millisecond)
	defer maintenance.Stop()
	var clock *time.Ticker
	defer func() {
		if clock != nil {
			clock.Stop()
		}
	}()
	var published []InstanceState
	publish := func() {
		s := r.Snapshot()
		if !reflect.DeepEqual(s, published) {
			published = s
			if r.deps.Changed != nil {
				r.deps.Changed(r.Snapshot())
			}
		}
	}
	for ctx.Err() == nil {
		r.mu.Lock()
		wanted := r.wantedLocked()
		var starts []*providerOwner
		clockNeeded := false
		for _, i := range r.instances {
			clockNeeded = clockNeeded || (eligible(i) && slices.Contains(usedCapabilities(i), "clock.read"))
		}
		for _, name := range providerNames {
			o := r.owners[name]
			if o != nil {
				select {
				case <-o.done:
					if o.endedAt.IsZero() {
						o.endedAt = r.deps.Now()
					}
					if o.actionCount == 0 && (o.retired || r.deps.Now().Sub(o.endedAt) >= time.Second) {
						delete(r.owners, name)
						o = nil
					}
				default:
				}
			}
			if o == nil && len(wanted[name]) > 0 && r.provider(name) != nil {
				child, cancel := context.WithCancel(ctx)
				r.next++
				o = &providerOwner{name: name, source: r.provider(name), caps: slices.Clone(wanted[name]), ctx: child, cancel: cancel, done: make(chan struct{}), serial: r.next}
				r.owners[name] = o
				starts = append(starts, o)
			}
		}
		if clockNeeded && clock == nil {
			r.clockTime = r.deps.Now()
		}
		for _, i := range r.instances {
			r.resolveLocked(i)
		}
		r.mu.Unlock()
		for _, o := range starts {
			go func(o *providerOwner) {
				defer close(o.done)
				defer o.cancel()
				if o.ctx.Err() != nil {
					return
				}
				_ = o.source.Observe(o.ctx, slices.Clone(o.caps), func(s Sample) { r.accept(o, s) })
				r.mu.Lock()
				if r.owners[o.name] == o {
					o.sample = Sample{Status: "unavailable", Reason: "sourceStopped"}
				}
				r.mu.Unlock()
				r.notify()
			}(o)
		}
		if clockNeeded && clock == nil {
			clock = time.NewTicker(time.Second)
		} else if !clockNeeded && clock != nil {
			clock.Stop()
			clock = nil
		}
		var tick <-chan time.Time
		if clock != nil {
			tick = clock.C
		}
		publish()
		select {
		case <-ctx.Done():
		case <-r.wake:
		case <-maintenance.C:
		case <-tick:
			r.mu.Lock()
			r.clockTime = r.deps.Now()
			r.mu.Unlock()
		}
	}
	r.mu.Lock()
	r.closed = true
	owners := make([]*providerOwner, 0, len(r.owners))
	for _, o := range r.owners {
		o.retired = true
		owners = append(owners, o)
	}
	for _, i := range r.instances {
		i.state.Lease.Revision++
		i.state.Status = "closed"
		i.state.Root = RenderNode{Key: "root", Kind: "row", Status: "closed"}
		i.actions = nil
		i.assets = nil
	}
	r.mu.Unlock()
	for _, o := range owners {
		o.cancel()
	}
	for _, o := range owners {
		<-o.done
	}
	r.actions.Wait()
	publish()
	return ctx.Err()
}

func usedCapabilities(i *runtimeInstance) []string {
	used := map[string]bool{}
	var visit func(Node)
	visit = func(n Node) {
		cap := ""
		if n.Binding != nil {
			cap = fields[n.Binding.Provider+"."+n.Binding.Field].capability
		}
		if n.Command != nil {
			if n.Command.Provider == "audio" {
				cap = "audio.output.select"
			} else {
				cap = "media." + n.Command.Provider + ".control"
			}
		}
		if cap != "" && slices.Contains(i.request.Grants, cap) {
			used[cap] = true
		}
		for _, c := range n.Children {
			visit(c)
		}
	}
	visit(i.manifest.Root)
	out := make([]string, 0, len(used))
	for cap := range used {
		out = append(out, cap)
	}
	slices.Sort(out)
	return out
}

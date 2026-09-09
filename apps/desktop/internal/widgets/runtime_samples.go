package widgets

import (
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func copyRange(n *NumberRange) *NumberRange {
	if n == nil {
		return nil
	}
	v := *n
	return &v
}

func sanitizeSample(name string, caps []string, s Sample, now time.Time) Sample {
	invalid := Sample{Generation: s.Generation, Sequence: s.Sequence, ObservedAt: s.ObservedAt, Status: "unavailable", Reason: "invalidSample"}
	if len(s.Fields) > 16 || len(s.Actions) > 8 || len(s.Reason) > 160 || !slices.Contains([]string{"ready", "unavailable", "unsupported", "permissionRequired", "absent"}, s.Status) || s.ObservedAt.IsZero() || s.ObservedAt.After(now.Add(250*time.Millisecond)) || now.Sub(s.ObservedAt) > 30*time.Second {
		return invalid
	}
	out := s
	out.Fields = map[string]Value{}
	out.Actions = map[string]ActionSpec{}
	for field, v := range s.Fields {
		spec, ok := fields[name+"."+field]
		if !ok {
			return invalid
		}
		if !slices.Contains(caps, spec.capability) {
			continue
		}
		n := 0
		if v.Text != nil {
			n++
		}
		if v.Number != nil {
			n++
		}
		if v.Boolean != nil {
			n++
		}
		if n != 1 {
			return invalid
		}
		switch spec.kind {
		case "text":
			if v.Text == nil || !utf8.ValidString(*v.Text) || len(*v.Text) > 4096 || utf8.RuneCountInString(*v.Text) > 1024 || strings.ContainsRune(*v.Text, 0) {
				return invalid
			}
		case "boolean":
			if v.Boolean == nil {
				return invalid
			}
		case "fraction", "duration", "rate":
			if v.Number == nil || !finite(*v.Number) || *v.Number < 0 || *v.Number > 1e12 || (spec.kind == "fraction" && *v.Number > 1) {
				return invalid
			}
		default:
			return invalid
		}
		out.Fields[field] = copyValue(v)
	}
	capMap := map[string]bool{}
	for _, cap := range caps {
		capMap[cap] = true
	}
	for action, spec := range s.Actions {
		if !validCommand(Command{Provider: name, Action: action}, capMap) {
			continue
		}
		if len(spec.Options) > 64 {
			return invalid
		}
		seen := map[string]bool{}
		for _, o := range spec.Options {
			if !validText(o.ID, 1024) || !validText(o.Label, 160) || seen[o.ID] {
				return invalid
			}
			seen[o.ID] = true
		}
		if spec.Range != nil {
			n := spec.Range
			if !finite(n.Min) || !finite(n.Max) || !finite(n.Step) || n.Min < 0 || n.Max > 1e12 || n.Max < n.Min || n.Step <= 0 || n.Step > n.Max-n.Min {
				return invalid
			}
		}
		if action == "selectOutput" {
			if spec.Range != nil {
				return invalid
			}
		} else if action == "seek" {
			if len(spec.Options) != 0 {
				return invalid
			}
		} else if len(spec.Options) != 0 || spec.Range != nil {
			return invalid
		}
		spec.Options = slices.Clone(spec.Options)
		spec.Range = copyRange(spec.Range)
		out.Actions[action] = spec
	}
	return out
}

func (r *Runtime) accept(o *providerOwner, s Sample) {
	if s.Generation == 0 || s.Sequence == 0 {
		return
	}
	s = sanitizeSample(o.name, o.caps, s, r.deps.Now())
	r.mu.Lock()
	if r.closed || r.owners[o.name] != o || o.retired || o.ctx.Err() != nil || s.Generation < o.sample.Generation || (s.Generation == o.sample.Generation && s.Sequence <= o.sample.Sequence) {
		r.mu.Unlock()
		return
	}
	if o.authorities == nil {
		o.authorities = map[string]uint64{}
	}
	names := map[string]bool{}
	for name := range o.sample.Actions {
		names[name] = true
	}
	for name := range s.Actions {
		names[name] = true
	}
	for name := range names {
		if o.sample.Generation != s.Generation || o.sample.Status != s.Status || !reflect.DeepEqual(o.sample.Actions[name], s.Actions[name]) {
			o.nextAuthority++
			o.authorities[name] = o.nextAuthority
		}
	}
	o.sample = s
	r.mu.Unlock()
	r.notify()
}

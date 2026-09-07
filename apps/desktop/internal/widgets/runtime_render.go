package widgets

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"time"
)

func (r *Runtime) resolveLocked(i *runtimeInstance) {
	status := "ready"
	if !i.request.Enabled {
		status = "disabled"
	} else if !requiredGranted(i) {
		status = "grantRequired"
	}
	root := RenderNode{Key: "root", Kind: "row", Status: status}
	i.previousActions = i.actions
	i.actions = map[string]runtimeAction{}
	i.assets = map[string]string{}
	if status == "ready" {
		root = r.renderLocked(i, i.manifest.Root, "root")
		if root.Status != "ready" {
			status = root.Status
		}
	}
	if i.state.Status != status || !reflect.DeepEqual(i.state.Root, root) {
		i.state.Lease.Revision++
		i.state.Status = status
		i.state.Root = root
	}
	i.previousActions = nil
}

func (r *Runtime) renderLocked(i *runtimeInstance, n Node, key string) RenderNode {
	out := RenderNode{Key: key, Kind: n.Kind, Status: "ready"}
	if n.Kind == "row" || n.Kind == "column" {
		for j, c := range n.Children {
			child := r.renderLocked(i, c, key+"."+strconv.Itoa(j))
			out.Children = append(out.Children, child)
			if child.Status != "ready" {
				out.Status = "partial"
			}
		}
		return out
	}
	token := fmt.Sprintf("w%d/%s", i.state.Lease.AdmissionEpoch, key)
	if n.Kind == "icon" {
		out.AssetToken = token
		i.assets[token] = n.Asset
		return out
	}
	if n.Kind == "button" {
		out.Text = n.Text
		o := r.owners[n.Command.Provider]
		caps := map[string]bool{}
		for _, c := range i.request.Grants {
			caps[c] = true
		}
		if !validCommand(*n.Command, caps) || o == nil || o.retired || o.ctx.Err() != nil || o.sample.Status != "ready" {
			out.Status = "unavailable"
			return out
		}
		spec, ok := o.sample.Actions[n.Command.Action]
		_, canAct := o.source.(ActionProvider)
		if !ok || !spec.Enabled || !canAct || (n.Command.Action == "selectOutput" && len(spec.Options) == 0) || (n.Command.Action == "seek" && spec.Range == nil) {
			out.Status = "unavailable"
			return out
		}
		out.ActionToken = fmt.Sprintf("%s/o%d/g%d/a%d", token, o.serial, o.sample.Generation, o.authorities[n.Command.Action])
		action := runtimeAction{owner: o, generation: o.sample.Generation, authority: o.authorities[n.Command.Action], issued: i.state.Lease.Revision + 1, action: n.Command.Action, spec: spec, options: map[string]string{}}
		if previous, ok := i.previousActions[out.ActionToken]; ok {
			action.issued = previous.issued
		}
		for j, opt := range spec.Options {
			action.options[out.ActionToken+"/option"+strconv.Itoa(j)] = opt.ID
		}
		i.actions[out.ActionToken] = action
		return out
	}
	if n.Binding == nil {
		out.Text = n.Text
		return out
	}
	b := *n.Binding
	spec := fields[b.Provider+"."+b.Field]
	if !slices.Contains(i.request.Grants, spec.capability) {
		out.Status = "unavailable"
		return out
	}
	formatter := b.Formatter
	if b.FormatterSetting != "" {
		formatter = *i.request.Settings[b.FormatterSetting].Text
	}
	if b.Provider == "clock" {
		clock := r.clockTime
		if b.TimezoneSetting != "" {
			zone, _ := time.LoadLocation(*i.request.Settings[b.TimezoneSetting].Text)
			clock = clock.In(zone)
		}
		switch formatter {
		case "shortTime":
			out.Text = clock.Format("15:04")
		case "longTime":
			out.Text = clock.Format("15:04:05")
		case "date":
			out.Text = clock.Format("2006-01-02")
		}
		return out
	}
	o := r.owners[b.Provider]
	if o == nil || o.retired || o.ctx.Err() != nil || o.sample.Status != "ready" {
		out.Status = "unavailable"
		return out
	}
	v, ok := o.sample.Fields[b.Field]
	if !ok {
		out.Status = "unavailable"
		return out
	}
	if n.Kind == "progress" {
		if spec.kind != "fraction" {
			out.Status = "unavailable"
			return out
		}
		number := *v.Number
		out.Progress = &number
		return out
	}
	if n.Kind == "sparkline" {
		version := fmt.Sprintf("%d/%d/%d", o.serial, o.sample.Generation, o.sample.Sequence)
		history := i.histories[key]
		generation := fmt.Sprintf("%d/%d", o.serial, o.sample.Generation)
		if history.generation != generation {
			history = runtimeHistory{generation: generation}
		}
		if history.version != version {
			history.values = append(history.values, *v.Number)
			if len(history.values) > n.History {
				history.values = slices.Clone(history.values[len(history.values)-n.History:])
			}
			history.version = version
			i.histories[key] = history
		}
		out.History = slices.Clone(history.values)
		return out
	}
	switch formatter {
	case "text":
		out.Text = *v.Text
	case "boolean":
		out.Text = strconv.FormatBool(*v.Boolean)
	case "percent":
		out.Text = fmt.Sprintf("%.0f%%", *v.Number*100)
	case "number":
		out.Text = strconv.FormatFloat(*v.Number, 'f', -1, 64)
	case "bytesPerSecond":
		out.Text = fmt.Sprintf("%.0f B/s", *v.Number)
	case "duration":
		seconds := int64(*v.Number / 1000)
		out.Text = fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
	}
	return out
}

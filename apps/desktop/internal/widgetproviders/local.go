// Package widgetproviders connects fixed native ports to the declarative widget
// runtime. It never resolves package-authored provider names or native methods.
package widgetproviders

import (
	"context"
	"math"
	"slices"
	"strings"

	"option-tab/internal/platform"
	"option-tab/internal/widgets"
)

type (
	Battery struct{ Source platform.BatterySource }
	Network struct{ Source platform.NetworkSource }
	Audio   struct{ Source platform.AudioOutputSource }
)

func validObservation(ctx context.Context, caps, allowed []string, emit func(widgets.Sample)) error {
	if ctx == nil || emit == nil || len(caps) == 0 || len(caps) > len(allowed) {
		return widgets.ErrInvalid
	}
	seen := map[string]bool{}
	for _, cap := range caps {
		if seen[cap] || !slices.Contains(allowed, cap) {
			return widgets.ErrInvalid
		}
		seen[cap] = true
	}
	return ctx.Err()
}

func textValue(value string) widgets.Value { return widgets.Value{Text: &value} }

func putNumber(fields map[string]widgets.Value, key string, value *float64, max float64) {
	if value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0 && *value <= max {
		copy := *value
		fields[key] = widgets.Value{Number: &copy}
	}
}

func putBool(fields map[string]widgets.Value, key string, value *bool) {
	if value != nil {
		copy := *value
		fields[key] = widgets.Value{Boolean: &copy}
	}
}

func (p Battery) Observe(ctx context.Context, caps []string, emit func(widgets.Sample)) error {
	if err := validObservation(ctx, caps, []string{"battery.read"}, emit); err != nil {
		return err
	}
	if p.Source == nil {
		return widgets.ErrUnavailable
	}
	return p.Source.ObserveBattery(ctx, func(value platform.BatterySnapshot) {
		s := widgets.Sample{Generation: value.Generation, Sequence: value.Sequence, ObservedAt: value.ObservedAt, Status: value.Status, Reason: value.Reason, Fields: map[string]widgets.Value{}}
		if value.Status == "ready" {
			putNumber(s.Fields, "charge", value.Charge, 1)
			putBool(s.Fields, "charging", value.Charging)
			if slices.Contains([]string{"battery", "external", "unknown"}, value.PowerSource) {
				s.Fields["powerSource"] = textValue(value.PowerSource)
			}
		}
		if ctx.Err() == nil {
			emit(s)
		}
	})
}

func (p Network) Observe(ctx context.Context, caps []string, emit func(widgets.Sample)) error {
	if err := validObservation(ctx, caps, []string{"network.status.read", "network.usage.read"}, emit); err != nil {
		return err
	}
	if p.Source == nil {
		return widgets.ErrUnavailable
	}
	status, usage := slices.Contains(caps, "network.status.read"), slices.Contains(caps, "network.usage.read")
	return p.Source.ObserveNetwork(ctx, usage, func(value platform.NetworkSnapshot) {
		s := widgets.Sample{Generation: value.Generation, Sequence: value.Sequence, ObservedAt: value.ObservedAt, Status: value.Status, Reason: value.Reason, Fields: map[string]widgets.Value{}}
		if value.Status == "ready" {
			if status {
				putBool(s.Fields, "connected", value.Connected)
				if slices.Contains([]string{"none", "wifi", "ethernet", "vpn", "other", "unknown"}, value.Category) {
					s.Fields["category"] = textValue(value.Category)
				}
			}
			if usage {
				putNumber(s.Fields, "uploadRate", value.UploadRate, 1e12)
				putNumber(s.Fields, "downloadRate", value.DownloadRate, 1e12)
			}
		}
		if ctx.Err() == nil {
			emit(s)
		}
	})
}

func (p Audio) Observe(ctx context.Context, caps []string, emit func(widgets.Sample)) error {
	if err := validObservation(ctx, caps, []string{"audio.status.read", "audio.output.select"}, emit); err != nil {
		return err
	}
	if p.Source == nil {
		return widgets.ErrUnavailable
	}
	status, control := slices.Contains(caps, "audio.status.read"), slices.Contains(caps, "audio.output.select")
	return p.Source.ObserveAudioOutputs(ctx, func(value platform.AudioOutputSnapshot) {
		s := widgets.Sample{Generation: value.Generation, Sequence: value.Sequence, ObservedAt: value.ObservedAt, Status: value.Status, Reason: value.Reason, Fields: map[string]widgets.Value{}, Actions: map[string]widgets.ActionSpec{}}
		if value.Status == "ready" {
			options := []widgets.ProviderOption{}
			seen := map[string]bool{}
			for _, device := range value.Devices[:min(len(value.Devices), 64)] {
				if !device.Alive || device.OutputChannels <= 0 || device.UID == "" || seen[device.UID] {
					continue
				}
				seen[device.UID] = true
				if status && device.UID == value.DefaultUID {
					s.Fields["outputName"] = textValue(device.Name)
				}
				if control {
					label := []rune(strings.ReplaceAll(strings.ToValidUTF8(device.Name, "�"), "\x00", ""))
					if len(label) == 0 {
						label = []rune("Audio output")
					}
					options = append(options, widgets.ProviderOption{ID: device.UID, Label: string(label[:min(len(label), 160)])})
				}
			}
			if status {
				putNumber(s.Fields, "volume", value.Volume, 1)
				putBool(s.Fields, "muted", value.Muted)
			}
			if control {
				s.Actions["selectOutput"] = widgets.ActionSpec{Enabled: len(options) > 0, Options: options}
			}
		}
		if ctx.Err() == nil {
			emit(s)
		}
	})
}

// Perform is reachable only through the runtime's fixed action catalog and
// digest/instance grant checks. The source revalidates generation and UID again
// after native preparation and invokes the guard immediately before dispatch.
func (p Audio) Perform(ctx context.Context, action widgets.ProviderAction, guard func() error) error {
	if ctx == nil || guard == nil || action.Generation == 0 || action.Action != "selectOutput" || action.OptionID == "" || action.Value != nil {
		return widgets.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.Source == nil {
		return widgets.ErrUnavailable
	}
	if err := guard(); err != nil {
		return err
	}
	return p.Source.SelectAudioOutput(ctx, action.Generation, action.OptionID, guard)
}

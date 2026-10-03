package config

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"option-tab/internal/widgets"
)

var (
	widgetPackageID    = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	widgetDigest       = regexp.MustCompile(`^[a-f0-9]{64}$`)
	widgetSettingID    = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)
	widgetCapabilities = []string{"clock.read", "battery.read", "network.status.read", "network.usage.read", "audio.status.read", "audio.output.select", "media.music.read", "media.spotify.read", "media.music.control", "media.spotify.control"}
)

func cloneWidgetSettings(settings map[string]widgets.Value) map[string]widgets.Value {
	if settings == nil {
		return nil
	}
	out := make(map[string]widgets.Value, len(settings))
	for key, value := range settings {
		if value.Text != nil {
			v := *value.Text
			value.Text = &v
		}
		if value.Number != nil {
			v := *value.Number
			value.Number = &v
		}
		if value.Boolean != nil {
			v := *value.Boolean
			value.Boolean = &v
		}
		out[key] = value
	}
	return out
}

func validWidgetInstance(w WidgetInstance) bool {
	if len(w.PackageID) > 160 || !widgetPackageID.MatchString(w.PackageID) || len(w.Grants) > len(widgetCapabilities) || len(w.Settings) > 16 {
		return false
	}
	legacy := w.PackageID == BuiltinClockPackage && w.Digest == BuiltinClockDigest
	if !legacy && !widgetDigest.MatchString(w.Digest) {
		return false
	}
	seen := map[string]bool{}
	for _, cap := range w.Grants {
		if seen[cap] || !slices.Contains(widgetCapabilities, cap) || (legacy && cap != "clock.read") {
			return false
		}
		seen[cap] = true
	}
	if legacy {
		return len(w.Settings) == 0
	}
	for key, value := range w.Settings {
		if !widgetSettingID.MatchString(key) {
			return false
		}
		count := 0
		if value.Text != nil {
			count++
			if !utf8.ValidString(*value.Text) || len(*value.Text) > 1024 || strings.ContainsRune(*value.Text, 0) {
				return false
			}
		}
		if value.Number != nil {
			count++
			if math.IsNaN(*value.Number) || math.IsInf(*value.Number, 0) || math.Abs(*value.Number) > 1e12 {
				return false
			}
		}
		if value.Boolean != nil {
			count++
		}
		if count != 1 {
			return false
		}
	}
	if p, ok := widgets.Builtin(w.PackageID); ok && p.Digest() == w.Digest {
		m := p.Manifest()
		for _, cap := range w.Grants {
			if !slices.Contains(m.RequiredCapabilities, cap) && !slices.Contains(m.OptionalCapabilities, cap) {
				return false
			}
		}
		if _, err := widgets.ValidateSettings(m, w.Settings); err != nil {
			return false
		}
	}
	return true
}

func validWidgetStacks(stacks []WidgetStack, instances map[string]bool) bool {
	if len(stacks) > 4 {
		return false
	}
	ids, members := map[string]bool{}, map[string]bool{}
	for _, stack := range stacks {
		if !launcherID.MatchString(stack.ID) || ids[stack.ID] || !utf8.ValidString(stack.Name) || strings.TrimSpace(stack.Name) == "" || utf8.RuneCountInString(stack.Name) > 80 || strings.ContainsRune(stack.Name, 0) || len(stack.Members) < 2 || len(stack.Members) > 4 || !slices.Contains(stack.Members, stack.ActiveID) {
			return false
		}
		ids[stack.ID] = true
		for _, id := range stack.Members {
			if !instances[id] || members[id] {
				return false
			}
			members[id] = true
		}
	}
	return len(instances)-len(members)+len(stacks) <= 4
}

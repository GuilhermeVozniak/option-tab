package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"slices"
	"unicode/utf8"
)

const (
	BuiltinClockPackage = "org.optiontab.clock"
	// BuiltinClockManifest is immutable declarative data; no executable package is loaded.
	BuiltinClockManifest = `{"packageID":"org.optiontab.clock","version":1,"capabilities":["clock.read"],"root":{"kind":"row","children":[{"kind":"text","binding":"localTime","format":"15:04"}]}}`
	BuiltinClockDigest   = "sha256:09bcb4221f7ace94e21898b4e583f9db72be1be5f6524fbe9972af70726c9dc3"
)

type ReplacementDockSettings struct {
	Version  int               `json:"version"`
	Enabled  bool              `json:"enabled"`
	Profiles []LauncherProfile `json:"profiles"`
	Bindings []LauncherBinding `json:"bindings"`
}
type LauncherProfile struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Edge              string           `json:"edge"`
	Layout            string           `json:"layout"`
	IconPx            int              `json:"iconPx"`
	ThicknessPx       int              `json:"thicknessPx"`
	MaxLengthFraction float64          `json:"maxLengthFraction"`
	InsetPx           int              `json:"insetPx"`
	AutoHide          bool             `json:"autoHide"`
	Widgets           []WidgetInstance `json:"widgets"`
}
type LauncherBinding struct {
	ID          string `json:"id"`
	Target      string `json:"target"`
	DisplayUUID string `json:"displayUUID"`
	ProfileID   string `json:"profileID"`
}
type WidgetInstance struct {
	ID        string   `json:"id"`
	PackageID string   `json:"packageID"`
	Digest    string   `json:"digest"`
	Enabled   bool     `json:"enabled"`
	Grants    []string `json:"grants"`
}

func DefaultReplacementDock() ReplacementDockSettings {
	return ReplacementDockSettings{Version: 1, Profiles: []LauncherProfile{{ID: "default", Name: "Default", Edge: "bottom", Layout: "floating", IconPx: 40, ThicknessPx: 64, MaxLengthFraction: .8, InsetPx: 16, Widgets: []WidgetInstance{{ID: "clock", PackageID: BuiltinClockPackage, Digest: BuiltinClockDigest, Grants: []string{}}}}}, Bindings: []LauncherBinding{{ID: "main", Target: "main", ProfileID: "default"}}}
}

func CloneReplacementDock(s ReplacementDockSettings) ReplacementDockSettings {
	s.Profiles = slices.Clone(s.Profiles)
	s.Bindings = slices.Clone(s.Bindings)
	for i := range s.Profiles {
		s.Profiles[i].Widgets = slices.Clone(s.Profiles[i].Widgets)
		for j := range s.Profiles[i].Widgets {
			s.Profiles[i].Widgets[j].Grants = slices.Clone(s.Profiles[i].Widgets[j].Grants)
		}
	}
	return s
}

var launcherID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func ValidateReplacementDock(s ReplacementDockSettings) error {
	bad := errors.New("config: invalid replacement Dock settings")
	if s.Version != 1 || len(s.Profiles) == 0 || len(s.Profiles) > 8 || len(s.Bindings) > 8 {
		return bad
	}
	profiles := map[string]bool{}
	for _, p := range s.Profiles {
		if !launcherID.MatchString(p.ID) || profiles[p.ID] || !utf8.ValidString(p.Name) || utf8.RuneCountInString(p.Name) < 1 || utf8.RuneCountInString(p.Name) > 80 || p.Edge != "bottom" || p.Layout != "floating" || p.IconPx < 24 || p.IconPx > 64 || p.ThicknessPx < 48 || p.ThicknessPx > 112 || p.ThicknessPx < p.IconPx+8 || math.IsNaN(p.MaxLengthFraction) || math.IsInf(p.MaxLengthFraction, 0) || p.MaxLengthFraction < .25 || p.MaxLengthFraction > .9 || p.InsetPx < 12 || p.InsetPx > 64 || len(p.Widgets) > 4 {
			return bad
		}
		profiles[p.ID] = true
		widgets := map[string]bool{}
		for _, w := range p.Widgets {
			if !launcherID.MatchString(w.ID) || widgets[w.ID] || w.PackageID != BuiltinClockPackage || w.Digest != BuiltinClockDigest || len(w.Grants) > 1 {
				return bad
			}
			widgets[w.ID] = true
			for _, g := range w.Grants {
				if g != "clock.read" {
					return bad
				}
			}
		}
	}
	ids := map[string]bool{}
	targets := map[string]bool{}
	for _, b := range s.Bindings {
		if !launcherID.MatchString(b.ID) || ids[b.ID] || !profiles[b.ProfileID] || (b.Target != "main" && b.Target != "display") || (b.Target == "main" && b.DisplayUUID != "") || (b.Target == "display" && !validDisplayUUID(b.DisplayUUID)) {
			return bad
		}
		key := b.Target + ":" + b.DisplayUUID
		if targets[key] {
			return bad
		}
		targets[key] = true
		ids[b.ID] = true
	}
	return nil
}

// Strict bounded nested decoder; old top-level settings retain their migration rules.
func decodeReplacement(raw json.RawMessage) (ReplacementDockSettings, error) {
	bad := errors.New("config: invalid replacement Dock document")
	if len(raw) > 64*1024 {
		return ReplacementDockSettings{}, bad
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	nodes := 0
	var walk func(int) error
	walk = func(depth int) error {
		nodes++
		if depth > 8 || nodes > 2048 {
			return bad
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		v, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch v {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return bad
				}
				seen[name] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			count := 0
			for d.More() {
				count++
				if count > 8 {
					return bad
				}
				if e := walk(depth + 1); e != nil {
					return e
				}
			}
		default:
			return bad
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return ReplacementDockSettings{}, bad
	}
	if _, err := d.Token(); err != io.EOF {
		return ReplacementDockSettings{}, bad
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var s ReplacementDockSettings
	if err := d.Decode(&s); err != nil {
		return s, bad
	}
	return s, ValidateReplacementDock(s)
}

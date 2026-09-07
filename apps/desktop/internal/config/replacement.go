package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	BuiltinClockPackage = "org.optiontab.clock"
	// BuiltinClockManifest is immutable declarative data; no executable package is loaded.
	BuiltinClockManifest = `{"packageID":"org.optiontab.clock","version":1,"capabilities":["clock.read"],"root":{"kind":"row","children":[{"kind":"text","binding":"localTime","format":"15:04"}]}}`
	BuiltinClockDigest   = "sha256:09bcb4221f7ace94e21898b4e583f9db72be1be5f6524fbe9972af70726c9dc3"
)

type LauncherProfileRule struct {
	ID        string `json:"id"`
	Enabled   bool   `json:"enabled"`
	BundleID  string `json:"bundleID"`
	ProfileID string `json:"profileID"`
	BindingID string `json:"bindingID,omitempty"`
}
type ReplacementDockSettings struct {
	Rules    []LauncherProfileRule `json:"rules,omitempty"`
	Version  int                   `json:"version"`
	Enabled  bool                  `json:"enabled"`
	Profiles []LauncherProfile     `json:"profiles"`
	Bindings []LauncherBinding     `json:"bindings"`
}
type LauncherAppearance struct {
	Theme          string  `json:"theme"`
	Material       string  `json:"material"`
	Tint           string  `json:"tint"`
	Opacity        float64 `json:"opacity"`
	BorderOpacity  float64 `json:"borderOpacity"`
	CornerRadiusPx int     `json:"cornerRadiusPx"`
	ItemSpacingPx  int     `json:"itemSpacingPx"`
	ShowLabels     bool    `json:"showLabels"`
}

func DefaultLauncherAppearance() LauncherAppearance {
	return LauncherAppearance{Theme: "system", Material: "solid", Tint: "#172033", Opacity: .76, BorderOpacity: .16, CornerRadiusPx: 18, ItemSpacingPx: 6, ShowLabels: true}
}

type LauncherProfile struct {
	Alignment         string             `json:"alignment"`
	Appearance        LauncherAppearance `json:"appearance"`
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Edge              string             `json:"edge"`
	Layout            string             `json:"layout"`
	IconPx            int                `json:"iconPx"`
	ThicknessPx       int                `json:"thicknessPx"`
	MaxLengthFraction float64            `json:"maxLengthFraction"`
	InsetPx           int                `json:"insetPx"`
	AutoHide          bool               `json:"autoHide"`
	Widgets           []WidgetInstance   `json:"widgets"`
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
	return ReplacementDockSettings{Version: 2, Profiles: []LauncherProfile{{ID: "default", Name: "Default", Alignment: "center", Appearance: DefaultLauncherAppearance(), Edge: "bottom", Layout: "floating", IconPx: 40, ThicknessPx: 64, MaxLengthFraction: .8, InsetPx: 16, Widgets: []WidgetInstance{{ID: "clock", PackageID: BuiltinClockPackage, Digest: BuiltinClockDigest, Grants: []string{}}}}}, Bindings: []LauncherBinding{{ID: "main", Target: "main", ProfileID: "default"}}}
}

func CloneReplacementDock(s ReplacementDockSettings) ReplacementDockSettings {
	s.Profiles = slices.Clone(s.Profiles)
	s.Bindings = slices.Clone(s.Bindings)
	s.Rules = slices.Clone(s.Rules)
	for i := range s.Profiles {
		s.Profiles[i].Widgets = slices.Clone(s.Profiles[i].Widgets)
		for j := range s.Profiles[i].Widgets {
			s.Profiles[i].Widgets[j].Grants = slices.Clone(s.Profiles[i].Widgets[j].Grants)
		}
	}
	return s
}

var launcherTint = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validLauncherAppearance(a LauncherAppearance) bool {
	return slices.Contains([]string{"system", "light", "dark"}, a.Theme) && slices.Contains([]string{"solid", "system"}, a.Material) && launcherTint.MatchString(a.Tint) && !math.IsNaN(a.Opacity) && !math.IsInf(a.Opacity, 0) && a.Opacity >= .35 && a.Opacity <= 1 && !math.IsNaN(a.BorderOpacity) && !math.IsInf(a.BorderOpacity, 0) && a.BorderOpacity >= 0 && a.BorderOpacity <= .5 && a.CornerRadiusPx >= 0 && a.CornerRadiusPx <= 28 && a.ItemSpacingPx >= 2 && a.ItemSpacingPx <= 20
}

var launcherID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func ValidateReplacementDock(s ReplacementDockSettings) error {
	bad := errors.New("config: invalid replacement Dock settings")
	if s.Version != 2 || len(s.Profiles) == 0 || len(s.Profiles) > 8 || len(s.Bindings) > 8 || len(s.Rules) > 8 {
		return bad
	}
	profiles := map[string]bool{}
	for _, p := range s.Profiles {
		if !launcherID.MatchString(p.ID) || profiles[p.ID] || !utf8.ValidString(p.Name) || utf8.RuneCountInString(p.Name) < 1 || utf8.RuneCountInString(p.Name) > 80 || !slices.Contains([]string{"bottom", "top", "left", "right"}, p.Edge) || !slices.Contains([]string{"floating", "fullWidth"}, p.Layout) || !slices.Contains([]string{"start", "center", "end"}, p.Alignment) || !validLauncherAppearance(p.Appearance) || p.IconPx < 24 || p.IconPx > 64 || p.ThicknessPx < 48 || p.ThicknessPx > 112 || p.ThicknessPx < p.IconPx+8 || math.IsNaN(p.MaxLengthFraction) || math.IsInf(p.MaxLengthFraction, 0) || p.MaxLengthFraction < .25 || p.MaxLengthFraction > .9 || p.InsetPx < 12 || p.InsetPx > 64 || len(p.Widgets) > 4 {
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
	ruleIDs := map[string]bool{}
	for _, r := range s.Rules {
		if !launcherID.MatchString(r.ID) || ruleIDs[r.ID] || !ValidLauncherBundleID(r.BundleID) || !profiles[r.ProfileID] || (r.BindingID != "" && !ids[r.BindingID]) {
			return bad
		}
		ruleIDs[r.ID] = true
	}
	return nil
}

// ValidLauncherBundleID bounds exact identity text; it does not establish that
// an application is installed or authorize a process action.
var launcherBundleID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,255}$`)

func ValidLauncherBundleID(id string) bool { return len(id) <= 255 && launcherBundleID.MatchString(id) }

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
	if s.Version == 1 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return s, bad
		}
		for key := range fields {
			if strings.EqualFold(key, "rules") {
				return s, bad
			}
		}

		var legacy struct {
			Profiles []map[string]json.RawMessage `json:"profiles"`
		}
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return s, bad
		}
		for i, p := range s.Profiles {
			if p.Edge != "bottom" || p.Layout != "floating" {
				return s, bad
			}
			if _, exists := legacy.Profiles[i]["alignment"]; exists {
				return s, bad
			}
			if _, exists := legacy.Profiles[i]["appearance"]; exists {
				return s, bad
			}
			s.Profiles[i].Alignment = "center"
			s.Profiles[i].Appearance = DefaultLauncherAppearance()
		}
		s.Version = 2
	}
	return s, ValidateReplacementDock(s)
}

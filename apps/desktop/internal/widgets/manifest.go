// Package widgets validates inert declarative packages. It executes no provider,
// command, permission request, network operation or package-authored code.
package widgets

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("widgets: invalid package")

const MaxManifest = 256 * 1024

type (
	Localized map[string]string
	Manifest  struct {
		SchemaVersion        int       `json:"schemaVersion"`
		ID                   string    `json:"id"`
		Version              string    `json:"version"`
		MinimumAppVersion    string    `json:"minimumAppVersion"`
		Name                 Localized `json:"name"`
		Description          Localized `json:"description"`
		RequiredCapabilities []string  `json:"requiredCapabilities,omitempty"`
		OptionalCapabilities []string  `json:"optionalCapabilities,omitempty"`
		Settings             []Setting `json:"settings,omitempty"`
		Root                 Node      `json:"root"`
		Assets               []Asset   `json:"assets,omitempty"`
	}
)

type Setting struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Name          Localized `json:"name"`
	DefaultText   *string   `json:"defaultText,omitempty"`
	DefaultNumber *float64  `json:"defaultNumber,omitempty"`
	DefaultBool   *bool     `json:"defaultBool,omitempty"`
	Options       []string  `json:"options,omitempty"`
	Min           *float64  `json:"min,omitempty"`
	Max           *float64  `json:"max,omitempty"`
}
type Node struct {
	Kind     string   `json:"kind"`
	Text     string   `json:"text,omitempty"`
	Binding  *Binding `json:"binding,omitempty"`
	Asset    string   `json:"asset,omitempty"`
	History  int      `json:"history,omitempty"`
	Command  *Command `json:"command,omitempty"`
	Children []Node   `json:"children,omitempty"`
}
type Binding struct {
	Provider        string `json:"provider"`
	Field           string `json:"field"`
	Formatter       string `json:"formatter"`
	TimezoneSetting string `json:"timezoneSetting,omitempty"`
}
type (
	Command struct {
		Provider string `json:"provider"`
		Action   string `json:"action"`
	}
	Asset struct {
		ID     string `json:"id"`
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
)

var (
	identifier   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)
	packageID    = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	version      = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
	hash         = regexp.MustCompile(`^[a-f0-9]{64}$`)
	component    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*(\.[A-Za-z0-9_-]+)*$`)
	capabilities = []string{"clock.read", "battery.read", "network.status.read", "network.usage.read", "audio.status.read", "audio.output.select", "media.music.read", "media.spotify.read", "media.music.control", "media.spotify.control"}
)

func strictJSON(raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxManifest || !utf8.Valid(raw) {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	nodes := 0
	var walk func(int) error
	walk = func(depth int) error {
		nodes++
		if depth > 32 || nodes > 4096 {
			return ErrInvalid
		}
		v, err := d.Token()
		if err != nil {
			return ErrInvalid
		}
		delim, ok := v.(json.Delim)
		if !ok {
			if v == nil {
				return ErrInvalid
			}
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return ErrInvalid
				}
				k, ok := key.(string)
				fold := strings.ToLower(k)
				if !ok || seen[fold] {
					return ErrInvalid
				}
				seen[fold] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			n := 0
			for d.More() {
				n++
				if n > 128 {
					return ErrInvalid
				}
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return ErrInvalid
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInvalid
	}
	return nil
}

func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if err := strictJSON(raw); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return Manifest{}, ErrInvalid
	}
	if err := validate(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func validText(s string, max int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) > 0 && utf8.RuneCountInString(s) <= max && !strings.ContainsRune(s, 0)
}

func localized(l Localized, max int) bool {
	if len(l) < 1 || len(l) > 3 || !validText(l["en"], max) {
		return false
	}
	for k, v := range l {
		if !slices.Contains([]string{"en", "pt-BR", "es"}, k) || !validText(v, max) {
			return false
		}
	}
	return true
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func validVersion(v string) bool {
	if len(v) > 80 || !version.MatchString(v) {
		return false
	}
	pre := strings.SplitN(v, "+", 2)[0]
	parts := strings.SplitN(pre, "-", 2)
	if len(parts) == 2 {
		for _, id := range strings.Split(parts[1], ".") {
			numeric := true
			for _, c := range id {
				numeric = numeric && c >= '0' && c <= '9'
			}
			if numeric && len(id) > 1 && id[0] == '0' {
				return false
			}
		}
	}
	return true
}

func safeAssetPath(p string) bool {
	if len(p) > 160 || !strings.HasPrefix(p, "assets/") || !strings.HasSuffix(p, ".png") {
		return false
	}
	parts := strings.Split(p, "/")
	if len(parts) > 5 {
		return false
	}
	for _, c := range parts {
		if !component.MatchString(c) || reservedComponent(c) {
			return false
		}
	}
	return true
}

func validate(m Manifest) error {
	if m.SchemaVersion != 1 || len(m.ID) > 160 || !packageID.MatchString(m.ID) || !validVersion(m.Version) || !validVersion(m.MinimumAppVersion) || !localized(m.Name, 80) || !localized(m.Description, 1024) || len(m.Settings) > 16 || len(m.Assets) > 63 {
		return ErrInvalid
	}
	caps := map[string]bool{}
	for _, list := range [][]string{m.RequiredCapabilities, m.OptionalCapabilities} {
		if len(list) > len(capabilities) {
			return ErrInvalid
		}
		for _, c := range list {
			if !slices.Contains(capabilities, c) || caps[c] {
				return ErrInvalid
			}
			caps[c] = true
		}
	}
	settings := map[string]string{}
	for _, s := range m.Settings {
		if !identifier.MatchString(s.ID) || settings[s.ID] != "" || !localized(s.Name, 80) {
			return ErrInvalid
		}
		settings[s.ID] = s.Type
		switch s.Type {
		case "boolean":
			if s.DefaultBool == nil || s.DefaultText != nil || s.DefaultNumber != nil || s.Min != nil || s.Max != nil || len(s.Options) != 0 {
				return ErrInvalid
			}
		case "number":
			if s.DefaultNumber == nil || s.DefaultBool != nil || s.DefaultText != nil || s.Min == nil || s.Max == nil || len(s.Options) != 0 || !finite(*s.Min) || !finite(*s.Max) || !finite(*s.DefaultNumber) || *s.Min > *s.Max || *s.Min < -1e9 || *s.Max > 1e9 || *s.DefaultNumber < *s.Min || *s.DefaultNumber > *s.Max {
				return ErrInvalid
			}
		case "choice", "timezone":
			if s.DefaultText == nil || !validText(*s.DefaultText, 80) || s.DefaultBool != nil || s.DefaultNumber != nil || s.Min != nil || s.Max != nil {
				return ErrInvalid
			}
			if s.Type == "timezone" {
				if len(s.Options) != 0 {
					return ErrInvalid
				}
				if _, err := time.LoadLocation(*s.DefaultText); err != nil {
					return ErrInvalid
				}
			} else {
				if len(s.Options) < 1 || len(s.Options) > 16 || !slices.Contains(s.Options, *s.DefaultText) {
					return ErrInvalid
				}
				seen := map[string]bool{}
				for _, v := range s.Options {
					if !validText(v, 80) || seen[v] {
						return ErrInvalid
					}
					seen[v] = true
				}
			}
		default:
			return ErrInvalid
		}
	}
	assets := map[string]bool{}
	paths := map[string]bool{}
	for _, a := range m.Assets {
		key := strings.ToLower(a.Path)
		if !identifier.MatchString(a.ID) || assets[a.ID] || !safeAssetPath(a.Path) || paths[key] || !hash.MatchString(a.SHA256) {
			return ErrInvalid
		}
		assets[a.ID] = true
		paths[key] = true
	}
	count := 0
	var node func(Node, int) bool
	node = func(n Node, depth int) bool {
		count++
		if count > 128 || depth > 8 || len(n.Children) > 16 || len(n.Text) > 2048 {
			return false
		}
		if n.Kind == "row" || n.Kind == "column" {
			if n.Text != "" || n.Binding != nil || n.Asset != "" || n.History != 0 || n.Command != nil {
				return false
			}
			for _, c := range n.Children {
				if !node(c, depth+1) {
					return false
				}
			}
			return true
		}
		if len(n.Children) != 0 {
			return false
		}
		if n.Kind == "button" {
			return validText(n.Text, 160) && n.Binding == nil && n.Asset == "" && n.History == 0 && n.Command != nil && validCommand(*n.Command, caps)
		}
		if n.Command != nil {
			return false
		}
		if n.Kind == "icon" {
			return assets[n.Asset] && n.Text == "" && n.Binding == nil && n.History == 0
		}
		if n.Asset != "" {
			return false
		}
		if n.Kind == "text" {
			if n.History != 0 {
				return false
			}
			if n.Binding == nil {
				return validText(n.Text, 1024)
			}
			return n.Text == "" && validBinding(*n.Binding, caps, settings, false)
		}
		if n.Kind == "progress" || n.Kind == "sparkline" {
			return n.Text == "" && n.Binding != nil && validBinding(*n.Binding, caps, settings, true) && ((n.Kind == "progress" && n.History == 0) || (n.Kind == "sparkline" && n.History >= 1 && n.History <= 120))
		}
		return false
	}
	if !node(m.Root, 1) {
		return ErrInvalid
	}
	return nil
}

func reservedComponent(c string) bool {
	stem := strings.ToLower(strings.SplitN(c, ".", 2)[0])
	if slices.Contains([]string{"con", "prn", "aux", "nul"}, stem) {
		return true
	}
	return len(stem) == 4 && (strings.HasPrefix(stem, "com") || strings.HasPrefix(stem, "lpt")) && stem[3] >= '1' && stem[3] <= '9'
}

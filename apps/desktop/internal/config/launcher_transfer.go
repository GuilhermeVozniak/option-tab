package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	LauncherProfileTransferLimit = 256 * 1024
	launcherProfileFormat        = "option-tab.launcher-profile"
)

var errLauncherTransfer = errors.New("config: invalid launcher profile document")

type launcherProfileEnvelope struct {
	Format  string          `json:"format"`
	Version int             `json:"version"`
	Profile LauncherProfile `json:"profile"`
}

func validTransferProfile(p LauncherProfile) error {
	if strings.IndexFunc(p.Name, unicode.IsControl) >= 0 {
		return errLauncherTransfer
	}
	return ValidateReplacementDock(ReplacementDockSettings{Version: 2, Profiles: []LauncherProfile{p}})
}

func sanitizeTransferProfile(p LauncherProfile) LauncherProfile {
	p = CloneReplacementDock(ReplacementDockSettings{Profiles: []LauncherProfile{p}}).Profiles[0]
	for i := range p.Widgets {
		p.Widgets[i].Enabled = false
		p.Widgets[i].Grants = []string{}
	}
	for i := range p.Items {
		item := &p.Items[i]
		item.IconID = ""
		if item.Kind == "app" || item.Kind == "folder" || item.Kind == "file" {
			sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%s", i, item.Kind, item.ID)))
			item.ReferenceID = "selection-" + hex.EncodeToString(sum[:16])
		}
	}
	return p
}

// Export contains structural configuration only. It never exposes native IDs,
// private assets or executable widget permission/enablement.
func ExportLauncherProfile(profile LauncherProfile) ([]byte, error) {
	if err := validTransferProfile(profile); err != nil {
		return nil, errLauncherTransfer
	}
	profile = sanitizeTransferProfile(profile)
	b, err := json.MarshalIndent(launcherProfileEnvelope{Format: launcherProfileFormat, Version: 1, Profile: profile}, "", "  ")
	if err != nil || len(b) > LauncherProfileTransferLimit {
		return nil, errLauncherTransfer
	}
	return b, nil
}

// Parse validates exact envelope keys and delegates nested duplicate, depth,
// node, array, unknown-field and profile validation to the existing strict decoder.
func ParseLauncherProfile(data []byte) (LauncherProfile, error) {
	bad := func() (LauncherProfile, error) { return LauncherProfile{}, errLauncherTransfer }
	if len(data) == 0 || len(data) > LauncherProfileTransferLimit || !utf8.Valid(data) {
		return bad()
	}
	d := json.NewDecoder(bytes.NewReader(data))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return bad()
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, err = d.Token()
		if err != nil {
			return bad()
		}
		key, ok := t.(string)
		if !ok || (key != "format" && key != "version" && key != "profile") || fields[key] != nil {
			return bad()
		}
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return bad()
		}
		fields[key] = raw
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') || len(fields) != 3 {
		return bad()
	}
	if _, err = d.Token(); err != io.EOF {
		return bad()
	}
	var format string
	var version int
	if json.Unmarshal(fields["format"], &format) != nil || format != launcherProfileFormat || json.Unmarshal(fields["version"], &version) != nil || version != 1 {
		return bad()
	}
	wrapped := append([]byte(`{"version":2,"profiles":[`), fields["profile"]...)
	wrapped = append(wrapped, []byte(`],"bindings":[]}`)...)
	dock, err := decodeReplacement(wrapped)
	if err != nil || len(dock.Profiles) != 1 {
		return bad()
	}
	p := dock.Profiles[0]
	if validTransferProfile(p) != nil {
		return bad()
	}
	return sanitizeTransferProfile(p), nil
}

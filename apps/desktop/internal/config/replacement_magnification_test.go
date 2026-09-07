package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLauncherMagnificationStrictDefaultsAndCopies(t *testing.T) {
	s := DefaultReplacementDock()
	if s.Profiles[0].Magnification == nil || s.Profiles[0].Magnification.Enabled || s.Profiles[0].Magnification.Scale != 1.35 || s.Profiles[0].Magnification.Reach != 2 {
		t.Fatal("magnification defaults")
	}
	b, _ := json.Marshal(s)
	old := strings.Replace(string(b), `"magnification":{"enabled":false,"scale":1.35,"reach":2},`, "", 1)
	if strings.Contains(old, "magnification") {
		t.Fatal("old fixture still contains field")
	}
	loaded, err := decodeReplacement([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profiles[0].Magnification != nil && loaded.Profiles[0].Magnification.Enabled {
		t.Fatal("old profile opted in")
	}
	copied := CloneReplacementDock(s)
	copied.Profiles[0].Magnification.Enabled = true
	if s.Profiles[0].Magnification.Enabled {
		t.Fatal("magnification aliases config")
	}
	for _, raw := range []string{`{"enabled":true,"scale":0,"reach":2}`, `{"enabled":true,"scale":2.01,"reach":2}`, `{"enabled":true,"scale":1.5,"reach":5}`, `{"enabled":true,"scale":1.5,"reach":-1}`, `{"enabled":true,"scale":1.5,"unknown":1}`, `{"enabled":true,"scale":1.5,"Scale":1.7}`, `null`} {
		doc := strings.Replace(string(b), `{"enabled":false,"scale":1.35,"reach":2}`, raw, 1)
		if _, err := decodeReplacement([]byte(doc)); err == nil {
			t.Fatal("accepted invalid magnification", raw)
		}
	}
}

func TestLauncherMagnificationPartialDefaultsAndZeroReach(t *testing.T) {
	s := DefaultReplacementDock()
	b, _ := json.Marshal(s)
	for _, raw := range []string{`{"enabled":true}`, `{"enabled":true,"scale":2,"reach":0}`} {
		doc := strings.Replace(string(b), `{"enabled":false,"scale":1.35,"reach":2}`, raw, 1)
		loaded, err := decodeReplacement([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		m := loaded.Profiles[0].Magnification
		if raw == `{"enabled":true}` && (m.Scale != 1.35 || m.Reach != 2 || !m.Enabled) {
			t.Fatal("missing parameter defaults", m)
		}
		if raw != `{"enabled":true}` && m.Reach != 0 {
			t.Fatal("explicit zero reach overwritten")
		}
	}
}

func TestLauncherMagnificationRejectsVersionOneField(t *testing.T) {
	legacy := `{"version":1,"profiles":[{"id":"default","name":"Default","edge":"bottom","layout":"floating","iconPx":40,"thicknessPx":64,"maxLengthFraction":0.8,"insetPx":16}],"bindings":[]}`
	if _, err := decodeReplacement([]byte(legacy)); err != nil {
		t.Fatal("invalid legacy fixture", err)
	}
	for _, key := range []string{"magnification", "Magnification"} {
		doc := strings.Replace(legacy, `"insetPx":16`, `"insetPx":16,"`+key+`":{"enabled":true,"scale":2,"reach":4}`, 1)
		if _, err := decodeReplacement([]byte(doc)); err == nil {
			t.Fatal("version 1 accepted magnification", key)
		}
	}
}

package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The preferences UI maps over profile.widgets, widget.grants and bindings; a
// nil slice marshals as JSON null and blanks the whole panel.
func assertReplacementJSONHasNoNull(t *testing.T, name string, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", name, err)
	}
	if bytes.Contains(b, []byte("null")) {
		t.Fatalf("%s: slices must marshal as [], got: %s", name, b)
	}
	return b
}

func TestReplacementDockJSON_SlicesNeverNull(t *testing.T) {
	profile := `"id":"default","name":"Default","alignment":"center","appearance":{"theme":"system","material":"solid","tint":"#172033","opacity":0.76,"borderOpacity":0.16,"cornerRadiusPx":18,"itemSpacingPx":6,"showLabels":true},"edge":"bottom","layout":"floating","iconPx":40,"thicknessPx":64,"maxLengthFraction":0.8,"insetPx":16,"autoHide":false`
	clock := `"id":"clock","packageID":"` + BuiltinClockPackage + `","digest":"` + BuiltinClockDigest + `","enabled":false`
	for name, doc := range map[string]string{
		"nullWidgets":     `{"version":2,"enabled":false,"profiles":[{` + profile + `,"widgets":null}],"bindings":[]}`,
		"missingWidgets":  `{"version":2,"enabled":false,"profiles":[{` + profile + `}],"bindings":[]}`,
		"nullBindings":    `{"version":2,"enabled":false,"profiles":[{` + profile + `,"widgets":[]}],"bindings":null}`,
		"missingBindings": `{"version":2,"enabled":false,"profiles":[{` + profile + `,"widgets":[]}]}`,
		"nullGrants":      `{"version":2,"enabled":false,"profiles":[{` + profile + `,"widgets":[{` + clock + `,"grants":null}]}],"bindings":[]}`,
		"missingGrants":   `{"version":2,"enabled":false,"profiles":[{` + profile + `,"widgets":[{` + clock + `}]}],"bindings":[]}`,
		"nullOptional":    `{"version":2,"enabled":false,"rules":null,"profiles":[{` + profile + `,"widgets":null,"stacks":null,"items":null}],"bindings":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := decodeReplacement(json.RawMessage(doc))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			b := assertReplacementJSONHasNoNull(t, name, got)
			for _, want := range []string{`"widgets":[`, `"bindings":[`} {
				if !bytes.Contains(b, []byte(want)) {
					t.Fatalf("missing %s in %s", want, b)
				}
			}
			if len(got.Profiles[0].Widgets) == 1 && !bytes.Contains(b, []byte(`"grants":[]`)) {
				t.Fatalf("grants must marshal as [], got: %s", b)
			}
		})
	}
}

func TestSettingsJSON_ReplacementDockNullSlicesLoadAndResaveAsArrays(t *testing.T) {
	// A settings.json already broken by an older build must load cleanly.
	raw, err := json.Marshal(Default())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	dock := doc["replacementDock"].(map[string]any)
	dock["bindings"] = nil
	dock["profiles"].([]any)[0].(map[string]any)["widgets"] = nil
	broken, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(broken, []byte(`"widgets":null`)) || !bytes.Contains(broken, []byte(`"bindings":null`)) {
		t.Fatalf("fixture is not broken: %s", broken)
	}
	loaded, err := Load(bytes.NewReader(broken))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var saved bytes.Buffer
	if err = Save(&saved, loaded); err != nil {
		t.Fatalf("save: %v", err)
	}
	var out struct {
		ReplacementDock json.RawMessage `json:"replacementDock"`
	}
	if err = json.Unmarshal(saved.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err = json.Compact(&compact, out.ReplacementDock); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"widgets":[]`, `"bindings":[]`} {
		if !strings.Contains(compact.String(), want) {
			t.Fatalf("resaved settings missing %s: %s", want, compact.String())
		}
	}
	if strings.Contains(compact.String(), "null") {
		t.Fatalf("resaved settings still contain null: %s", compact.String())
	}
}

func TestLauncherTransferImportWithoutWidgetsNeverYieldsNullSlices(t *testing.T) {
	exported, err := ExportLauncherProfile(DefaultReplacementDock().Profiles[0])
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err = json.Unmarshal(exported, &envelope); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(map[string]any){
		"missingWidgets": func(p map[string]any) { delete(p, "widgets") },
		"nullWidgets":    func(p map[string]any) { p["widgets"] = nil },
	} {
		t.Run(name, func(t *testing.T) {
			var copyOf map[string]any
			if e := json.Unmarshal(exported, &copyOf); e != nil {
				t.Fatal(e)
			}
			edit(copyOf["profile"].(map[string]any))
			doc, e := json.Marshal(copyOf)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ParseLauncherProfile(doc)
			if e != nil {
				t.Fatalf("parse: %v", e)
			}
			if got.Widgets == nil {
				t.Fatal("imported profile has nil widgets")
			}
			// The import appends the profile to the persisted settings.
			dock := DefaultReplacementDock()
			got.ID = "imported"
			dock.Profiles = append(dock.Profiles, got)
			b := assertReplacementJSONHasNoNull(t, name, dock)
			if bytes.Count(b, []byte(`"widgets":[`)) != 2 {
				t.Fatalf("every profile needs a widgets array: %s", b)
			}
		})
	}

	// Export of an in-memory profile that never went through the decoder.
	bare := DefaultReplacementDock().Profiles[0]
	bare.Widgets = nil
	out, err := ExportLauncherProfile(bare)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("null")) {
		t.Fatalf("export contains null: %s", out)
	}
}

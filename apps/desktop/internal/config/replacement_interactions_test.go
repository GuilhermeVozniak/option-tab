package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLauncherInteractionsDefaultsPartialAndClone(t *testing.T) {
	s := DefaultReplacementDock()
	v := s.Profiles[0].Interactions
	if v == nil || v.Enabled || v.PreciseScroll || v.Pinch || v.Swipe || v.Haptics || v.LetterNavigation || v.EnterActivates || v.PrimaryAction != "next" || v.TowardAction != "showPreview" || v.PinchAction != "showPreview" {
		t.Fatal("interaction defaults", v)
	}
	b, _ := json.Marshal(s)
	needle := `"interactions":{"enabled":false,"preciseScroll":false,"pinch":false,"swipe":false,"primaryAction":"next","towardAction":"showPreview","pinchAction":"showPreview","haptics":false,"letterNavigation":false,"enterActivates":false}`
	absent, err := decodeReplacement([]byte(strings.Replace(string(b), needle+`,`, "", 1)))
	if err != nil || absent.Profiles[0].Interactions == nil || absent.Profiles[0].Interactions.PrimaryAction != "next" {
		t.Fatal("absent interactions did not resolve defaults", absent.Profiles[0].Interactions, err)
	}
	partial := strings.Replace(string(b), needle, `"interactions":{"enabled":true,"preciseScroll":true}`, 1)
	loaded, err := decodeReplacement([]byte(partial))
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Profiles[0].Interactions
	if !got.Enabled || !got.PreciseScroll || got.Pinch || got.PrimaryAction != "next" || got.TowardAction != "showPreview" || got.PinchAction != "showPreview" {
		t.Fatal("partial defaults", got)
	}
	copy := CloneReplacementDock(loaded)
	copy.Profiles[0].Interactions.PrimaryAction = "previous"
	if loaded.Profiles[0].Interactions.PrimaryAction != "next" {
		t.Fatal("clone aliases interaction settings")
	}
}

func TestLauncherInteractionsStrictValidationAndLegacyRefusal(t *testing.T) {
	s := DefaultReplacementDock()
	b, _ := json.Marshal(s)
	needle := `"interactions":{"enabled":false,"preciseScroll":false,"pinch":false,"swipe":false,"primaryAction":"next","towardAction":"showPreview","pinchAction":"showPreview","haptics":false,"letterNavigation":false,"enterActivates":false}`
	for _, raw := range []string{
		`null`,
		`{"primaryAction":"activate","towardAction":"showPreview","pinchAction":"showPreview"}`,
		`{"primaryAction":"next","towardAction":"show","pinchAction":"showPreview"}`,
		`{"primaryAction":"next","towardAction":"showPreview","pinchAction":"open"}`,
		`{"primaryAction":"next","towardAction":"showPreview","pinchAction":"showPreview","unknown":true}`,
		`{"primaryAction":"next","PrimaryAction":"previous","towardAction":"showPreview","pinchAction":"showPreview"}`,
	} {
		if _, err := decodeReplacement([]byte(strings.Replace(string(b), needle, `"interactions":`+raw, 1))); err == nil {
			t.Fatal("accepted invalid interactions", raw)
		}
	}
	legacy := `{"version":1,"profiles":[{"id":"default","name":"Default","edge":"bottom","layout":"floating","iconPx":40,"thicknessPx":64,"maxLengthFraction":0.8,"insetPx":16}],"bindings":[]}`
	for _, key := range []string{"interactions", "Interactions"} {
		doc := strings.Replace(legacy, `"insetPx":16`, `"insetPx":16,"`+key+`":{"enabled":false}`, 1)
		if _, err := decodeReplacement([]byte(doc)); err == nil {
			t.Fatal("version 1 accepted interactions", key)
		}
	}
}

package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDockMiddleClickPersistsIndependentlyOfSwitcherSettings(t *testing.T) {
	for _, action := range []string{"none", "close", "minimize"} {
		t.Run(action, func(t *testing.T) {
			s, err := Load(strings.NewReader(`{"version":3,"behavior":{"middleClickAction":"close"},"appSwitcher":{"behavior":{"middleClickAction":"none"}},"dock":{"input":{"middleClickAction":"` + action + `"}}}`))
			if err != nil {
				t.Fatal(err)
			}
			var data bytes.Buffer
			if err := Save(&data, s); err != nil {
				t.Fatal(err)
			}
			var saved struct {
				Dock struct {
					Input struct {
						MiddleClickAction string `json:"middleClickAction"`
					} `json:"input"`
				} `json:"dock"`
			}
			if err := json.Unmarshal(data.Bytes(), &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Dock.Input.MiddleClickAction != action {
				t.Fatalf("saved middle-click action=%q, want %q", saved.Dock.Input.MiddleClickAction, action)
			}
			if s.Behavior.MiddleClickAction != PointerClose || s.AppSwitcher.Behavior.MiddleClickAction != PointerNone {
				t.Fatal("Dock action changed a switcher preference")
			}
		})
	}
}

func TestDockMiddleClickRejectsUnsupportedActionsAndNormalizesOldSettings(t *testing.T) {
	for _, value := range []PointerAction{"", "quit", "forceQuit", "fullscreen"} {
		s := Default()
		s.Dock.Input.MiddleClickAction = value
		if s.Validate() == nil {
			t.Fatalf("accepted unsupported middle-click action %q", value)
		}
		if got := s.Normalize().Dock.Input.MiddleClickAction; got != PointerNone {
			t.Fatalf("normalized %q to %q, want none", value, got)
		}
	}
	s, err := Load(strings.NewReader(`{"version":3,"dock":{"input":{"previewDrag":true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Dock.Input.MiddleClickAction != PointerNone || !s.Dock.Input.PreviewDrag {
		t.Fatal("adding middle-click changed existing Dock settings")
	}
}

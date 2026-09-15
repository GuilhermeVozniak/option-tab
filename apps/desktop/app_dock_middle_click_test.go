package main

import (
	"encoding/json"
	"strings"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform/fake"
)

func TestDockPreviewPublishesIndependentMiddleClickPolicy(t *testing.T) {
	s, err := config.Load(strings.NewReader(`{"version":3,"dock":{"enabled":true,"input":{"middleClickAction":"minimize"}},"behavior":{"middleClickAction":"close"}}`))
	if err != nil {
		t.Fatal(err)
	}
	p := fake.New()
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "First"}})
	a := newApp(p, s, "")
	defer a.stopCapture()
	a.showDock(dockFixtureState(7, 101, 10), true)
	data, err := json.Marshal(a.dockViewState)
	if err != nil {
		t.Fatal(err)
	}
	var published struct {
		MiddleClickAction string `json:"middleClickAction"`
	}
	if err := json.Unmarshal(data, &published); err != nil {
		t.Fatal(err)
	}
	if published.MiddleClickAction != "minimize" {
		t.Fatalf("preview middle-click policy=%q, want minimize", published.MiddleClickAction)
	}
}

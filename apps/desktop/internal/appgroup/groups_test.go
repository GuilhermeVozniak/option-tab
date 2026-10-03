package appgroup

import (
	"reflect"
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func TestFilteredWindowsDoNotReappearAsWindowlessApps(t *testing.T) {
	apps := []domain.App{{ID: 10, Name: "A"}, {ID: 20, Name: "B"}, {ID: 30, Name: "C"}}
	raw := []domain.Window{{ID: 1, AppID: 10}, {ID: 2, AppID: 20}}
	got := Compose(apps, raw, raw[:1])
	if len(got) != 2 || got[0].App.ID != 10 || got[1].App.ID != 30 || len(got[1].Windows) != 0 {
		t.Fatalf("filtered app must stay excluded and truly windowless app survive: %+v", got)
	}
}

func TestUnknownInventoryIsNotRelabeledWindowless(t *testing.T) {
	apps := []domain.App{{ID: 10, Name: "Filtered"}, {ID: 20, Name: "Unknown"}, {ID: 30, Name: "Empty"}}
	// Even layer-zero metadata belonging to a windowless app can contain CG
	// menu replicas. Explicit native presence controls the empty group label.
	raw := []domain.Window{{ID: 101, AppID: 10}, {ID: 201, AppID: 20}, {ID: 301, AppID: 30}}
	presence := map[domain.AppID]platform.WindowPresence{10: platform.WindowsPresent, 20: platform.WindowsUnknown, 30: platform.WindowsNone}
	groups := ComposeWithPresence(apps, raw, nil, presence)
	if len(groups) != 2 || groups[0].App.ID != 30 || groups[0].Presence != platform.WindowsNone || groups[1].App.ID != 20 || groups[1].Presence != platform.WindowsUnknown {
		t.Fatalf("wrong availability/filtered groups: %+v", groups)
	}
	for _, group := range groups {
		if len(group.Windows) != 0 {
			t.Fatal("invented an actionable window")
		}
	}
}

func TestActualEligibleWindowOverridesUncertainPresence(t *testing.T) {
	apps := []domain.App{{ID: 10, Name: "Example"}}
	windows := []domain.Window{{ID: 101, AppID: 10}}
	groups := ComposeWithPresence(apps, windows, windows, map[domain.AppID]platform.WindowPresence{10: platform.WindowsUnknown})
	if len(groups) != 1 || len(groups[0].Windows) != 1 || groups[0].Presence != platform.WindowsPresent {
		t.Fatalf("lost real window: %+v", groups)
	}
}

func TestGroupsPreserveWindowOrderAndSeparateSameNamedProcesses(t *testing.T) {
	apps := []domain.App{{ID: 10, Name: "Editor"}, {ID: 20, Name: "Editor"}, {ID: 30, Name: "beta"}, {ID: 40, Name: "Alpha"}, {ID: 50, Name: "alpha"}}
	windows := []domain.Window{{ID: 2, AppID: 20}, {ID: 3, AppID: 10}, {ID: 1, AppID: 10}}
	got := Compose(apps, windows, windows)
	ids := []domain.AppID{}
	for _, group := range got {
		ids = append(ids, group.App.ID)
	}
	if !reflect.DeepEqual(ids, []domain.AppID{20, 10, 40, 50, 30}) {
		t.Fatalf("group order = %v", ids)
	}
	if got[1].Windows[0].ID != 3 || got[1].Windows[1].ID != 1 {
		t.Fatalf("selected-app windows lost established order: %+v", got[1])
	}
	got[1].Windows[0].Title = "changed"
	if windows[1].Title != "" {
		t.Fatal("group mutated the caller's window slice")
	}
}

func TestGroupsRejectMissingAppsAndInventedWindows(t *testing.T) {
	apps := []domain.App{{ID: 10, Name: "A"}, {ID: 10, Name: "duplicate"}, {ID: 0, Name: "invalid"}}
	windows := []domain.Window{{ID: 0, AppID: 10}, {ID: 1, AppID: 20}}
	got := Compose(apps, windows, windows)
	if len(got) != 1 || got[0].App.Name != "A" || len(got[0].Windows) != 0 {
		t.Fatalf("must only group actual running app identities: %+v", got)
	}
}

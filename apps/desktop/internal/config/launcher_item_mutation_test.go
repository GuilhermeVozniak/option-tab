package config

import (
	"reflect"
	"testing"
)

func mutationApps(ids ...string) []LauncherItem {
	out := []LauncherItem{}
	for _, id := range ids {
		out = append(out, launcherItemFixture(id, "app"))
	}
	return out
}

func mutationGroup(id string, members ...string) LauncherItem {
	return LauncherItem{ID: id, Kind: "group", Label: "Group", Members: members}
}

func itemIDs(items []LauncherItem) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func TestLauncherItemMutationMovesAndGroups(t *testing.T) {
	tests := []struct {
		name     string
		items    []LauncherItem
		mutation LauncherItemMutation
		wantIDs  []string
		group    string
		members  []string
	}{
		{"before", mutationApps("a", "b", "c"), LauncherItemMutation{"moveBefore", "c", "a"}, []string{"c", "a", "b"}, "", nil},
		{"after", mutationApps("a", "b", "c"), LauncherItemMutation{"moveAfter", "a", "c"}, []string{"b", "c", "a"}, "", nil},
		{"member order", append(mutationApps("a", "b", "c"), mutationGroup("g", "a", "b", "c")), LauncherItemMutation{"moveBefore", "c", "a"}, []string{"a", "b", "c", "g"}, "g", []string{"c", "a", "b"}},
		{"new group", mutationApps("a", "b", "c"), LauncherItemMutation{"addToGroup", "a", "b"}, []string{"a", "group-1", "b", "c"}, "group-1", []string{"b", "a"}},
		{"append group", append(mutationApps("a", "b", "c"), mutationGroup("g", "a", "b")), LauncherItemMutation{"addToGroup", "c", "g"}, []string{"a", "b", "c", "g"}, "g", []string{"a", "b", "c"}},
		{"remove member", append(mutationApps("a", "b", "c"), mutationGroup("g", "a", "b")), LauncherItemMutation{"removeFromGroup", "a", "g"}, []string{"b", "c", "g", "a"}, "g", []string{"b"}},
		{"remove last", append(mutationApps("a", "b"), mutationGroup("g", "a")), LauncherItemMutation{"removeFromGroup", "a", "g"}, []string{"b", "a"}, "", nil},
		{"atomic group", append(mutationApps("a", "b", "c"), mutationGroup("g", "a", "b")), LauncherItemMutation{"moveBefore", "g", "c"}, []string{"a", "b", "g", "c"}, "g", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MutateLauncherItems(tt.items, tt.mutation)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(itemIDs(got), tt.wantIDs) {
				t.Fatal(itemIDs(got), tt.wantIDs)
			}
			if tt.group != "" {
				found := false
				for _, item := range got {
					if item.ID == tt.group {
						found = true
						if !reflect.DeepEqual(item.Members, tt.members) {
							t.Fatal(item.Members, tt.members)
						}
					}
				}
				if !found {
					t.Fatal("group missing")
				}
			}
			if !validLauncherItems(got) {
				t.Fatal("invalid result")
			}
		})
	}
}

func TestLauncherItemMutationRefusesInvalidAuthority(t *testing.T) {
	base := append(mutationApps("a", "b", "c"), mutationGroup("g", "a", "b"), LauncherItem{ID: "space", Kind: "spacer"})
	tests := []LauncherItemMutation{
		{"unknown", "a", "b"},
		{"moveBefore", "a", "a"},
		{"moveAfter", "missing", "b"},
		{"moveBefore", "a", "c"},
		{"moveAfter", "c", "a"},
		{"addToGroup", "c", "a"},
		{"addToGroup", "a", "g"},
		{"addToGroup", "g", "c"},
		{"removeFromGroup", "a", "c"},
		{"removeFromGroup", "c", "g"},
		{"moveBefore", "space", "c"},
	}
	for _, m := range tests {
		before := append([]LauncherItem(nil), base...)
		got, err := MutateLauncherItems(base, m)
		if err == nil || got != nil {
			t.Fatalf("accepted %+v", m)
		}
		if !reflect.DeepEqual(base, before) {
			t.Fatal("mutated input")
		}
	}
	for _, invalid := range [][]LauncherItem{
		append(mutationApps("a"), launcherItemFixture("a", "app")),
		append(mutationApps("a"), mutationGroup("g", "a"), mutationGroup("h", "a")),
		{mutationGroup("g", "h"), mutationGroup("h", "g")},
	} {
		if _, err := MutateLauncherItems(invalid, LauncherItemMutation{"moveBefore", "a", "g"}); err == nil {
			t.Fatal("accepted invalid prior")
		}
	}
}

func TestLauncherItemMutationTransferCapacityAndCopies(t *testing.T) {
	original := append(mutationApps("a", "b", "c"), mutationGroup("old", "a"), mutationGroup("new", "b"))
	got, err := MutateLauncherItems(original, LauncherItemMutation{"addToGroup", "a", "new"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(itemIDs(got), []string{"a", "b", "c", "new"}) || !reflect.DeepEqual(got[3].Members, []string{"b", "a"}) {
		t.Fatal(got)
	}
	got[0].ReferenceID = "changed"
	got[3].Members[0] = "changed"
	if original[0].ReferenceID != "ref-a" || original[4].Members[0] != "b" {
		t.Fatal("aliased native identity or members")
	}
	full := mutationApps("a", "b", "c", "d", "e", "f", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q")
	if _, err := MutateLauncherItems(full, LauncherItemMutation{"addToGroup", "a", "b"}); err == nil {
		t.Fatal("exceeded full record limit")
	}
	full[15] = mutationGroup("group-1", "a")
	result, err := MutateLauncherItems(full, LauncherItemMutation{"addToGroup", "a", "b"})
	if err != nil || len(result) != 16 {
		t.Fatal(result, err)
	}
	collision := append(mutationApps("a", "b"), LauncherItem{ID: "group-1", Kind: "separator"})
	result, err = MutateLauncherItems(collision, LauncherItemMutation{"addToGroup", "a", "b"})
	if err != nil || result[1].ID != "group-2" {
		t.Fatal(result, err)
	}
}

func TestLauncherItemMutationDecorationTargetAndNoopCopy(t *testing.T) {
	items := append(mutationApps("a", "b"), LauncherItem{ID: "space", Kind: "spacer"})
	got, err := MutateLauncherItems(items, LauncherItemMutation{"moveAfter", "a", "space"})
	if err != nil || !reflect.DeepEqual(itemIDs(got), []string{"b", "space", "a"}) {
		t.Fatal(got, err)
	}
	grouped := append(mutationApps("a", "b"), mutationGroup("g", "a", "b"))
	got, err = MutateLauncherItems(grouped, LauncherItemMutation{"moveBefore", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	got[2].Members[0] = "changed"
	if grouped[2].Members[0] != "a" {
		t.Fatal("no-op aliases original")
	}
}

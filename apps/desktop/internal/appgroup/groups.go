// Package appgroup composes application entries from real app/window identities.
package appgroup

import (
	"cmp"
	"slices"
	"strings"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type Group struct {
	App      domain.App
	Windows  []domain.Window
	Presence platform.WindowPresence
}

// Compose preserves eligible window order and app process identities. raw must
// contain actual unfiltered windows, not CG-only surfaces: a filtered app must
// not reappear as windowless merely because none of its windows are eligible.
func Compose(apps []domain.App, raw, eligible []domain.Window) []Group {
	return ComposeWithPresence(apps, raw, eligible, nil)
}

// ComposeWithPresence accepts native evidence about app window availability.
// Apps with confirmed windows filtered out stay excluded. An unknown empty
// group remains explicitly unknown, so inaccessible/off-Space metadata never
// asserts that an app is windowless. Missing map entries use Compose semantics.
func ComposeWithPresence(apps []domain.App, raw, eligible []domain.Window, presence map[domain.AppID]platform.WindowPresence) []Group {
	known := make(map[domain.AppID]domain.App, len(apps))
	for _, app := range apps {
		if _, exists := known[app.ID]; app.ID > 0 && !exists {
			known[app.ID] = app
		}
	}
	counts := make(map[domain.AppID]int)
	for _, window := range raw {
		if window.ID != 0 {
			counts[window.AppID]++
		}
	}
	groups := make([]Group, 0, len(known))
	positions := make(map[domain.AppID]int)
	seen := make(map[domain.WindowID]bool)
	for _, window := range eligible {
		app, exists := known[window.AppID]
		if !exists || window.ID == 0 || seen[window.ID] {
			continue
		}
		seen[window.ID] = true
		index, grouped := positions[app.ID]
		if !grouped {
			index = len(groups)
			positions[app.ID] = index
			groups = append(groups, Group{App: app, Windows: []domain.Window{}, Presence: platform.WindowsPresent})
		}
		groups[index].Windows = append(groups[index].Windows, window)
	}
	empty := make([]domain.App, 0)
	for _, app := range known {
		if _, grouped := positions[app.ID]; grouped {
			continue
		}
		p, explicit := presence[app.ID]
		if (explicit && p != platform.WindowsPresent) || (!explicit && counts[app.ID] == 0) {
			empty = append(empty, app)
		}
	}
	slices.SortFunc(empty, func(a, b domain.App) int {
		if order := cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	for _, app := range empty {
		p, explicit := presence[app.ID]
		if !explicit {
			p = platform.WindowsNone
		} else if p != platform.WindowsNone {
			p = platform.WindowsUnknown
		}
		groups = append(groups, Group{App: app, Windows: []domain.Window{}, Presence: p})
	}
	return groups
}

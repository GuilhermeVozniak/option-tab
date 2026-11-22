package main

import (
	"fmt"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

type iconPlatform struct {
	*fake.Fake
	calls []string
	fail  bool
}

func (p *iconPlatform) AppIcon(pid, maxPx int) string {
	p.calls = append(p.calls, fmt.Sprintf("%d@%d", pid, maxPx))
	if p.fail {
		return ""
	}
	return fmt.Sprintf("icon:%d@%d", pid, maxPx)
}

func iconState(iconPx int, entries ...switcher.Entry) switcher.State {
	appearance := config.Default().Appearance
	appearance.IconSizePx = iconPx
	return switcher.State{Appearance: appearance, Entries: entries}
}

// Icons are rendered at the configured size. Raising the size must render them
// again instead of reusing (and upscaling) the small ones until restart.
func TestEnrichIconsRendersAgainWhenIconSizeChanges(t *testing.T) {
	p := &iconPlatform{Fake: fake.New()}
	a := &App{platform: p}
	entry := switcher.Entry{WindowID: 1, AppID: 10, BundleID: "a"}

	st := iconState(32, entry)
	a.enrichIcons(&st)
	if st.Entries[0].Icon != "icon:10@64" {
		t.Fatalf("icon = %q", st.Entries[0].Icon)
	}
	st = iconState(32, entry)
	a.enrichIcons(&st)
	if len(p.calls) != 1 {
		t.Fatalf("same size rendered again: %v", p.calls)
	}
	st = iconState(128, entry)
	a.enrichIcons(&st)
	if st.Entries[0].Icon != "icon:10@256" {
		t.Fatalf("after raising the size the icon is still %q", st.Entries[0].Icon)
	}
}

// A pid can be reused by a different app; its cached icon must not be.
func TestEnrichIconsDoesNotReuseAnotherAppsIconForTheSamePid(t *testing.T) {
	p := &iconPlatform{Fake: fake.New()}
	a := &App{platform: p}
	st := iconState(32, switcher.Entry{WindowID: 1, AppID: 10, BundleID: "a"})
	a.enrichIcons(&st)
	st = iconState(32, switcher.Entry{WindowID: 2, AppID: 10, BundleID: "b"})
	a.enrichIcons(&st)
	if len(p.calls) != 2 {
		t.Fatalf("pid reused by another bundle kept the old icon: %v", p.calls)
	}
}

// An app that has no icon yet (still launching) must be asked again later.
func TestEnrichIconsRetriesAfterAFailure(t *testing.T) {
	p := &iconPlatform{Fake: fake.New(), fail: true}
	a := &App{platform: p}
	entry := switcher.Entry{WindowID: 1, AppID: 10, BundleID: "a"}
	st := iconState(32, entry)
	a.enrichIcons(&st)
	p.fail = false
	st = iconState(32, entry)
	a.enrichIcons(&st)
	if st.Entries[0].Icon != "icon:10@64" {
		t.Fatalf("a failed icon read was cached forever: %q", st.Entries[0].Icon)
	}
}

func TestEnrichIconsCacheIsBounded(t *testing.T) {
	p := &iconPlatform{Fake: fake.New()}
	a := &App{platform: p}
	for pid := 1; pid <= 3*maxIconCacheEntries; pid++ {
		st := iconState(32, switcher.Entry{WindowID: 1, AppID: domain.AppID(pid), BundleID: "a"})
		a.enrichIcons(&st)
	}
	if len(a.iconCache) > maxIconCacheEntries {
		t.Fatalf("icon cache grew to %d entries", len(a.iconCache))
	}
}

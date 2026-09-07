package platform

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLauncherReferenceInvalidationRetiresPendingAndPublishedAuthority(t *testing.T) {
	a := newLauncherReferenceAuthority()
	first := a.start("ref")
	revision, ok := a.publish("ref", first, 1, "resource")
	if !ok {
		t.Fatal("initial publication refused")
	}
	pending := a.start("ref")
	a.invalidate("ref")
	if a.current("ref", revision, 1, "resource") {
		t.Fatal("removed reference retained authority")
	}
	if _, ok := a.publish("ref", pending, 1, "resource"); ok {
		t.Fatal("late resolution revived removed reference")
	}
	next := a.start("ref")
	replacement, ok := a.publish("ref", next, 1, "resource")
	if !ok || replacement == revision {
		t.Fatal("reselected resource reused retired authority")
	}
}

func TestLauncherLabelsStripControlsAndBoundUnicode(t *testing.T) {
	if got := boundedLauncherLabel(" \x00\n", "Item"); got != "Item" {
		t.Fatal("empty sanitized label omitted fallback", got)
	}
	if got := boundedLauncherLabel("\tEditor\x00\n", "Item"); got != "Editor" {
		t.Fatal("control characters retained", got)
	}
	got := boundedLauncherLabel(strings.Repeat("文", 100), "Item")
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 80 {
		t.Fatal("unicode label not bounded by characters")
	}
}

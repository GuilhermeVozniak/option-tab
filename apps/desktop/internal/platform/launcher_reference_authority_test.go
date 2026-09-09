package platform

import "testing"

func TestLauncherReferenceAuthorityTracksResolutionNotOnlyStoreRevision(t *testing.T) {
	a := newLauncherReferenceAuthority()
	first := a.start("id")
	one, ok := a.publish("id", first, 1, "A")
	if !ok || one == 0 {
		t.Fatal(one)
	}
	next := a.start("id")
	same, ok := a.publish("id", next, 1, "A")
	if !ok || same != one {
		t.Fatal("stable identity lost revision")
	}
	next = a.start("id")
	two, _ := a.publish("id", next, 1, "B")
	next = a.start("id")
	three, _ := a.publish("id", next, 1, "A")
	if two == one || three == one || three == two {
		t.Fatal("A-B-A reused authority")
	}
	if a.current("id", one, 1, "A") {
		t.Fatal("old resource admitted")
	}
	older := a.start("id")
	newer := a.start("id")
	_, _ = a.publish("id", newer, 2, "A")
	if _, ok := a.publish("id", older, 1, "A"); ok {
		t.Fatal("older resolution replaced relink")
	}
}

package platform

import (
	"testing"

	"option-tab/internal/domain"
)

func retirementFixture() (windowIdentity, []domain.Window) {
	id := windowIdentity{Window: 42, PID: 7, StartSec: 100, StartUsec: 8}
	return id, []domain.Window{{ID: 42, PID: 7, OnScreen: false, Minimized: true, Hidden: true}}
}

func TestWindowRetirementRequiresPositiveObservedDestruction(t *testing.T) {
	var r windowRetirements
	id, ws := retirementFixture()
	inventory := func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }
	if got := r.filter(ws, inventory, func(windowIdentity) bool { t.Fatal("unobserved root queried"); return false }); len(got) != 1 {
		t.Fatal("offscreen/minimized/hidden window removed")
	}
	ticket := r.observe(id)
	r.finish(ticket)
	if got := r.filter(ws, inventory, nil); len(got) != 1 {
		t.Fatal("ordinary stream completion retired window")
	}
	ticket = r.observe(id)
	if !r.destroyed(ticket) {
		t.Fatal("positive exact destruction rejected")
	}
	r.finish(ticket)
	if got := r.filter(ws, inventory, func(windowIdentity) bool { return false }); len(got) != 0 {
		t.Fatal("observed closed surface retained")
	}
}

func TestWindowRetirementRejectsUnknownIdentityAndStaleCallbacks(t *testing.T) {
	var r windowRetirements
	id, ws := retirementFixture()
	for _, bad := range []windowIdentity{{}, {Window: 42, PID: 7}, {Window: 42, StartSec: 1}} {
		if r.destroyed(r.observe(bad)) {
			t.Fatal("unknown identity retired")
		}
	}
	old := r.observe(id)
	newer := r.observe(id)
	if r.destroyed(old) {
		t.Fatal("stale observer callback accepted")
	}
	if !r.destroyed(newer) {
		t.Fatal("current callback rejected")
	}
	if got := r.filter(ws, func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }, func(windowIdentity) bool { return true }); len(got) != 1 {
		t.Fatal("positive reappearance not admitted")
	}
	if r.destroyed(old) || r.destroyed(newer) {
		t.Fatal("old observer revived after reappearance")
	}
}

func TestWindowRetirementReappearanceCannotClearNewerDestruction(t *testing.T) {
	var r windowRetirements
	id, ws := retirementFixture()
	ticket := r.observe(id)
	r.destroyed(ticket)
	got := r.filter(ws, func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }, func(windowIdentity) bool { r.destroyed(ticket); return true })
	if len(got) != 0 {
		t.Fatal("stale positive validation cleared newer destruction")
	}
}

func TestWindowRetirementOwnerLaunchReuseAndSuccessfulPrune(t *testing.T) {
	for _, replacement := range []windowIdentity{{Window: 42, PID: 8, StartSec: 100, StartUsec: 8}, {Window: 42, PID: 7, StartSec: 101, StartUsec: 8}, {Window: 42, PID: 7, StartSec: 100, StartUsec: 9}} {
		var r windowRetirements
		id, ws := retirementFixture()
		ticket := r.observe(id)
		r.destroyed(ticket)
		if got := r.filter(ws, func() ([]windowIdentity, bool) { return []windowIdentity{replacement}, true }, nil); len(got) != 1 {
			t.Fatal("replacement hidden")
		}
		if r.destroyed(ticket) {
			t.Fatal("old identity survived complete changed-owner inventory")
		}
	}
	var r windowRetirements
	id, ws := retirementFixture()
	ticket := r.observe(id)
	r.destroyed(ticket)
	r.filter(ws, func() ([]windowIdentity, bool) { return nil, true }, nil)
	if r.destroyed(ticket) {
		t.Fatal("disappeared identity not pruned")
	}
}

func TestWindowRetirementFailedInventoryDoesNotPrune(t *testing.T) {
	var r windowRetirements
	id, ws := retirementFixture()
	ticket := r.observe(id)
	r.destroyed(ticket)
	if got := r.filter(ws, func() ([]windowIdentity, bool) { return nil, false }, nil); len(got) != 1 {
		t.Fatal("failed identity query hid candidate")
	}
	if got := r.filter(ws, func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }, nil); len(got) != 0 {
		t.Fatal("failed query pruned positive evidence")
	}
	unknown := id
	unknown.StartSec = 0
	unknown.StartUsec = 0
	if got := r.filter(ws, func() ([]windowIdentity, bool) { return []windowIdentity{unknown}, true }, nil); len(got) != 1 {
		t.Fatal("unknown current launch hidden")
	}
	if got := r.filter(ws, func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }, nil); len(got) != 0 {
		t.Fatal("unknown launch destroyed evidence")
	}
}

func TestWindowRetirementStaleInventoryCannotPruneNewEvidence(t *testing.T) {
	var r windowRetirements
	id, windows := retirementFixture()
	ticket := r.observe(id)
	r.destroyed(ticket)
	var newer windowObservation
	r.filter(windows, func() ([]windowIdentity, bool) { newer = r.observe(id); r.destroyed(newer); return nil, true }, nil)
	if got := r.filter(windows, func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }, nil); len(got) != 0 {
		t.Fatal("stale full inventory pruned newer destruction")
	}
}

func TestWindowRetirementReappearanceReleasesFinishedObservation(t *testing.T) {
	var r windowRetirements
	id, windows := retirementFixture()
	ticket := r.observe(id)
	r.destroyed(ticket)
	r.finish(ticket)
	r.filter(windows, func() ([]windowIdentity, bool) { return []windowIdentity{id}, true }, func(windowIdentity) bool { return true })
	if len(r.windows) != 0 {
		t.Fatal("finished reappeared observation leaked registry state")
	}
	if r.destroyed(ticket) {
		t.Fatal("retired observer affected reappeared identity")
	}
}

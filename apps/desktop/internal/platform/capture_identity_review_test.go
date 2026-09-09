package platform

import "testing"

func TestCapturePeerRetirementSurvivesOtherObserverFinishing(t *testing.T) {
	var retirements windowRetirements
	identity := windowIdentity{Window: 42, PID: 123, StartSec: 456, StartUsec: 789}
	stillActive := retirements.observe(identity)
	hiddenPeer := retirements.observe(identity)
	retirements.finish(hiddenPeer)
	if !retirements.destroyed(stillActive) {
		t.Fatal("positive destruction from still-active stream was discarded after peer hid")
	}
}

func TestCapturePeersRaceFinishAndDestructionRetainsEvidence(t *testing.T) {
	for i := 0; i < 64; i++ {
		var r windowRetirements
		id := windowIdentity{Window: 42, PID: 123, StartSec: 456, StartUsec: 789}
		first, second := r.observe(id), r.observe(id)
		start := make(chan struct{})
		finished := make(chan struct{})
		destroyed := make(chan bool, 1)
		go func() { <-start; r.finish(second); close(finished) }()
		go func() { <-start; destroyed <- r.destroyed(first) }()
		close(start)
		<-finished
		if !<-destroyed {
			t.Fatal("finishing one peer invalidated another active observer")
		}
		r.finish(first)
		r.mu.Lock()
		state, exists := r.windows[id.Window]
		r.mu.Unlock()
		if !exists || !state.retired || len(state.observers) != 0 {
			t.Fatal("last watcher close removed positive evidence")
		}
		replacement := id
		replacement.StartUsec++
		current := r.observe(replacement)
		if r.destroyed(first) || r.destroyed(second) {
			t.Fatal("old process ticket retired replacement")
		}
		if !r.destroyed(current) {
			t.Fatal("replacement observer not active")
		}
	}
}

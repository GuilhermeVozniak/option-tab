package platform

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestSessionReasonsRemainIndependent(t *testing.T) {
	var r sessionReducer
	active := sessionRaw{valid: true, onConsole: true, loginDone: true, lockState: -1}
	if s := r.reduce(active); s.Inactive {
		t.Fatalf("baseline: %+v", s)
	}
	locked := active
	locked.lockEvent = 1
	if s := r.reduce(locked); !s.Inactive || s.Reason != "locked" {
		t.Fatalf("lock: %+v", s)
	}
	wake := active
	wake.screenEvent = 2
	if s := r.reduce(wake); !s.Inactive || s.Reason != "locked" {
		t.Fatalf("wake cleared lock: %+v", s)
	}
	away := active
	away.sessionEvent = 1
	r.reduce(away)
	unlock := active
	unlock.lockEvent = 2
	if s := r.reduce(unlock); !s.Inactive || s.Reason != "sessionAway" {
		t.Fatalf("unlock cleared resigned session: %+v", s)
	}
	back := active
	back.sessionEvent = 2
	back.screenEvent = 1
	if s := r.reduce(back); !s.Inactive || s.Reason != "screensAsleep" {
		t.Fatalf("session return cleared screen sleep: %+v", s)
	}
	if s := r.reduce(wake); s.Inactive {
		t.Fatalf("all reasons cleared: %+v", s)
	}
}

func TestSessionInitialUnavailableAndDictionaryLock(t *testing.T) {
	var r sessionReducer
	if s := r.reduce(sessionRaw{}); !s.Inactive || s.Reason != "unavailable" {
		t.Fatalf("unknown startup: %+v", s)
	}
	raw := sessionRaw{valid: true, onConsole: false, loginDone: true, lockState: -1}
	if s := r.reduce(raw); !s.Inactive {
		t.Fatal("inactive launch admitted")
	}
	raw.onConsole = true
	raw.sessionEvent = 2
	raw.lockState = 1
	if s := r.reduce(raw); s.Reason != "locked" {
		t.Fatalf("dictionary lock ignored: %+v", s)
	}
	raw.lockEvent = 2
	if s := r.reduce(raw); !s.Inactive {
		t.Fatal("unlock hint overrode current dictionary lock")
	}
	raw.lockState = 0
	if s := r.reduce(raw); s.Inactive {
		t.Fatalf("dictionary unlock: %+v", s)
	}
}

type sessionTestPoller struct {
	reads   chan sessionRaw
	closed  chan struct{}
	polling chan struct{}
}

func (p *sessionTestPoller) Poll() (sessionRaw, error) {
	if p.polling != nil {
		p.polling <- struct{}{}
	}
	return <-p.reads, nil
}
func (p *sessionTestPoller) Close() { close(p.closed) }

func TestSessionCancellationJoinsPollAndDropsLateDelivery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := &sessionTestPoller{make(chan sessionRaw), make(chan struct{}), make(chan struct{}, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		done := make(chan error, 1)
		go func() {
			done <- runSessionObservation(ctx, func(SessionState) { calls++ }, func() (sessionPoller, error) { return p, nil }, time.Millisecond)
		}()
		<-p.polling
		cancel()
		synctest.Wait()
		select {
		case <-done:
			t.Error("returned before in-flight native poll cleanup")
		default:
		}
		p.reads <- sessionRaw{valid: true, onConsole: true, loginDone: true, lockState: 0}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		select {
		case <-p.closed:
		default:
			t.Fatal("native cleanup missing")
		}
		if calls != 0 {
			t.Fatal("cancelled callback delivered")
		}
	})
}

func TestSessionSlowCallbackCoalescesAndGenerationChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := &sessionTestPoller{make(chan sessionRaw, 20), make(chan struct{}), nil}
		active := sessionRaw{valid: true, onConsole: true, loginDone: true, lockState: 0}
		p.reads <- active
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		states := []SessionState{}
		go func() {
			done <- runSessionObservation(ctx, func(s SessionState) {
				states = append(states, s)
				if len(states) == 1 {
					close(entered)
					<-release
				}
			}, func() (sessionPoller, error) { return p, nil }, time.Millisecond)
		}()
		<-entered
		for i := range 8 {
			raw := active
			if i%2 == 0 {
				raw.lockState = 1
			}
			p.reads <- raw
			time.Sleep(time.Millisecond)
			synctest.Wait()
		}
		close(release)
		synctest.Wait()
		if len(states) != 2 || states[1].Inactive || states[1].Generation <= states[0].Generation {
			t.Errorf("latest state not coalesced: %+v", states)
		}
		cancel()
		synctest.Wait()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestSessionFactoryFailure(t *testing.T) {
	err := runSessionObservation(context.Background(), func(SessionState) {}, func() (sessionPoller, error) { return nil, errors.New("fixture") }, time.Millisecond)
	if err == nil {
		t.Fatal("factory failure hidden")
	}
}

func TestSessionCoalescedNativeTransitionStillInvalidatesGeneration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := &sessionTestPoller{make(chan sessionRaw, 2), make(chan struct{}), nil}
		raw := sessionRaw{valid: true, onConsole: true, loginDone: true, lockState: 0}
		p.reads <- raw
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		states := []SessionState{}
		done := make(chan error, 1)
		go func() {
			done <- runSessionObservation(ctx, func(s SessionState) { states = append(states, s) }, func() (sessionPoller, error) { return p, nil }, time.Millisecond)
		}()
		synctest.Wait()
		raw.revision = 2 // Lock/unlock completed between polls; final state is active.
		p.reads <- raw
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if len(states) != 2 || states[1].Generation <= states[0].Generation || states[1].Inactive {
			t.Errorf("transition lost: %+v", states)
		}
		cancel()
		synctest.Wait()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

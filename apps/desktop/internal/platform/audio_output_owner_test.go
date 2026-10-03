package platform

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeAudioNative struct {
	read     func() AudioOutputSnapshot
	selectFn func(string, func() error) error
	closed   chan struct{}
}

func (f *fakeAudioNative) Read() AudioOutputSnapshot               { return f.read() }
func (f *fakeAudioNative) Select(uid string, g func() error) error { return f.selectFn(uid, g) }
func (f *fakeAudioNative) Close()                                  { close(f.closed) }
func (f *fakeAudioNative) Dirty() bool                             { return true }
func TestAudioOutputCancelRetiresAndJoinsSelection(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	native := &fakeAudioNative{closed: make(chan struct{}), read: func() AudioOutputSnapshot {
		return AudioOutputSnapshot{Status: "ready", Devices: []AudioOutputDevice{{UID: "one", Alive: true, OutputChannels: 2}}}
	}, selectFn: func(_ string, guard func() error) error { close(entered); <-release; return guard() }}
	source := newAudioOutputSource(func() (audioOutputNative, error) { return native, nil })
	ctx, cancel := context.WithCancel(context.Background())
	states := make(chan AudioOutputSnapshot, 4)
	done := make(chan error, 1)
	go func() {
		done <- source.ObserveAudioOutputs(ctx, func(s AudioOutputSnapshot) {
			select {
			case states <- s:
			default:
			}
		})
	}()
	state := <-states
	selection := make(chan error, 1)
	go func() {
		selection <- source.SelectAudioOutput(context.Background(), state.Generation, "one", func() error { return nil })
	}()
	<-entered
	cancel()
	select {
	case <-native.closed:
		t.Fatal("native closed before in-flight selection joined")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-selection; err == nil {
		t.Fatal("cancelled source dispatched")
	}
	<-done
	select {
	case <-native.closed:
	default:
		t.Fatal("native not closed")
	}
	if source.SelectAudioOutput(context.Background(), state.Generation, "one", func() error { return nil }) == nil {
		t.Fatal("retired generation accepted")
	}
}

func TestAudioOutputSnapshotsCopiedAndBusyRefuses(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	shared := []AudioOutputDevice{{UID: "one", Alive: true, OutputChannels: 2}}
	native := &fakeAudioNative{closed: make(chan struct{}), read: func() AudioOutputSnapshot { return AudioOutputSnapshot{Status: "ready", Devices: shared} }, selectFn: func(_ string, g func() error) error { once.Do(func() { close(entered) }); <-release; return g() }}
	source := newAudioOutputSource(func() (audioOutputNative, error) { return native, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	states := make(chan AudioOutputSnapshot, 1)
	done := make(chan error, 1)
	go func() {
		done <- source.ObserveAudioOutputs(ctx, func(s AudioOutputSnapshot) {
			select {
			case states <- s:
			default:
			}
		})
	}()
	st := <-states
	st.Devices[0].UID = "mutated"
	if shared[0].UID != "one" {
		t.Fatal("aliased callback")
	}
	selected := make(chan error, 1)
	go func() { selected <- source.SelectAudioOutput(ctx, st.Generation, "one", func() error { return nil }) }()
	<-entered
	if !errors.Is(source.SelectAudioOutput(ctx, st.Generation, "one", func() error { return nil }), errAudioBusy) {
		t.Fatal("busy selection not refused")
	}
	close(release)
	if err := <-selected; err != nil {
		t.Fatal(err)
	}
	cancel()
	<-done
}

func TestAudioOutputListenerFailureRetiresOwner(t *testing.T) {
	native := &fakeAudioNative{closed: make(chan struct{}), read: func() AudioOutputSnapshot {
		return AudioOutputSnapshot{Status: "unavailable", Reason: "listenerUnavailable"}
	}}
	source := newAudioOutputSource(func() (audioOutputNative, error) { return native, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- source.ObserveAudioOutputs(ctx, func(AudioOutputSnapshot) {}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("listener failure returned success")
		}
	case <-time.After(100 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("failed listener kept an unobserved source alive")
	}
	select {
	case <-native.closed:
	default:
		t.Fatal("failed owner not joined")
	}
}

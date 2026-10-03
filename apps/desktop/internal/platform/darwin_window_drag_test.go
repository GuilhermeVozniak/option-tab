//go:build darwin

package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWindowDragCancelledContextInstallsNothing(t *testing.T) {
	p := &darwinPlatform{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.ObserveWindowDrags(ctx, func(WindowDragEvent) { t.Error("cancelled source emitted") }); err != nil {
		t.Fatal(err)
	}
	if p.WindowDragGestureCurrent(WindowDragValidation{}) {
		t.Fatal("missing drag validated")
	}
}

func TestWindowDragInvalidArgumentsRefuse(t *testing.T) {
	p := &darwinPlatform{}
	if err := p.ObserveWindowDrags(context.Background(), nil); err == nil {
		t.Fatal("nil callback accepted")
	}
	if err := p.PerformOtherWindowAction("close", 0, 0); err == nil {
		t.Fatal("unknown window action accepted")
	}
	if err := p.PerformOtherWindowAction("fullscreen", 42, 7); err == nil {
		t.Fatal("unsupported bulk action accepted")
	}
}

func TestWindowDragPassiveNativeSeam(t *testing.T) {
	for name, pass := range nativeWindowDragProbe() {
		if !pass {
			t.Error(name)
		}
	}
}

func TestWindowDragObservationJoinsNativeLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runNativeWindowDrags(ctx, func(WindowDragEvent) {}, true) }()
	deadline := time.Now().Add(time.Second)
	for !nativeWindowDragActive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !nativeWindowDragActive() {
		cancel()
		t.Fatal("test observation not started")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("native source did not join")
	}
	if nativeWindowDragActive() {
		t.Fatal("native lifetime remains after cancellation")
	}
}

func TestWindowDragFailedSecondObserverCannotClearFirstEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runNativeWindowDrags(ctx, func(WindowDragEvent) {}, true) }()
	deadline := time.Now().Add(time.Second)
	for !nativeWindowDragActive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !nativeWindowDragActive() {
		cancel()
		t.Fatal("first observation not started")
	}
	before := nativeWindowDragEvidenceEpoch()
	err := runNativeWindowDrags(context.Background(), func(WindowDragEvent) {}, true)
	after := nativeWindowDragEvidenceEpoch()
	cancel()
	if firstErr := <-done; firstErr != nil {
		t.Fatal(firstErr)
	}
	if err == nil {
		t.Fatal("concurrent source was accepted")
	}
	if after != before {
		t.Fatal("failed observer cleared first observer evidence")
	}
}

func TestWindowDragPermissionDenialAndRevocation(t *testing.T) {
	var allowed atomic.Bool
	if err := runWindowDragObservation(context.Background(), func(WindowDragEvent) {}, true, allowed.Load, time.Millisecond); err == nil {
		t.Fatal("permission denial accepted")
	}
	if nativeWindowDragActive() {
		t.Fatal("denied observation installed native lifetime")
	}
	allowed.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runWindowDragObservation(ctx, func(WindowDragEvent) {}, true, allowed.Load, time.Millisecond)
	}()
	deadline := time.Now().Add(time.Second)
	for !nativeWindowDragActive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !nativeWindowDragActive() {
		t.Fatal("allowed observation did not start")
	}
	allowed.Store(false)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("revocation was not reported")
		}
	case <-time.After(time.Second):
		t.Fatal("revoked observation did not join")
	}
	if nativeWindowDragActive() {
		t.Fatal("revoked native source remains active")
	}
}

func TestWindowDragFinalNativeGuardRefusal(t *testing.T) {
	refused := errors.New("retired after native preparation")
	called := false
	err := nativeGuardedOtherWindowProbe(func() error { called = true; _ = nativeWindowDragEvidenceEpoch(); return refused })
	if !called || err != refused {
		t.Fatalf("guard refusal lost: called=%v err=%v", called, err)
	}
	if err := nativeGuardedOtherWindowProbe(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestWindowDragListeningFailureJoinsWithAXStillAllowed(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	refused := errors.New("input listening permission revoked")
	health := func() error {
		if !healthy.Load() {
			return refused
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runWindowDragObservationWithHealth(ctx, func(WindowDragEvent) {}, true, func() bool { return true }, time.Millisecond, health)
	}()
	deadline := time.Now().Add(time.Second)
	for !nativeWindowDragActive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !nativeWindowDragActive() {
		t.Fatal("source did not start")
	}
	healthy.Store(false)
	select {
	case err := <-done:
		if err != refused {
			t.Fatalf("listening refusal lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("failed listening source did not join")
	}
	if nativeWindowDragActive() {
		t.Fatal("failed listening source still active")
	}
}

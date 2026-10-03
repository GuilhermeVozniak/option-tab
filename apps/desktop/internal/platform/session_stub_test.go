//go:build !darwin

package platform

import (
	"context"
	"testing"
)

func TestSessionStubExplicitlyUnsupported(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	source, ok := p.(SessionObservationSource)
	if !ok {
		t.Fatal("stub lacks session capability")
	}
	called := false
	if err := source.ObserveSession(context.Background(), func(SessionState) { called = true }); err == nil {
		t.Fatal("stub reported fake success")
	}
	if called {
		t.Fatal("stub emitted fake native state")
	}
}

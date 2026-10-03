//go:build !darwin

package platform

import (
	"context"
	"testing"
)

func TestWindowDragStubFailsClosed(t *testing.T) {
	p := &stub{}
	if err := p.ObserveWindowDrags(context.Background(), func(WindowDragEvent) {}); err == nil {
		t.Fatal("unsupported observation accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.ObserveWindowDrags(ctx, func(WindowDragEvent) {}); err != nil {
		t.Fatal(err)
	}
	if p.WindowDragGestureCurrent(WindowDragValidation{}) {
		t.Fatal("unsupported gesture validated")
	}
	if _, err := p.ActionWindowRoles(); err == nil {
		t.Fatal("unknown roles treated as known")
	}
	if err := p.PerformOtherWindowAction("close", 42, 7); err == nil {
		t.Fatal("unsupported action accepted")
	}
}

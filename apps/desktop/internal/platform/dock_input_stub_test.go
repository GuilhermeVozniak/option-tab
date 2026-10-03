//go:build !darwin

package platform

import (
	"context"
	"testing"
)

func TestStubDockInputUnsupportedWhenEnabled(t *testing.T) {
	p := &stub{}
	if err := p.ObserveDockInput(context.Background(), DockInputPolicy{ClickToHide: true}, nil, nil); err == nil {
		t.Fatal("unsupported filter reported success")
	}
	if err := p.ObserveDockInput(context.Background(), DockInputPolicy{}, nil, nil); err != nil {
		t.Fatal("disabled filter should install nothing", err)
	}
	if p.DockInputGestureCurrent(1, 1) {
		t.Fatal("stub authorized native action")
	}
	p.CompleteDockInputGesture(1, 1)
}

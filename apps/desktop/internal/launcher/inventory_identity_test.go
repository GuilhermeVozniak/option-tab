package launcher

import (
	"context"
	"errors"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// exitedApp lists two apps but can no longer identify one of them, which is
// what the native layer reports when an app quits between the app listing and
// the per-process identity read.
type exitedApp struct{ *fixture }

func (exitedApp) Apps() ([]domain.App, error) {
	return []domain.App{
		{ID: 42, Name: "Alive", BundleID: "alive"},
		{ID: 43, Name: "Exited", BundleID: "exited"},
	}, nil
}

func (exitedApp) ProcessIdentity(id domain.AppID) (platform.ProcessIdentity, error) {
	if id == 43 {
		return platform.ProcessIdentity{}, errors.New("process exited")
	}
	return platform.ProcessIdentity{PID: id, StartSeconds: 1}, nil
}

func TestLauncherStaysVisibleWhenOneAppExitsDuringInventory(t *testing.T) {
	f := &fixture{starts: make(chan struct{}, 4), views: make(chan State, 64)}
	source := exitedApp{f}
	c := New(Deps{Environment: f, Applications: source, Identities: source, View: f})
	s := config.DefaultReplacementDock()
	s.Enabled = true
	if err := c.Configure(s); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	recv(t, f.starts)
	f.send(env())
	p := visible(t, f)
	names := []string{}
	for _, item := range p.Items {
		names = append(names, item.Name)
	}
	if len(names) != 1 || names[0] != "Alive" {
		t.Fatalf("items = %v, want only the identifiable app", names)
	}
	cancel()
	<-done
}

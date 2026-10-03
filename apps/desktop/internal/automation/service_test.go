package automation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type identities struct {
	process platform.ProcessIdentity
	window  platform.AutomationWindowIdentity
	current bool
}

func (i *identities) ProcessIdentity(domain.AppID) (platform.ProcessIdentity, error) {
	return i.process, nil
}

func (i *identities) WindowIdentity(domain.WindowID) (platform.AutomationWindowIdentity, error) {
	return i.window, nil
}

func (i *identities) WindowIdentityCurrent(platform.AutomationWindowIdentity) bool { return i.current }

type performer struct {
	before func()
	calls  int
}

func (p *performer) PerformAutomationWindowAction(_ context.Context, _ string, _ platform.AutomationWindowIdentity, _ *bool, g func() error) error {
	if p.before != nil {
		p.before()
	}
	if err := g(); err != nil {
		return err
	}
	p.calls++
	return nil
}

func fixture() (Deps, *identities, *performer) {
	identity := platform.ProcessIdentity{PID: 7, StartSeconds: 10, StartMicros: 2}
	ids := &identities{identity, platform.AutomationWindowIdentity{ID: 9, Process: identity}, true}
	p := &performer{}
	return Deps{Apps: func(context.Context) ([]domain.App, error) {
		return []domain.App{{ID: 7, Name: "Música", BundleID: "fixture.app"}}, nil
	}, Windows: func(context.Context) ([]domain.Window, error) {
		return []domain.Window{{ID: 9, AppID: 7, Title: "fixture"}}, nil
	}, Identities: ids, Actions: p}, ids, p
}

func request(op platform.AutomationOperation) platform.AutomationRequest {
	return platform.AutomationRequest{ID: 1, Operation: op}
}

func TestExactSelectorsAndAmbiguity(t *testing.T) {
	d, _, _ := fixture()
	d.ShowPreviews = func(context.Context, platform.ProcessIdentity, []domain.Window, *platform.AutomationPoint, func() error) (Presentation, error) {
		return Presentation{Token: "1", Status: "accepted"}, nil
	}
	for _, selector := range []platform.AutomationAppSelector{{Name: "Música"}, {BundleID: "fixture.app"}, {PID: 7}} {
		r := request(platform.AutomationShowPreviews)
		r.App = &selector
		if got := New(d).Handle(context.Background(), r); got.ErrorCode != "" {
			t.Fatal(got)
		}
	}
	r := request(platform.AutomationShowPreviews)
	r.App = &platform.AutomationAppSelector{Name: "música"}
	if got := New(d).Handle(context.Background(), r); got.ErrorCode != "notFound" {
		t.Fatal(got)
	}
	d.Apps = func(context.Context) ([]domain.App, error) {
		return []domain.App{{ID: 7, Name: "Música"}, {ID: 8, Name: "Música"}}, nil
	}
	r.App.Name = "Música"
	if got := New(d).Handle(context.Background(), r); got.ErrorCode != "ambiguous" {
		t.Fatal(got)
	}
	d.Apps = func(context.Context) ([]domain.App, error) {
		return []domain.App{{ID: 7, Name: "Música"}}, errors.New("partial")
	}
	if got := New(d).Handle(context.Background(), r); got.ErrorCode != "unavailable" {
		t.Fatal(got)
	}
}

func TestActionRetiresDuringNativePreparation(t *testing.T) {
	for _, replace := range []bool{false, true} {
		d, ids, p := fixture()
		ctx, cancel := context.WithCancel(context.Background())
		p.before = func() {
			if replace {
				ids.process.StartSeconds++
			} else {
				cancel()
			}
		}
		r := request(platform.AutomationWindowAction)
		r.WindowID = 9
		r.Action = "minimize"
		got := New(d).Handle(ctx, r)
		cancel()
		if got.ErrorCode == "" || p.calls != 0 {
			t.Fatalf("retired native dispatch: %+v %d", got, p.calls)
		}
	}
}

func TestDefaultQueriesNeverReadImagesAndBoundEntries(t *testing.T) {
	d, _, _ := fixture()
	d.Apps = func(context.Context) ([]domain.App, error) {
		out := make([]domain.App, 501)
		for i := range out {
			out[i] = domain.App{ID: 7, Name: "fixture"}
		}
		return out, nil
	}
	d.CachedFrames = func(context.Context) ([]CachedFrame, error) { panic("default query touched image cache") }
	got := New(d).Handle(context.Background(), request(platform.AutomationQueryApps))
	if got.ErrorCode != "" {
		t.Fatal(got)
	}
	var result struct {
		SchemaVersion int
		Apps          []json.RawMessage
		Omitted       int
		Truncated     bool
	}
	if err := json.Unmarshal(got.JSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || len(result.Apps) != 500 || result.Omitted != 1 || !result.Truncated {
		t.Fatalf("bounds: %+v", result)
	}
}

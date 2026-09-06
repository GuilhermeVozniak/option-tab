package actions

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type othersBackend struct {
	*fake.Fake
	roots []platform.ActionWindowRole
	err   error
	read  func()
	fail  domain.WindowID
	calls []domain.WindowID
	kinds []string
}

func (f *othersBackend) ActionWindowRoles() ([]platform.ActionWindowRole, error) {
	if f.read != nil {
		f.read()
	}
	return append([]platform.ActionWindowRole(nil), f.roots...), f.err
}

func (f *othersBackend) PerformOtherWindowAction(kind string, id domain.WindowID, _ domain.AppID) error {
	f.calls = append(f.calls, id)
	f.kinds = append(f.kinds, kind)
	if id == f.fail {
		return errors.New("native relationship changed")
	}
	return nil
}

func (f *othersBackend) PerformOtherWindowActionGuarded(kind string, id domain.WindowID, app domain.AppID, guard func() error) error {
	if err := guard(); err != nil {
		return err
	}
	return f.PerformOtherWindowAction(kind, id, app)
}

func normalOther(id domain.WindowID, app domain.AppID) platform.ActionWindowRole {
	return platform.ActionWindowRole{WindowRole: platform.WindowRole{WindowID: id, AppID: app, Role: "AXWindow", Subrole: "AXStandardWindow"}, RootConfirmed: true, RelationshipsKnown: true}
}

func othersFixture() *othersBackend {
	return &othersBackend{Fake: fake.New(), roots: []platform.ActionWindowRole{normalOther(1, 10), normalOther(2, 10), normalOther(3, 20)}}
}

func TestOthersPreservesKeepSelfDialogsAndModalParents(t *testing.T) {
	for _, kind := range []string{"closeOthers", "minimizeOthers"} {
		t.Run(kind, func(t *testing.T) {
			f := othersFixture()
			for i, variant := range []string{"dialog", "sheet", "modal", "attached", "child", "parent", "self", "self bundle"} {
				w := normalOther(domain.WindowID(10+i), 30)
				switch variant {
				case "dialog":
					w.Subrole = "AXDialog"
				case "sheet":
					w.Role = "AXSheet"
				case "modal":
					w.Modal = true
				case "attached":
					w.HasAttachedSheet = true
				case "child":
					w.HasModalChild = true
				case "parent":
					w.ParentWindowID = 99
				case "self":
					w.AppID = domain.AppID(os.Getpid())
				case "self bundle":
					w.SelfApplication = true
				}
				f.roots = append(f.roots, w)
			}
			r, err := New(f).PerformOthers(kind, 1, 10)
			if err != nil || r.Succeeded != 2 || len(r.Failures) != 0 || !reflect.DeepEqual(f.calls, []domain.WindowID{2, 3}) {
				t.Fatalf("result=%+v err=%v calls=%v", r, err, f.calls)
			}
			want := "close"
			if kind == "minimizeOthers" {
				want = "setMinimized"
			}
			if !reflect.DeepEqual(f.kinds, []string{want, want}) {
				t.Fatalf("wrong native operations: %v", f.kinds)
			}
		})
	}
}

func TestOthersReportsUncertainRelationshipsAndContinuesAfterNativeRefusal(t *testing.T) {
	f := othersFixture()
	f.fail = 2
	f.err = errors.New("one app denied enumeration")
	unknown := normalOther(4, 30)
	unknown.RelationshipsKnown = false
	unknown.Reason = "sheet lookup denied"
	f.roots = append(f.roots, unknown)
	r, err := New(f).PerformOthers("closeOthers", 1, 10)
	if err != nil || r.Succeeded != 1 || len(r.Failures) != 3 || !reflect.DeepEqual(f.calls, []domain.WindowID{2, 3}) {
		t.Fatalf("result=%+v err=%v calls=%v", r, err, f.calls)
	}
}

func TestOthersPreservesKnownSheetWithoutRequiringItsModalAttribute(t *testing.T) {
	f := othersFixture()
	f.roots = append(f.roots, platform.ActionWindowRole{
		WindowRole:     platform.WindowRole{WindowID: 4, AppID: 10, Role: "AXSheet"},
		ParentWindowID: 1, Reason: "modal attribute unsupported",
	})
	r, err := New(f).PerformOthers("closeOthers", 1, 10)
	if err != nil || r.Succeeded != 2 || len(r.Failures) != 0 || !reflect.DeepEqual(f.calls, []domain.WindowID{2, 3}) {
		t.Fatalf("protected sheet treated as failed target: %+v %v calls=%v", r, err, f.calls)
	}
}

func TestOthersRefusesMissingOrAmbiguousKeptRoot(t *testing.T) {
	for _, change := range []string{"missing", "pid", "unknown", "modal", "duplicate"} {
		t.Run(change, func(t *testing.T) {
			f := othersFixture()
			switch change {
			case "missing":
				f.roots = f.roots[1:]
			case "pid":
				f.roots[0].AppID = 77
			case "unknown":
				f.roots[0].RelationshipsKnown = false
			case "modal":
				f.roots[0].HasAttachedSheet = true
			case "duplicate":
				f.roots = append(f.roots, normalOther(1, 99))
			}
			_, err := New(f).PerformOthers("closeOthers", 1, 10)
			if err == nil || len(f.calls) > 0 {
				t.Fatalf("uncertain keeper allowed action: %v %v", err, f.calls)
			}
		})
	}
}

func TestOthersDeduplicatesTargetsAndRefusesAmbiguousOwners(t *testing.T) {
	f := othersFixture()
	f.roots = append(f.roots, normalOther(2, 10), normalOther(3, 99))
	r, err := New(f).PerformOthers("closeOthers", 1, 10)
	if err != nil || r.Succeeded != 1 || len(r.Failures) != 1 || !reflect.DeepEqual(f.calls, []domain.WindowID{2}) {
		t.Fatalf("result=%+v err=%v calls=%v", r, err, f.calls)
	}
}

func TestOthersRechecksGuardAfterLookupAndBetweenActions(t *testing.T) {
	for _, duringLookup := range []bool{true, false} {
		f := othersFixture()
		retired := false
		if duringLookup {
			f.read = func() { retired = true }
		}
		r, err := New(f).PerformOthersGuarded("closeOthers", 1, 10, func() error {
			if retired || len(f.calls) > 0 {
				return errors.New("drag retired")
			}
			return nil
		})
		want := 1
		if duringLookup {
			want = 0
		}
		if err == nil || r.Succeeded != want || len(f.calls) != want {
			t.Fatalf("retired action dispatched: result=%+v err=%v calls=%v", r, err, f.calls)
		}
	}
}

func TestOthersNeverFallsBackToUnclassifiedWindowActions(t *testing.T) {
	f := fixture()
	_, err := New(f).PerformOthers("closeOthers", 1, 10)
	if err == nil || len(f.CloseCalls) > 0 {
		t.Fatalf("unsupported backend dispatched: %v %v", err, f.CloseCalls)
	}
	for _, kind := range []string{"minimize", "hide", "quit", "closeAll"} {
		backend := othersFixture()
		_, err := New(backend).PerformOthers(kind, 1, 10)
		if err == nil || len(backend.calls) > 0 {
			t.Fatalf("unsupported kind %q dispatched", kind)
		}
	}
}

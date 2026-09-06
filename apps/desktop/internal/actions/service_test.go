package actions

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/platform/fake"
)

type actionFake struct {
	*fake.Fake
	fail domain.WindowID
}

func (f *actionFake) Close(id domain.WindowID) error {
	if id == f.fail {
		return errors.New("denied")
	}
	return f.Fake.Close(id)
}

func fixture() *actionFake {
	f := &actionFake{Fake: fake.New(), fail: 3}
	f.SetWindows([]domain.Window{{ID: 1, AppID: 10}, {ID: 2, AppID: 20}, {ID: 3, AppID: 10}})
	return f
}

func TestBulkReportsIndividualFailure(t *testing.T) {
	f := fixture()
	r, err := New(f).Perform("closeAll", 0, 10)
	if err != nil || r.Succeeded != 1 || len(r.Failures) != 1 || r.Failures[0].WindowID != 3 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
	if !reflect.DeepEqual(f.CloseCalls, []domain.WindowID{1}) {
		t.Fatalf("closed %v", f.CloseCalls)
	}
}

func TestBulkGuardStopsRemainingTargetsWhenPresentationRetires(t *testing.T) {
	f := fake.New()
	f.SetWindows([]domain.Window{{ID: 1, AppID: 10}, {ID: 2, AppID: 10}})
	cancelled := errors.New("presentation retired")
	r, err := New(f).PerformGuarded("closeAll", 0, 10, func() error {
		if len(f.CloseCalls) > 0 {
			return cancelled
		}
		return nil
	})
	if err != nil || r.Succeeded != 1 || len(r.Failures) != 1 || r.Failures[0].WindowID != 2 || r.Failures[0].Error != cancelled.Error() {
		t.Fatalf("partial cancellation result=%+v err=%v", r, err)
	}
	if !reflect.DeepEqual(f.CloseCalls, []domain.WindowID{1}) {
		t.Fatalf("dispatched retired target: %v", f.CloseCalls)
	}
}

func TestWindowIdentityRejectsMismatchAndMissing(t *testing.T) {
	for _, id := range []domain.WindowID{2, 99} {
		f := fixture()
		_, err := New(f).Perform("close", id, 10)
		if err == nil || len(f.CloseCalls) != 0 {
			t.Fatalf("id=%d err=%v calls=%v", id, err, f.CloseCalls)
		}
	}
}

func TestEveryWindowAction(t *testing.T) {
	for _, kind := range []string{"focus", "close", "minimize", "fullscreen"} {
		t.Run(kind, func(t *testing.T) {
			f := fixture()
			r, err := New(f).Perform(kind, 1, 10)
			if err != nil || r.Succeeded != 1 {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestUnsupportedAndInvalidApp(t *testing.T) {
	for _, tc := range []struct {
		k string
		p domain.AppID
	}{{"newWindow", 10}, {"forceQuit", 10}, {"hide", 0}, {"quit", -1}, {"quit", domain.AppID(os.Getpid())}, {"explode", 10}} {
		f := fixture()
		r, err := New(f).Perform(tc.k, 0, tc.p)
		if err == nil || r.Succeeded != 0 || len(f.QuitCalls)+len(f.HideCalls) != 0 {
			t.Fatalf("%+v: %+v %v", tc, r, err)
		}
	}
}

func TestAppActions(t *testing.T) {
	for _, kind := range []string{"hide", "quit", "minimizeAll"} {
		f := fixture()
		r, err := New(f).Perform(kind, 0, 10)
		want := 1
		if kind == "minimizeAll" {
			want = 2
		}
		if err != nil || r.Succeeded != want {
			t.Fatalf("%s %+v %v", kind, r, err)
		}
	}
}

func TestSourceErrorPreventsActions(t *testing.T) {
	f := fixture()
	f.WindowsErr = errors.New("unavailable")
	_, err := New(f).Perform("closeAll", 0, 10)
	if err == nil || len(f.CloseCalls) != 0 {
		t.Fatal("expected enumeration failure")
	}
}

func TestMinimizeAllDoesNotRestoreAlreadyMinimizedWindows(t *testing.T) {
	f := fixture()
	f.SetWindows([]domain.Window{{ID: 1, AppID: 10, Minimized: true}, {ID: 2, AppID: 10}})
	r, err := New(f).Perform("minimizeAll", 0, 10)
	if err != nil || r.Succeeded != 2 || !reflect.DeepEqual(f.MinimizeCalls, []domain.WindowID{2}) {
		t.Fatalf("%+v %v calls=%v", r, err, f.MinimizeCalls)
	}
}

type changingSource struct {
	*actionFake
	reads int
}

func (f *changingSource) Windows() ([]domain.Window, error) {
	f.reads++
	if f.reads > 1 {
		return []domain.Window{{ID: 1, AppID: 20}}, nil
	}
	return []domain.Window{{ID: 1, AppID: 10}}, nil
}

func TestBulkRevalidatesEachOwner(t *testing.T) {
	f := &changingSource{actionFake: fixture()}
	r, err := New(f).Perform("closeAll", 0, 10)
	if err != nil || r.Succeeded != 0 || len(r.Failures) != 1 || len(f.CloseCalls) != 0 {
		t.Fatalf("%+v %v calls=%v", r, err, f.CloseCalls)
	}
}

type optionalActions struct {
	*actionFake
	newApps, forceApps []domain.AppID
}

func (f *optionalActions) NewWindow(app domain.AppID) error {
	f.newApps = append(f.newApps, app)
	return nil
}

func (f *optionalActions) ForceQuitApp(app domain.AppID) error {
	f.forceApps = append(f.forceApps, app)
	return errors.New("refused")
}

func TestOptionalAppCapabilitiesAndNativeErrors(t *testing.T) {
	f := &optionalActions{actionFake: fixture()}
	r, err := New(f).Perform("newWindow", 0, 10)
	if err != nil || r.Succeeded != 1 || !reflect.DeepEqual(f.newApps, []domain.AppID{10}) {
		t.Fatalf("new window %+v %v", r, err)
	}
	r, err = New(f).Perform("forceQuit", 0, 20)
	if err == nil || r.Succeeded != 0 || len(r.Failures) != 1 || !reflect.DeepEqual(f.forceApps, []domain.AppID{20}) {
		t.Fatalf("force quit %+v %v", r, err)
	}
}

type toggleTarget struct {
	*actionFake
	minimized bool
	kinds     []string
}

func (f *toggleTarget) PerformTargetAction(kind string, id domain.WindowID, app domain.AppID) error {
	f.kinds = append(f.kinds, kind)
	if kind == "minimize" {
		f.minimized = !f.minimized
	}
	if kind == "setMinimized" {
		f.minimized = true
	}
	return nil
}

func TestSingleMinimizeTogglesAlreadyMinimizedWindow(t *testing.T) {
	f := &toggleTarget{actionFake: fixture(), minimized: true}
	f.SetWindows([]domain.Window{{ID: 1, AppID: 10, Minimized: true}})
	r, err := New(f).Perform("minimize", 1, 10)
	if err != nil || r.Succeeded != 1 || f.minimized {
		t.Fatalf("single minimize did not toggle: %+v %v minimized=%v", r, err, f.minimized)
	}
}

func TestBulkMinimizeUsesSetterDespiteStaleEnumeration(t *testing.T) {
	f := &toggleTarget{actionFake: fixture(), minimized: true}
	f.SetWindows([]domain.Window{{ID: 1, AppID: 10, Minimized: false}})
	r, err := New(f).Perform("minimizeAll", 0, 10)
	if err != nil || r.Succeeded != 1 || !f.minimized || !reflect.DeepEqual(f.kinds, []string{"setMinimized"}) {
		t.Fatalf("bulk restored window: %+v %v minimized=%v kinds=%v", r, err, f.minimized, f.kinds)
	}
}

type actionableSource struct {
	*actionFake
	actionable []domain.Window
	actionErr  error
}

func (f *actionableSource) ActionWindows(app domain.AppID) ([]domain.Window, error) {
	return f.actionable, f.actionErr
}

func TestBulkUsesActionableRootsWithoutVisibilityFiltering(t *testing.T) {
	f := &actionableSource{actionFake: fixture()}
	f.fail = 0
	// A hidden untitled window, a minimized one, and an off-Space one are all real
	// AX roots; the transient menu surface is not, despite sharing its app PID.
	f.actionable = []domain.Window{{ID: 1, AppID: 10, Hidden: true}, {ID: 2, AppID: 10, Minimized: true}, {ID: 3, AppID: 10, SpaceID: 99}}
	f.SetWindows(append(append([]domain.Window{}, f.actionable...), domain.Window{ID: 4, AppID: 10, Title: "Transient menu surface"}))
	r, err := New(f).Perform("closeAll", 0, 10)
	if err != nil || r.Succeeded != 3 || len(r.Failures) != 0 || !reflect.DeepEqual(f.CloseCalls, []domain.WindowID{1, 2, 3}) {
		t.Fatalf("roots lost or menu targeted: %+v %v calls=%v", r, err, f.CloseCalls)
	}
}

func TestBulkActionableEnumerationFailureStopsDispatch(t *testing.T) {
	f := &actionableSource{actionFake: fixture(), actionErr: errors.New("AX enumeration refused")}
	r, err := New(f).Perform("closeAll", 0, 10)
	if err == nil || r.Succeeded != 0 || len(f.CloseCalls) != 0 {
		t.Fatalf("acted after failed AX enumeration: %+v %v calls=%v", r, err, f.CloseCalls)
	}
}

func TestBulkRetainsKnownActionsAndReportsIncompleteEnumeration(t *testing.T) {
	f := &actionableSource{actionFake: fixture(), actionable: []domain.Window{{ID: 1, AppID: 10}}, actionErr: errors.New("native AX classification timed out")}
	r, err := New(f).Perform("closeAll", 0, 10)
	if err != nil || r.Succeeded != 1 || len(r.Failures) != 1 || r.Failures[0].WindowID != 0 || !reflect.DeepEqual(f.CloseCalls, []domain.WindowID{1}) {
		t.Fatalf("partial enumeration was hidden or known targets lost: %+v %v calls=%v", r, err, f.CloseCalls)
	}
}

//go:build darwin

package platform

import (
	"os"
	"testing"

	"option-tab/internal/domain"
)

func TestExplicitNativeActionsRejectInvalidIdentity(t *testing.T) {
	p := &darwinPlatform{}
	for _, pid := range []domain.AppID{0, -1, domain.AppID(os.Getpid()), 1 << 40} {
		for _, kind := range []string{"focus", "close", "minimize", "fullscreen", "hide", "quit", "forceQuit", "newWindow"} {
			if err := p.PerformTargetAction(kind, 1, pid); err == nil {
				t.Fatalf("accepted %s for pid %d", kind, pid)
			}
		}
	}
	if err := p.PerformTargetAction("close", 1<<40, 10); err == nil {
		t.Fatal("accepted truncated window identity")
	}
}

func TestExplicitFocusRequiresAcceptedAXRaise(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		prepared := false
		err := performNativeWindowAction("focus", 1, 10, nativeWindowOps{
			prepare: func(id domain.WindowID, app domain.AppID) { prepared = true },
			raise: func(id domain.WindowID, app domain.AppID) bool {
				if !prepared {
					t.Fatal("AX raise before Space preparation")
				}
				return accepted
			},
		})
		if (err == nil) != accepted {
			t.Fatalf("AX accepted=%v err=%v", accepted, err)
		}
	}
}

func TestNativeBulkMinimizeDispatchesSetterAndPropagatesRefusal(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		minimized := true
		calls := 0
		err := performNativeWindowAction("setMinimized", 1, 10, nativeWindowOps{
			toggleMinimize: func(domain.WindowID, domain.AppID) bool { minimized = !minimized; return true },
			setMinimized:   func(domain.WindowID, domain.AppID) bool { calls++; minimized = true; return accepted },
		})
		if !minimized || calls != 1 || (err == nil) != accepted {
			t.Fatalf("minimized=%v setter calls=%d accepted=%v err=%v", minimized, calls, accepted, err)
		}
	}
}

func TestNativeActionSnapshotDoesNotHideClassificationFailure(t *testing.T) {
	for _, failure := range []string{"role lookup refused", "remote lookup timed out"} {
		t.Run(failure, func(t *testing.T) {
			probe := false
			windows, err := snapshotActionWindows(10, nativeActionSourceOps{
				available: func() bool { probe = true; return true },
				windows: func() ([]domain.Window, error) {
					return []domain.Window{{ID: 1, AppID: 10}, {ID: 2, AppID: 10}, {ID: 3, AppID: 10}}, nil
				},
				classify: func(id domain.WindowID, app domain.AppID) int {
					if !probe {
						t.Fatal("classification before availability")
					}
					if id == 1 {
						return 1
					}
					if id == 2 {
						return 0
					}
					if failure == "role lookup refused" {
						return -2
					}
					return -1
				},
			})
			if err == nil || len(windows) != 1 || windows[0].ID != 1 {
				t.Fatalf("native uncertainty silently dropped: windows=%+v err=%v", windows, err)
			}
		})
	}
}

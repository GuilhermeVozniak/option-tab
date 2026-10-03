//go:build darwin

package platform

import (
	"context"
	"os"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

func TestNativeDockLockCallbackAdmission(t *testing.T) {
	if got := nativeDockLockProbe(); got != 31 {
		t.Fatalf("callback invariant bits %b", got)
	}
}

func TestNativeDockLockReadOnlySnapshot(t *testing.T) {
	if os.Getenv("OPTION_TAB_DOCK_LOCK_READONLY") != "1" {
		t.Skip("opt-in read-only desktop inventory")
	}
	raw, err := nativeDockLockSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("native Dock inventory: %+v", raw)
	edge := raw.Edge
	if edge == "" && raw.Reason == "" {
		edge = dockLockInferredEdge(raw.Displays, raw.Container)
	}
	t.Logf("positive container authority: edge=%s actualUUID=%s", edge, dockLockActual(raw.Displays, edge, raw.Container))
	if len(raw.Displays) == 0 {
		t.Fatal("display inventory empty")
	}
}

func TestNativeDockLockPlacementOwnership(t *testing.T) {
	if got := nativeDockLockPlacementProbe(); got != 31 {
		t.Fatalf("placement ownership bits %b", got)
	}
}

func lockTestPolicy() DockMonitorLockPolicy {
	return DockMonitorLockPolicy{Session: 1, Revision: 1, Target: "main", Bypass: hotkey.ModSet(0).With(hotkey.ModOption)}
}

func lockTestRaw(moved bool) dockLockRaw {
	container := domain.Bounds{X: 120, Y: 90, W: 60, H: 20}
	if moved {
		container.X = 20
	}
	return dockLockRaw{Generation: nativeDockLockGeneration(), PID: 99, Edge: "bottom", Container: container, Displays: []DockLockDisplay{{UUID: "one", Main: true, Bounds: domain.Bounds{W: 100, H: 100}}, {UUID: "two", Bounds: domain.Bounds{X: 100, W: 100, H: 100}}}}
}

func TestNativeDockLockJoinedPlacementLifecycle(t *testing.T) {
	for _, mode := range []string{"verified", "cancel", "timeout", "topology", "preparation-cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			states := make(chan DockMonitorLockState, 16)
			done := make(chan error, 1)
			requestCtx, requestCancel := context.WithCancel(context.Background())
			defer requestCancel()
			reads := 0
			go func() {
				done <- runDockMonitorLock(ctx, lockTestPolicy(), func(s DockMonitorLockState) {
					select {
					case states <- s:
					default:
					}
				}, true, func(moves int) (dockLockRaw, error) {
					reads++
					raw := lockTestRaw(mode == "verified" && moves >= 3)
					if mode == "topology" && moves > 0 {
						raw.Displays[0].Bounds.W = 90
					}
					if mode == "preparation-cancel" && reads == 2 {
						requestCancel()
					}
					return raw, nil
				})
			}()
			var initial DockMonitorLockState
			select {
			case initial = <-states:
			case <-time.After(time.Second):
				t.Fatal("owner did not observe")
			}
			// A second native observation must refuse before acquiring any resource.
			if err := runDockMonitorLock(ctx, lockTestPolicy(), func(DockMonitorLockState) {}, true, nil); err == nil {
				t.Fatal("second owner admitted")
			}
			reply := make(chan lockPlaceReply, 1)
			go func() {
				r, e := placeDockOnOwner(requestCtx, DockPlacementRequest{Session: 1, Revision: 1, Generation: initial.Generation, RequestID: 1})
				reply <- lockPlaceReply{r, e}
			}()
			if mode == "cancel" {
				select {
				case <-time.After(120 * time.Millisecond):
					requestCancel()
				case r := <-reply:
					t.Fatalf("completed before cancellation %+v", r)
				}
			}
			select {
			case r := <-reply:
				if mode == "verified" {
					if r.err != nil || !r.result.Verified || !r.result.CursorRestored || r.result.ActualUUID != "one" {
						t.Fatalf("bad verification %+v", r)
					}
				} else if r.err == nil || r.result.Verified {
					t.Fatalf("invalid placement succeeded %+v", r)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("placement did not finish boundedly")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("native owner did not join")
			}
			dockLockActive.Lock()
			live := dockLockActive.owner != nil
			dockLockActive.Unlock()
			if live {
				t.Fatal("native owner retained after return")
			}
		})
	}
}

func TestNativeDockLockQueuedCarrierAdmission(t *testing.T) {
	done := make(chan uint64, 1)
	go func() { done <- nativeDockLockDeliveryProbe() }()
	var got uint64
	select {
	case got = <-done:
	case <-time.After(time.Second):
		t.Fatal("isolated callback owner did not finish cleanup")
	}
	for _, check := range []struct {
		bit  uint64
		name string
	}{
		{1, "current tagged carrier converts to movement"},
		{2, "queued carrier stays inert after unmarked PID0 input"},
		{4, "cancelled request stays inert"},
		{8, "cancellation permits restoration while ownership remains current"},
		{16, "old generation stays inert"},
		{32, "unmarked PID0 callback retires cursor ownership"},
		{64, "unmarked PID0 input refuses new movement and restoration"},
		{128, "unmarked nonzero-PID input preserves cursor ownership"},
	} {
		if got&check.bit == 0 {
			t.Error(check.name)
		}
	}
}

func TestNativeDockLockInertTransport(t *testing.T) {
	if os.Getenv("OPTION_TAB_DOCK_LOCK_NULL_TRANSPORT") != "1" {
		t.Skip("opt-in inert carrier transport only")
	}
	if err := nativeDockLockNullTransport(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeDockLockPlacementUnavailable(t *testing.T) {
	p := &darwinPlatform{}
	if capability, ok := any(p).(interface{ DockPlacementAvailable() bool }); !ok || capability.DockPlacementAvailable() {
		t.Fatal("unproven placement must report unavailable")
	}
	result, err := p.PlaceDock(context.Background(), DockPlacementRequest{RequestID: 73})
	if err == nil || result.Status != "unavailable" || result.RequestID != 73 || result.Verified {
		t.Fatalf("unproven placement accepted: %+v %v", result, err)
	}
}

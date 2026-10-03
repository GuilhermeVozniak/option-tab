package platform

import (
	"testing"

	"option-tab/internal/domain"
)

func TestLauncherFocusUnknownPreservesTopology(t *testing.T) {
	for _, bad := range []LauncherEnvironment{
		{FocusKnown: true, FocusedProcess: ProcessIdentity{PID: 12, StartSeconds: 8}, FocusedBundleID: "bad bundle"},
		{FocusKnown: true, FocusedProcess: ProcessIdentity{PID: 12, StartSeconds: 8, StartMicros: 1000000}, FocusedBundleID: "com.test"},
		{FocusKnown: true, FocusedProcess: ProcessIdentity{PID: 12}, FocusedBundleID: "com.test"},
		{FocusKnown: false, FocusedProcess: ProcessIdentity{PID: 12, StartSeconds: 8}, FocusedBundleID: "com.test"},
	} {
		bad.Complete = true
		bad.Status = "ready"
		bad.Displays = []LauncherDisplay{{UUID: "display", Frame: domain.Bounds{W: 100, H: 100}}}
		got := normalizeLauncherFocus(bad)
		if got.FocusKnown || got.FocusedBundleID != "" || got.FocusedProcess != (ProcessIdentity{}) {
			t.Fatal("invalid focus retained")
		}
		if !got.Complete || got.Status != "ready" || len(got.Displays) != 1 {
			t.Fatal("unknown focus discarded topology")
		}
	}
	good := LauncherEnvironment{FocusKnown: true, FocusedProcess: ProcessIdentity{PID: 12, StartSeconds: 8, StartMicros: 1}, FocusedBundleID: "com.test.App"}
	if got := normalizeLauncherFocus(good); got.FocusedBundleID != good.FocusedBundleID || !got.FocusKnown {
		t.Fatal("exact focus lost")
	}
}

func TestLauncherFocusMalformedPacketDoesNotLoseDisplays(t *testing.T) {
	base := LauncherEnvironment{Complete: true, Status: "ready", Displays: []LauncherDisplay{{UUID: "one"}}, FocusKnown: true, FocusedProcess: ProcessIdentity{PID: 3, StartSeconds: 4}, FocusedBundleID: "old"}
	for _, raw := range []string{`{"Known":"true","Process":{"PID":3}}`, `{"Known":true,"Process":{"PID":3,"StartSeconds":-1},"BundleID":"com.app"}`, `{}`} {
		got := mapLauncherFocus(base, []byte(raw))
		if !got.Complete || got.Status != "ready" || len(got.Displays) != 1 || got.FocusKnown || got.FocusedBundleID != "" {
			t.Fatalf("invalid independent focus corrupted snapshot: %+v", got)
		}
	}
	got := mapLauncherFocus(base, []byte(`{"Known":true,"Process":{"PID":3,"StartSeconds":4,"StartMicros":2},"BundleID":"com.App.Exact"}`))
	if !got.FocusKnown || got.FocusedBundleID != "com.App.Exact" || got.FocusedProcess.StartMicros != 2 {
		t.Fatalf("exact focus mapping: %+v", got)
	}
}

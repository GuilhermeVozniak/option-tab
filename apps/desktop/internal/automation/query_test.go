package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type activeSource struct {
	calls    int
	window   domain.Window
	identity platform.AutomationWindowIdentity
}

func (a *activeSource) ActiveWindow(context.Context) (domain.Window, platform.AutomationWindowIdentity, error) {
	a.calls++
	return a.window, a.identity, nil
}

func TestActiveActionCapturesOnceAndKeepsExactIdentity(t *testing.T) {
	d, ids, p := fixture()
	active := &activeSource{window: domain.Window{ID: 9, AppID: 7}, identity: ids.window}
	d.Active = active
	d.Windows = func(context.Context) ([]domain.Window, error) { panic("active action must not substitute inventory") }
	p.before = func() { active.window.ID = 10 }
	r := request(platform.AutomationWindowAction)
	r.ActiveWindow = true
	r.Action = "focus"
	got := New(d).Handle(context.Background(), r)
	if got.ErrorCode != "" || active.calls != 1 || p.calls != 1 {
		t.Fatalf("active retargeted: %+v %d %d", got, active.calls, p.calls)
	}
}

func TestValidationRejectsConflictsUnknownFieldsAndNonfinite(t *testing.T) {
	base := request(platform.AutomationWindowAction)
	base.WindowID = 9
	base.Action = "close"
	cases := []platform.AutomationRequest{base, base, base, base, request(platform.AutomationShowPreviews), request(platform.AutomationQueryApps), request(platform.AutomationHidePreviews)}
	cases[0].ActiveWindow = true
	cases[1].Action = "quit"
	v := true
	cases[2].Fullscreen = &v
	cases[3].App = &platform.AutomationAppSelector{Name: "x", PID: 7}
	cases[4].App = &platform.AutomationAppSelector{PID: 7}
	cases[4].Position = &platform.AutomationPoint{X: math.NaN()}
	cases[5].IncludeImages = true
	cases[6].PresentationToken = "0"
	for _, r := range cases {
		if got := New(Deps{}).Handle(context.Background(), r); got.ErrorCode != "invalidArgument" {
			t.Fatalf("accepted invalid request %+v: %+v", r, got)
		}
	}
}

func TestDeadlineAndAppAdmissionRetireFinalDispatch(t *testing.T) {
	d, _, p := fixture()
	calls := 0
	d.Admission = func(context.Context, platform.AutomationOperation) error {
		calls++
		if calls >= 4 {
			return &Error{Code: "retired", Message: "suspended"}
		}
		return nil
	}
	r := request(platform.AutomationWindowAction)
	r.WindowID = 9
	r.Action = "close"
	got := New(d).Handle(context.Background(), r)
	if got.ErrorCode != "retired" || p.calls != 0 {
		t.Fatal(got)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if got = New(d).Handle(ctx, r); got.ErrorCode != "timeout" {
		t.Fatal(got)
	}
}

type manyIdentities struct{ *identities }

func (i manyIdentities) WindowIdentity(id domain.WindowID) (platform.AutomationWindowIdentity, error) {
	return platform.AutomationWindowIdentity{ID: id, Process: i.process}, nil
}

func TestCachedImagesAreExplicitExactFreshBoundedCopies(t *testing.T) {
	d, ids, _ := fixture()
	d.Identities = manyIdentities{ids}
	now := time.Unix(100, 0)
	d.Now = func() time.Time { return now }
	windows := make([]domain.Window, 12)
	for i := range windows {
		windows[i] = domain.Window{ID: domain.WindowID(i + 1), AppID: 7}
	}
	d.Windows = func(context.Context) ([]domain.Window, error) { return windows, nil }
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	calls := 0
	d.CachedFrames = func(context.Context) ([]CachedFrame, error) {
		calls++
		out := make([]CachedFrame, len(windows))
		for i, w := range windows {
			out[i] = CachedFrame{Window: platform.AutomationWindowIdentity{ID: w.ID, Process: ids.process}, CapturedAt: now, PNG: append([]byte(nil), buf.Bytes()...)}
		}
		out[0].CapturedAt = now.Add(-31 * time.Second)
		out[1].Window.Process.StartSeconds++
		out[2].CapturedAt = now.Add(time.Second)
		return out, nil
	}
	r := request(platform.AutomationQueryWindows)
	if got := New(d).Handle(context.Background(), r); got.ErrorCode != "" || calls != 0 {
		t.Fatal("default read touched cache", got)
	}
	r.IncludeImages = true
	got := New(d).Handle(context.Background(), r)
	if got.ErrorCode != "" {
		t.Fatal(got)
	}
	var out envelope
	if e := json.Unmarshal(got.JSON, &out); e != nil {
		t.Fatal(e)
	}
	cached := 0
	for i, w := range *out.Windows {
		if i < 3 && w.ImageStatus != "stale" {
			t.Fatal(w)
		}
		if w.ImageStatus == "cached" {
			cached++
		}
	}
	if calls != 1 || cached != 8 || out.ImagesOmitted != 1 || len(got.JSON) > MaxReplyBytes {
		t.Fatalf("image limits %d %+v", calls, out)
	}
}

func TestJSONByteBoundAndUnicodeEscaping(t *testing.T) {
	d, _, _ := fixture()
	d.Apps = func(context.Context) ([]domain.App, error) {
		apps := make([]domain.App, 500)
		for i := range apps {
			apps[i] = domain.App{ID: 7, Name: strings.Repeat("<é", 15000), BundleID: "fixture"}
		}
		return apps, nil
	}
	got := New(d).Handle(context.Background(), request(platform.AutomationQueryApps))
	if got.ErrorCode != "" || len(got.JSON) > MaxReplyBytes || !json.Valid(got.JSON) {
		t.Fatal(got.ErrorCode, len(got.JSON))
	}
	var out envelope
	if err := json.Unmarshal(got.JSON, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Truncated || out.Omitted == 0 || len(*out.Apps)+out.Omitted != 500 {
		t.Fatal("silent truncation")
	}
}

func TestFreshIdentityFailureAfterInventoryRefusesAction(t *testing.T) {
	d, ids, p := fixture()
	ids.window.Process.PID = 8
	r := request(platform.AutomationWindowAction)
	r.WindowID = 9
	r.Action = "close"
	got := New(d).Handle(context.Background(), r)
	if got.ErrorCode != "staleIdentity" || p.calls != 0 {
		t.Fatal(got)
	}
}

func TestImageEncodedByteLimitAndCacheFailureAreExplicit(t *testing.T) {
	d, ids, _ := fixture()
	d.Now = func() time.Time { return time.Unix(100, 0) }
	d.CachedFrames = func(context.Context) ([]CachedFrame, error) {
		return []CachedFrame{{Window: ids.window, CapturedAt: time.Unix(100, 0), PNG: make([]byte, MaxImageBytes)}}, nil
	}
	r := request(platform.AutomationQueryWindows)
	r.IncludeImages = true
	got := New(d).Handle(context.Background(), r)
	var out envelope
	if e := json.Unmarshal(got.JSON, &out); e != nil {
		t.Fatal(got, e)
	}
	if (*out.Windows)[0].ImageStatus != "omitted" || (*out.Windows)[0].Image != "" || out.ImagesOmitted != 1 || !out.Truncated {
		t.Fatal("oversize image not omitted explicitly")
	}
	d.CachedFrames = func(context.Context) ([]CachedFrame, error) {
		return nil, &Error{Code: "permissionDenied", Message: "no cache access"}
	}
	got = New(d).Handle(context.Background(), r)
	if e := json.Unmarshal(got.JSON, &out); e != nil {
		t.Fatal(e)
	}
	if (*out.Windows)[0].ImageStatus != "unavailable" || (*out.Windows)[0].Image != "" {
		t.Fatal("cache refusal disguised")
	}
}

func TestRequestDeadlineBoundIsPassedToDependencies(t *testing.T) {
	for _, duration := range []time.Duration{0, time.Hour, time.Second} {
		d, _, _ := fixture()
		d.OpenSwitcher = func(ctx context.Context, _ string, g func() error) (Presentation, error) {
			deadline, ok := ctx.Deadline()
			limit := 5 * time.Second
			if duration != 0 {
				limit = min(10*time.Second, duration)
			}
			if !ok || time.Until(deadline) > limit {
				t.Fatal("unbounded request")
			}
			return Presentation{Status: "accepted"}, g()
		}
		ctx := context.Background()
		cancel := func() {}
		if duration != 0 {
			ctx, cancel = context.WithTimeout(ctx, duration)
		}
		got := New(d).Handle(ctx, request(platform.AutomationOpenSwitcher))
		cancel()
		if got.ErrorCode != "" {
			t.Fatal(got)
		}
	}
}

func TestNativeAutomationErrorsKeepStableCodes(t *testing.T) {
	for _, code := range []string{"permissionDenied", "staleIdentity", "unsupported", "invalid-native-code"} {
		d, _, _ := fixture()
		d.Apps = func(context.Context) ([]domain.App, error) {
			return nil, &platform.AutomationNativeError{Code: code, Message: "native refusal"}
		}
		expected := code
		if code == "invalid-native-code" {
			expected = "internal"
		}
		if got := New(d).Handle(context.Background(), request(platform.AutomationQueryApps)); got.ErrorCode != expected {
			t.Fatalf("native code lost: %+v", got)
		}
	}
}

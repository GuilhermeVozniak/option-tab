package widgetproviders

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"option-tab/internal/platform"
	"option-tab/internal/widgets"
)

type batteryFake struct {
	calls  int
	sample platform.BatterySnapshot
	after  func()
}

func (f *batteryFake) ObserveBattery(_ context.Context, emit func(platform.BatterySnapshot)) error {
	f.calls++
	if f.after != nil {
		f.after()
	}
	emit(f.sample)
	return nil
}

type networkFake struct {
	calls  int
	usage  bool
	sample platform.NetworkSnapshot
}

func (f *networkFake) ObserveNetwork(_ context.Context, usage bool, emit func(platform.NetworkSnapshot)) error {
	f.calls++
	f.usage = usage
	emit(f.sample)
	return nil
}

type audioFake struct {
	calls, selections int
	sample            platform.AudioOutputSnapshot
	generation        uint64
	uid               string
}

func (f *audioFake) ObserveAudioOutputs(_ context.Context, emit func(platform.AudioOutputSnapshot)) error {
	f.calls++
	emit(f.sample)
	return nil
}

func (f *audioFake) SelectAudioOutput(_ context.Context, generation uint64, uid string, guard func() error) error {
	if err := guard(); err != nil {
		return err
	}
	f.selections++
	f.generation, f.uid = generation, uid
	return nil
}

func TestBatteryCopiesOptionalValuesAndDropsCancelledRead(t *testing.T) {
	charge, charging := .72, false
	source := &batteryFake{sample: platform.BatterySnapshot{Generation: 7, Sequence: 2, ObservedAt: time.Unix(100, 0), Status: "ready", Charge: &charge, Charging: &charging, PowerSource: "external"}}
	provider := Battery{Source: source}
	var got widgets.Sample
	if err := provider.Observe(context.Background(), []string{"battery.read"}, func(s widgets.Sample) { got = s }); err != nil {
		t.Fatal(err)
	}
	charge, charging = 0, true
	if got.Generation != 7 || got.Sequence != 2 || *got.Fields["charge"].Number != .72 || *got.Fields["charging"].Boolean || *got.Fields["powerSource"].Text != "external" {
		t.Fatalf("bad copied sample: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	source.after = cancel
	deliveries := 0
	_ = provider.Observe(ctx, []string{"battery.read"}, func(widgets.Sample) { deliveries++ })
	if deliveries != 0 {
		t.Fatal("cancelled native read reached runtime")
	}
}

func TestAbsentBatteryDoesNotInventZeroCharge(t *testing.T) {
	source := &batteryFake{sample: platform.BatterySnapshot{Generation: 1, Sequence: 1, Status: "absent", Reason: "noBattery"}}
	var got widgets.Sample
	if err := (Battery{Source: source}).Observe(context.Background(), []string{"battery.read"}, func(s widgets.Sample) { got = s }); err != nil {
		t.Fatal(err)
	}
	if got.Status != "absent" || len(got.Fields) != 0 {
		t.Fatalf("absent battery produced fields: %+v", got)
	}
}

func TestNetworkUsageRequiresSeparateGrant(t *testing.T) {
	connected, upload, download := true, 11.0, 22.0
	source := &networkFake{sample: platform.NetworkSnapshot{Generation: 3, Sequence: 1, Status: "ready", Connected: &connected, Category: "wifi", UploadRate: &upload, DownloadRate: &download}}
	provider := Network{Source: source}
	for _, test := range []struct {
		caps  []string
		usage bool
		keys  []string
	}{
		{[]string{"network.status.read"}, false, []string{"connected", "category"}},
		{[]string{"network.usage.read"}, true, []string{"uploadRate", "downloadRate"}},
		{[]string{"network.status.read", "network.usage.read"}, true, []string{"connected", "category", "uploadRate", "downloadRate"}},
	} {
		var got widgets.Sample
		if err := provider.Observe(context.Background(), test.caps, func(s widgets.Sample) { got = s }); err != nil {
			t.Fatal(err)
		}
		if source.usage != test.usage || len(got.Fields) != len(test.keys) {
			t.Fatalf("grant leakage: usage=%v fields=%v", source.usage, got.Fields)
		}
		for _, key := range test.keys {
			if _, ok := got.Fields[key]; !ok {
				t.Fatalf("missing %s", key)
			}
		}
	}
}

func TestAudioControlOnlyHasChoicesWithoutStatusAndSelectsExactGuardedUID(t *testing.T) {
	volume, muted := .8, false
	source := &audioFake{sample: platform.AudioOutputSnapshot{Generation: 17, Sequence: 2, Status: "ready", DefaultUID: "current", Volume: &volume, Muted: &muted, Devices: []platform.AudioOutputDevice{
		{UID: "current", Name: "Same name", Alive: true, OutputChannels: 2},
		{UID: "other", Name: "Same name", Alive: true, OutputChannels: 2},
		{UID: "dead", Name: "Disconnected", Alive: false, OutputChannels: 2},
		{UID: "input", Name: "Input", Alive: true},
	}}}
	provider := Audio{Source: source}
	var got widgets.Sample
	if err := provider.Observe(context.Background(), []string{"audio.output.select"}, func(s widgets.Sample) { got = s }); err != nil {
		t.Fatal(err)
	}
	want := []widgets.ProviderOption{{ID: "current", Label: "Same name"}, {ID: "other", Label: "Same name"}}
	if len(got.Fields) != 0 || !reflect.DeepEqual(got.Actions["selectOutput"].Options, want) {
		t.Fatalf("control-only leaked status or lost identity: %+v", got)
	}
	retired := errors.New("retired fixture")
	action := widgets.ProviderAction{Generation: 17, Action: "selectOutput", OptionID: "other"}
	if err := provider.Perform(context.Background(), action, func() error { return retired }); !errors.Is(err, retired) || source.selections != 0 {
		t.Fatalf("revoked action entered: %v", err)
	}
	if err := provider.Perform(context.Background(), action, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if source.selections != 1 || source.generation != 17 || source.uid != "other" {
		t.Fatalf("wrong native target: %+v", source)
	}
	if err := provider.Observe(context.Background(), []string{"audio.status.read"}, func(s widgets.Sample) { got = s }); err != nil {
		t.Fatal(err)
	}
	if len(got.Actions) != 0 || *got.Fields["outputName"].Text != "Same name" || *got.Fields["volume"].Number != .8 {
		t.Fatalf("read-only produced action or missing status: %+v", got)
	}
}

func TestLocalProvidersRejectUngrantableRequestsWithoutSourceWork(t *testing.T) {
	b, n, a := &batteryFake{}, &networkFake{}, &audioFake{}
	for _, p := range []widgets.Provider{Battery{Source: b}, Network{Source: n}, Audio{Source: a}} {
		for _, caps := range [][]string{nil, {"network.fetch"}, {"clock.read"}} {
			if err := p.Observe(context.Background(), caps, func(widgets.Sample) {}); !errors.Is(err, widgets.ErrInvalid) {
				t.Fatalf("accepted invalid caps %v: %v", caps, err)
			}
		}
	}
	if b.calls+n.calls+a.calls != 0 {
		t.Fatal("ungranted source work")
	}
	provider := Audio{Source: a}
	for _, command := range []widgets.ProviderAction{{Generation: 1, Action: "open", OptionID: "x"}, {Action: "selectOutput", OptionID: "x"}, {Generation: 1, Action: "selectOutput"}} {
		if err := provider.Perform(context.Background(), command, func() error { return nil }); !errors.Is(err, widgets.ErrInvalid) {
			t.Fatalf("accepted invalid action: %v", err)
		}
	}
	if a.selections != 0 {
		t.Fatal("invalid action reached device")
	}
}

func TestAudioLongDisplayNameKeepsChooserWithinRuntimeBounds(t *testing.T) {
	uid := "exact-native-uid"
	source := &audioFake{sample: platform.AudioOutputSnapshot{Generation: 1, Sequence: 1, Status: "ready", DefaultUID: uid, Devices: []platform.AudioOutputDevice{{UID: uid, Name: strings.Repeat("音", 300), Alive: true, OutputChannels: 2}}}}
	var got widgets.Sample
	err := (Audio{Source: source}).Observe(context.Background(), []string{"audio.status.read", "audio.output.select"}, func(s widgets.Sample) { got = s })
	if err != nil {
		t.Fatal(err)
	}
	options := got.Actions["selectOutput"].Options
	if len(options) != 1 || options[0].ID != uid || len([]rune(options[0].Label)) != 160 {
		t.Fatalf("chooser label exceeded runtime limit or native identity changed: %+v", options)
	}
	if len([]rune(*got.Fields["outputName"].Text)) != 300 {
		t.Fatal("shorter option bound incorrectly truncated ordinary display field")
	}
}

package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestWidgetPackageCancellationRetainsSlotAndDropsLateBytes(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	source := newWidgetPackageSource(func(context.Context) (WidgetPackageFile, error) {
		calls.Add(1)
		close(started)
		<-release
		return WidgetPackageFile{Name: "test.zip", Archive: []byte("late")}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := source.ChooseWidgetPackage(ctx); done <- err }()
	<-started
	cancel()
	if _, err := source.ChooseWidgetPackage(context.Background()); err == nil {
		t.Fatal("cancelled chooser released busy slot before native join")
	}
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("late result: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("second native chooser started")
	}
}

func TestWidgetPackageCopiesSnapshotAndRejectsInvalidAdmission(t *testing.T) {
	bytes := []byte("archive")
	calls := 0
	source := newWidgetPackageSource(func(context.Context) (WidgetPackageFile, error) {
		calls++
		return WidgetPackageFile{Name: "local.otwidget", Archive: bytes}, nil
	})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, cancelled} {
		if _, err := source.ChooseWidgetPackage(ctx); err == nil {
			t.Fatal("invalid context admitted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid context opened chooser")
	}
	result, err := source.ChooseWidgetPackage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bytes[0] = 'X'
	if string(result.Archive) != "archive" {
		t.Fatal("archive aliases native result")
	}
}

func TestWidgetPackageBoundsAndBasename(t *testing.T) {
	for _, file := range []WidgetPackageFile{{Name: "/tmp/test.zip", Archive: []byte("x")}, {Name: "test.app", Archive: []byte("x")}, {Name: "test.zip", Archive: make([]byte, 4*1024*1024+1)}} {
		source := newWidgetPackageSource(func(context.Context) (WidgetPackageFile, error) { return file, nil })
		if _, err := source.ChooseWidgetPackage(context.Background()); err == nil {
			t.Fatal("invalid file snapshot admitted")
		}
	}
}

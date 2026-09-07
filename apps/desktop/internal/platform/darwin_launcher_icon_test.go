//go:build darwin

package platform

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLauncherIconPrivateCopyAndJoinedCancel(t *testing.T) {
	root := filepath.Join(t.TempDir(), "icons")
	port, err := NewLauncherIconSource(root)
	if err != nil {
		t.Fatal(err)
	}
	s := port.(*darwinLauncherIconSource)
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewNRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	s.choose = func(context.Context, string) (launcherNativeRecord, error) {
		return launcherNativeRecord{Archive: pngBytes.Bytes()}, nil
	}
	id, err := s.ChooseLauncherIcon(context.Background())
	if err != nil || !launcherIconID(id) {
		t.Fatal(id, err)
	}
	got, err := s.LauncherIcon(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	got[0] = 0
	again, err := s.LauncherIcon(context.Background(), id)
	if err != nil || again[0] == 0 {
		t.Fatal("copied icon", err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	s.choose = func(ctx context.Context, _ string) (launcherNativeRecord, error) {
		close(started)
		<-ctx.Done()
		<-release
		return launcherNativeRecord{Archive: pngBytes.Bytes()}, nil
	}
	selected := make(chan error, 1)
	go func() { _, err := s.ChooseLauncherIcon(context.Background()); selected <- err }()
	<-started
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case <-closed:
		t.Fatal("close returned before chooser drain")
	default:
	}
	close(release)
	if err = <-selected; err == nil {
		t.Fatal("late selection accepted")
	}
	if err = <-closed; err != nil {
		t.Fatal(err)
	}
	reopened, err := NewLauncherIconSource(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	if _, err = reopened.LauncherIcon(context.Background(), id); err != nil {
		t.Fatal("restart", err)
	}
	if err = reopened.RemoveLauncherIcon(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err = reopened.RemoveLauncherIcon(context.Background(), id); err != nil {
		t.Fatal("idempotent", err)
	}
	if _, err = os.Stat(filepath.Join(root, id)); !os.IsNotExist(err) {
		t.Fatal("not removed", err)
	}
}

func TestLauncherReferenceRemoveMissingIsIdempotent(t *testing.T) {
	s, err := NewLauncherReferenceSource(filepath.Join(t.TempDir(), "references"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.RemoveLauncherReference(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if err = s.RemoveLauncherReference(context.Background(), "../other"); err == nil {
		t.Fatal("invalid identifier accepted")
	}
}

//go:build darwin

package platform

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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

func TestLauncherIconPublishToleratesOwnLeftoversOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "icons")
	port, err := NewLauncherIconSource(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = port.Close() }()
	s := port.(*darwinLauncherIconSource)
	size := 16
	s.choose = func(context.Context, string) (launcherNativeRecord, error) {
		// Every selection is new content, so each one reaches the directory scan.
		size++
		var pngBytes bytes.Buffer
		if err := png.Encode(&pngBytes, image.NewNRGBA(image.Rect(0, 0, size, size))); err != nil {
			return launcherNativeRecord{}, err
		}
		return launcherNativeRecord{Archive: pngBytes.Bytes()}, nil
	}
	// A crash between create and rename leaves the staged temp behind, and
	// Finder drops its metadata file into any directory it displays.
	stale := filepath.Join(root, ".icon-"+strings.Repeat("ab", 16)+".tmp")
	metadata := filepath.Join(root, ".DS_Store")
	for _, name := range []string{stale, metadata} {
		if err = os.WriteFile(name, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	id, err := s.ChooseLauncherIcon(context.Background())
	if err != nil || !launcherIconID(id) {
		t.Fatalf("leftovers blocked icon selection: %q %v", id, err)
	}
	if _, err = os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp not removed: %v", err)
	}
	if _, err = os.Lstat(metadata); err != nil {
		t.Fatalf("foreign metadata must be left alone: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err = os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, plant := range map[string]func(string) error{
		".icon-" + strings.Repeat("cd", 16) + ".tmp": func(p string) error { return os.Symlink(outside, p) },
		".icon-" + strings.Repeat("ef", 16) + ".tmp": func(p string) error { return os.Mkdir(p, 0o700) },
		".DS_Store":       func(p string) error { _ = os.Remove(p); return os.Symlink(outside, p) },
		".icon-short.tmp": func(p string) error { return os.WriteFile(p, nil, 0o600) },
		"notes.txt":       func(p string) error { return os.WriteFile(p, nil, 0o600) },
	} {
		path := filepath.Join(root, name)
		if err = plant(path); err != nil {
			t.Fatal(err)
		}
		var refused *LauncherItemError
		if _, err = s.ChooseLauncherIcon(context.Background()); !errors.As(err, &refused) || refused.Code != "unsafePath" {
			t.Fatalf("%s accepted: %v", name, err)
		}
		if _, err = os.Lstat(path); err != nil {
			t.Fatalf("%s was touched: %v", name, err)
		}
		if err = os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if raw, err := os.ReadFile(outside); err != nil || string(raw) != "keep" {
		t.Fatal("outside touched", err)
	}
}

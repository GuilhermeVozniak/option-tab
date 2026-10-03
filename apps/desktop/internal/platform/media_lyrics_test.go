package platform

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type fakeLyricsNative struct {
	selection lyricsSelection
	err       error
	during    func()
}

func (f *fakeLyricsNative) choose(context.Context) (lyricsSelection, error) {
	if f.during != nil {
		f.during()
	}
	return f.selection, f.err
}

func (f *fakeLyricsNative) read(context.Context, lyricsRecord) ([]byte, error) {
	return append([]byte(nil), f.selection.Data...), f.err
}

func TestMediaLyricsPersistenceIsolationAndOffset(t *testing.T) {
	native := &fakeLyricsNative{selection: lyricsSelection{Record: lyricsRecord{Bookmark: []byte("bookmark"), Device: 1, Inode: 2}, Data: []byte("[00:01]Original")}}
	path := filepath.Join(t.TempDir(), "media-lyrics.json")
	s := newMediaLyricsSource(path, native)
	scope := MediaLyricsScope{Provider: MediaMusic, TrackID: "one"}
	first, err := s.ChooseMediaLyrics(context.Background(), scope, func(data []byte) error { data[0] = 'X'; return nil })
	if err != nil || first.Data[0] != '[' {
		t.Fatalf("import: %v %+v", err, first)
	}
	if err = s.SetMediaLyricsOffset(context.Background(), scope, 30001); err == nil {
		t.Fatal("unbounded offset")
	}
	if err = s.SetMediaLyricsOffset(context.Background(), scope, -30000); err != nil {
		t.Fatal(err)
	}
	restarted := newMediaLyricsSource(path, native)
	loaded, err := restarted.LoadMediaLyrics(context.Background(), scope)
	if err != nil || loaded.DocumentID != first.DocumentID || loaded.OffsetMS != -30000 {
		t.Fatalf("restart: %v %+v", err, loaded)
	}
	other, err := restarted.LoadMediaLyrics(context.Background(), MediaLyricsScope{Provider: MediaSpotify, TrackID: "one"})
	if err != nil || other.Status != "missing" {
		t.Fatalf("provider isolation: %v %+v", err, other)
	}
	if err = restarted.RemoveMediaLyrics(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	loaded, err = restarted.LoadMediaLyrics(context.Background(), scope)
	if err != nil || loaded.Status != "missing" {
		t.Fatal("remove")
	}
}

func TestMediaLyricsCancelledInvalidImportKeepsPrior(t *testing.T) {
	native := &fakeLyricsNative{selection: lyricsSelection{Record: lyricsRecord{Bookmark: []byte("bookmark"), Inode: 2}, Data: []byte("original")}}
	s := newMediaLyricsSource(filepath.Join(t.TempDir(), "store"), native)
	scope := MediaLyricsScope{Provider: MediaMusic, TrackID: "one"}
	first, err := s.ChooseMediaLyrics(context.Background(), scope, func([]byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ChooseMediaLyrics(context.Background(), scope, func([]byte) error { return errors.New("invalid LRC") })
	if err == nil {
		t.Fatal("invalid accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	native.during = cancel
	_, err = s.ChooseMediaLyrics(ctx, scope, func([]byte) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	loaded, err := s.LoadMediaLyrics(context.Background(), scope)
	if err != nil || loaded.DocumentID != first.DocumentID {
		t.Fatal("prior record lost")
	}
}

type heldLyricsChooser struct {
	entered chan struct{}
	release chan struct{}
}

func (f *heldLyricsChooser) choose(ctx context.Context) (lyricsSelection, error) {
	close(f.entered)
	<-f.release
	return lyricsSelection{}, ctx.Err()
}
func (f *heldLyricsChooser) read(context.Context, lyricsRecord) ([]byte, error) { return nil, nil }
func TestMediaLyricsCancelledChooserRetainsSlotUntilJoined(t *testing.T) {
	native := &heldLyricsChooser{entered: make(chan struct{}), release: make(chan struct{})}
	s := newMediaLyricsSource(filepath.Join(t.TempDir(), "store"), native)
	scope := MediaLyricsScope{Provider: MediaMusic, TrackID: "A"}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := s.ChooseMediaLyrics(ctx, scope, func([]byte) error { return nil }); done <- err }()
	<-native.entered
	cancel()
	_, err := s.ChooseMediaLyrics(context.Background(), scope, func([]byte) error { return nil })
	if err == nil {
		t.Fatal("second chooser admitted during cancellation drain")
	}
	select {
	case <-done:
		t.Fatal("chooser returned before native joined")
	default:
	}
	close(native.release)
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel returned %v", err)
	}
}

func TestMediaLyricsInvalidScopeAndSizeNeverReplace(t *testing.T) {
	native := &fakeLyricsNative{selection: lyricsSelection{Record: lyricsRecord{Bookmark: []byte("b"), Inode: 2}, Data: make([]byte, (1<<20)+1)}}
	s := newMediaLyricsSource(filepath.Join(t.TempDir(), "store"), native)
	for _, scope := range []MediaLyricsScope{{}, {Provider: MediaMusic}, {Provider: "other", TrackID: "A"}} {
		if _, err := s.ChooseMediaLyrics(context.Background(), scope, func([]byte) error { return nil }); err == nil {
			t.Fatal("invalid scope accepted")
		}
	}
	called := false
	_, err := s.ChooseMediaLyrics(context.Background(), MediaLyricsScope{Provider: MediaMusic, TrackID: "A"}, func([]byte) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("oversized data reached validation or persistence")
	}
}

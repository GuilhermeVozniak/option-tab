package launcheritems

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureRecord() Record {
	return Record{Kind: "folder", Label: "Docs", Bookmark: []byte("private-bookmark"), SelectedPath: "/tmp/Documents"}
}

func TestStoreCreateUpdateRestartCopies(t *testing.T) {
	root := filepath.Join(t.TempDir(), "refs")
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.Create(context.Background(), fixtureRecord())
	if err != nil || created.ID == "" || created.Revision != 1 {
		t.Fatal(created, err)
	}
	created.Bookmark[0] = 'X'
	got, _ := s.Get(created.ID)
	if string(got.Bookmark) != "private-bookmark" {
		t.Fatal("copy")
	}
	got.Label = "Documents"
	updated, err := s.Put(context.Background(), got)
	if err != nil || updated.Revision != 2 {
		t.Fatal(updated, err)
	}
	if _, err = s.Put(context.Background(), got); !errors.Is(err, ErrStale) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, _ = s.Get(created.ID)
	if got.Label != "Documents" || got.Revision != 2 {
		t.Fatal(got)
	}
	list := s.List()
	list[0].Bookmark[0] = 'Y'
	again, _ := s.Get(created.ID)
	if bytes.Equal(list[0].Bookmark, again.Bookmark) {
		t.Fatal("alias")
	}
	info, _ := os.Stat(root)
	if info.Mode().Perm() != 0o700 {
		t.Fatal(info.Mode())
	}
	files, _ := os.ReadDir(root)
	fi, _ := files[0].Info()
	if fi.Mode().Perm() != 0o600 {
		t.Fatal(fi.Mode())
	}
}

func TestStoreCancelRollbackCorruptLinkClose(t *testing.T) {
	root := filepath.Join(t.TempDir(), "refs")
	s, _ := OpenStore(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Create(ctx, fixtureRecord()); !errors.Is(err, context.Canceled) || len(s.List()) != 0 {
		t.Fatal(err)
	}
	created, _ := s.Create(context.Background(), fixtureRecord())
	original, _ := s.Get(created.ID)
	s.rename = func(string, string) error { return errors.New("disk full") }
	created.Label = "Changed"
	if _, err := s.Put(context.Background(), created); err == nil {
		t.Fatal("save accepted")
	}
	after, _ := s.Get(created.ID)
	if after.Label != original.Label || after.Revision != original.Revision {
		t.Fatal("rollback")
	}
	s.rename = os.Rename
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(created.ID); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(root)
	if err := os.WriteFile(filepath.Join(root, files[0].Name()), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(root); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(link); !errors.Is(err, ErrUnsafePath) {
		t.Fatal(err)
	}
}

func TestStoreBoundsKinds(t *testing.T) {
	s, _ := OpenStore(filepath.Join(t.TempDir(), "refs"))
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	bad := []Record{{Kind: "link", Label: "x", Bookmark: []byte("x")}, {Kind: "folder", Label: "x", SelectedPath: "relative"}, {Kind: "app", Label: "x", SelectedPath: "/A.app"}, {Kind: "file", Label: strings.Repeat("x", 81), SelectedPath: "/tmp/x"}}
	for _, r := range bad {
		if _, err := s.Create(context.Background(), r); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted %+v: %v", r, err)
		}
	}
	big := fixtureRecord()
	big.Bookmark = make([]byte, 4<<20)
	if _, err := s.Create(context.Background(), big); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
}

func TestStoreBoundsRecordCount(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "refs"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	for i := 0; i < 128; i++ {
		if _, err := s.Create(context.Background(), fixtureRecord()); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, err := s.Create(context.Background(), fixtureRecord()); !errors.Is(err, ErrTooLarge) {
		t.Fatal("129th record accepted", err)
	}
}

func TestStoreRejectsPublicRecordPermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "refs")
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Create(context.Background(), fixtureRecord())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(filepath.Join(root, r.ID+".json"), 0o644); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(root)
	if reopened != nil {
		_ = reopened.Close()
	}
	if err == nil {
		t.Fatal("accepted readable-by-others private bookmark")
	}
}

type expandingRecordReader struct{ read int }

func (r *expandingRecordReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 'x'
	}
	r.read += len(b)
	return len(b), nil
}

func TestRecordReadAllocationUsesRemainingBudget(t *testing.T) {
	r := &expandingRecordReader{}
	if _, err := readBoundedRecord(r, 31); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if r.read != 32 {
		t.Fatalf("read %d bytes beyond remaining budget", r.read)
	}
	root := t.TempDir()
	name := "record.json"
	if err := os.WriteFile(filepath.Join(root, name), bytes.Repeat([]byte("x"), 64), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dir.Close() }()
	if _, err = readPrivateRecord(dir, name, 32); !errors.Is(err, ErrTooLarge) {
		t.Fatal("aggregate budget not enforced before parse", err)
	}
}

func TestRecordReadRejectsGrowthAndNamedReplacement(t *testing.T) {
	for _, mode := range []string{"growth", "replacement", "permissions"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "record.json")
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			dir, err := os.OpenRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = dir.Close() }()
			_, err = readPrivateRecordUsing(dir, "record.json", 32, func(r io.Reader, budget int) ([]byte, error) {
				b, e := readBoundedRecord(r, budget)
				if e != nil {
					return nil, e
				}
				switch mode {
				case "growth":
					if e = os.WriteFile(path, []byte("original grew"), 0o600); e != nil {
						t.Fatal(e)
					}
				case "replacement":
					replacement := filepath.Join(root, "new")
					if e = os.WriteFile(replacement, []byte("original"), 0o600); e != nil {
						t.Fatal(e)
					}
					if e = os.Rename(replacement, path); e != nil {
						t.Fatal(e)
					}
				case "permissions":
					if e = os.Chmod(path, 0o644); e != nil {
						t.Fatal(e)
					}
				}
				return b, nil
			})
			if !errors.Is(err, ErrCorrupt) {
				t.Fatal("changed record accepted", err)
			}
		})
	}
}

func TestOpenStoreToleratesOwnLeftoversOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "refs")
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.Create(context.Background(), fixtureRecord())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	// A crash between create and rename leaves the staged temp behind, and
	// Finder drops its metadata file into any directory it displays.
	stale := filepath.Join(root, ".record-"+strings.Repeat("ab", 16))
	for _, name := range []string{stale, filepath.Join(root, ".DS_Store")} {
		if err = os.WriteFile(name, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Only an abandoned temp is a leftover; age it past the live-writer window.
	aged := time.Now().Add(-2 * stagedRecordGrace)
	if err = os.Chtimes(stale, aged, aged); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(root)
	if err != nil {
		t.Fatalf("leftovers made the store unreadable: %v", err)
	}
	if got, err := s.Get(created.ID); err != nil || got.Label != "Docs" {
		t.Fatal(got, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp not removed: %v", err)
	}
	if _, err = os.Lstat(filepath.Join(root, ".DS_Store")); err != nil {
		t.Fatalf("foreign metadata must be left alone: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err = os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, plant := range map[string]func(string) error{
		".record-" + strings.Repeat("cd", 16): func(p string) error { return os.Symlink(outside, p) },
		".record-" + strings.Repeat("ef", 16): func(p string) error { return os.Mkdir(p, 0o700) },
		".DS_Store":                           func(p string) error { _ = os.Remove(p); return os.Symlink(outside, p) },
		".record-short":                       func(p string) error { return os.WriteFile(p, nil, 0o600) },
		"notes.txt":                           func(p string) error { return os.WriteFile(p, nil, 0o600) },
	} {
		path := filepath.Join(root, name)
		if err = plant(path); err != nil {
			t.Fatal(err)
		}
		if _, err = OpenStore(root); !errors.Is(err, ErrCorrupt) {
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

func TestOpenStoreCountsRecordsNotLeftovers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "refs")
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxRecords; i++ {
		if _, err = s.Create(context.Background(), fixtureRecord()); err != nil {
			t.Fatal(i, err)
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".DS_Store"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(root)
	if err != nil {
		t.Fatalf("full store with Finder metadata: %v", err)
	}
	if got := len(s.List()); got != maxRecords {
		t.Fatal(got)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
}

// A second launch opens the store before the single-instance guard stops it,
// so a temp the running instance is about to rename must survive that open.
func TestOpenStoreLeavesALiveWritersStagedTempAlone(t *testing.T) {
	root := filepath.Join(t.TempDir(), "refs")
	s, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	inFlight := filepath.Join(root, ".record-"+strings.Repeat("12", 16))
	if err = os.WriteFile(inFlight, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(root)
	if err != nil {
		t.Fatalf("an in-flight write made the store unreadable: %v", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(inFlight); err != nil {
		t.Fatalf("in-flight temp of a live writer was removed: %v", err)
	}
}

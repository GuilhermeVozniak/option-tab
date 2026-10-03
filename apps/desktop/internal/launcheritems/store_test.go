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

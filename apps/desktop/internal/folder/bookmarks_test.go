package folder

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBookmarksPersistPrivateCopiedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grants", "bookmarks.json")
	s := NewBookmarkStore(path)
	data := []byte("bookmark")
	if err := s.Put("file:///fixture", data); err != nil {
		t.Fatal(err)
	}
	data[0] = 'X'
	got, err := s.Get("file:///fixture")
	if err != nil || !bytes.Equal(got, []byte("bookmark")) {
		t.Fatal(got, err)
	}
	got[0] = 'Y'
	restarted := NewBookmarkStore(path)
	got, err = restarted.Get("file:///fixture")
	if err != nil || string(got) != "bookmark" {
		t.Fatal(got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal(info, err)
	}
	if err := restarted.Delete("file:///fixture"); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get("file:///fixture")
	if err != nil || got != nil {
		t.Fatal("removed grant survived", got, err)
	}
}

func TestBookmarksCorruptionCannotBeOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks.json")
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewBookmarkStore(path)
	if _, err := s.Get("x"); err == nil {
		t.Fatal("corruption hidden")
	}
	if err := s.Put("x", []byte("value")); err == nil {
		t.Fatal("corrupted store overwritten")
	}
}

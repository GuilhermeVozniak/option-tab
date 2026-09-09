//go:build darwin

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func folderFixture(t *testing.T) (*darwinFolderSource, FolderRef) {
	t.Helper()
	path := t.TempDir()
	identity, _ := FolderIdentity(path)
	source := newDarwinFolderSource(filepath.Join(t.TempDir(), "bookmarks.json"))
	source.testOpen = true
	return source, FolderRef{identity, path}
}

func TestFolderNativeBoundedListingAndExactOpenRefusal(t *testing.T) {
	s, ref := folderFixture(t)
	for _, name := range []string{"b", "a", ".hidden"} {
		if err := os.WriteFile(filepath.Join(ref.Path, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(ref.Path, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/", filepath.Join(ref.Path, "link")); err != nil {
		t.Fatal(err)
	}
	listing, err := s.ListFolder(context.Background(), ref, FolderSort{FoldersFirst: true})
	if err != nil || listing.Status != "ready" || len(listing.Entries) != 5 || listing.Entries[0].Kind != "folder" {
		t.Fatal(listing, err)
	}
	refusal := errors.New("explicit test refusal")
	calls := 0
	for _, e := range listing.Entries {
		if e.Name == "a" {
			err = s.OpenFolderEntryGuarded(context.Background(), ref, e.ID, func() error { calls++; return refusal })
			if !errors.Is(err, refusal) {
				t.Fatal(err)
			}
		}
		if e.Kind == "symlink" {
			if err = s.OpenFolderEntryGuarded(context.Background(), ref, e.ID, func() error { calls++; return refusal }); err == nil {
				t.Fatal("symlink opened")
			}
		}
	}
	if calls != 1 {
		t.Fatal("wrong native guard count", calls)
	}
	old := listing.Entries[0].ID
	if _, err = s.ListFolder(context.Background(), ref, FolderSort{}); err != nil {
		t.Fatal(err)
	}
	if err = s.OpenFolderEntryGuarded(context.Background(), ref, old, func() error { t.Error("retired ID dispatched"); return refusal }); err == nil {
		t.Fatal("old ID accepted")
	}
}

func TestFolderCapAndParentReplacement(t *testing.T) {
	s, ref := folderFixture(t)
	for i := 0; i < 502; i++ {
		if err := os.WriteFile(filepath.Join(ref.Path, fmt.Sprint(i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	listing, err := s.ListFolder(context.Background(), ref, FolderSort{})
	if err != nil || listing.Status != "partial" || len(listing.Entries) != 500 {
		t.Fatal(listing.Status, len(listing.Entries), err)
	}
	if err = os.Rename(ref.Path, ref.Path+"-retired"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(ref.Path + "-retired") }()
	if err = os.Mkdir(ref.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ListFolder(context.Background(), ref, FolderSort{}); err == nil {
		t.Fatal("replacement parent adopted")
	}
}

func TestFolderMissingAndCancellation(t *testing.T) {
	s, ref := folderFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ListFolder(ctx, ref, FolderSort{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := os.Remove(ref.Path); err != nil {
		t.Fatal(err)
	}
	listing, err := s.ListFolder(context.Background(), ref, FolderSort{})
	if err == nil || listing.Status != "missing" {
		t.Fatal(listing, err)
	}
}

func TestFolderNativeBookmarkRoundTripAndRevocation(t *testing.T) {
	s, ref := folderFixture(t)
	bytes, err := nativeFolderBookmark(ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.Put(ref.Identity, bytes); err != nil {
		t.Fatal(err)
	}
	listing, err := s.ListFolder(context.Background(), ref, FolderSort{})
	if err != nil || listing.Status != "ready" {
		t.Fatalf("native fixture bookmark reuse: status=%s err=%v", listing.Status, err)
	}
	if err = s.store.Put(ref.Identity, []byte("invalid bookmark")); err != nil {
		t.Fatal(err)
	}
	listing, err = s.ListFolder(context.Background(), ref, FolderSort{})
	if err == nil || listing.Status != "revoked" {
		t.Fatal(listing, err)
	}
	if data, e := s.store.Get(ref.Identity); e != nil || len(data) != 0 {
		t.Fatal("revoked bookmark retained", e)
	}
}

func TestFolderChildReplacementRefusedBeforeGuard(t *testing.T) {
	s, ref := folderFixture(t)
	path := filepath.Join(ref.Path, "target")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	listing, err := s.ListFolder(context.Background(), ref, FolderSort{})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	err = s.OpenFolderEntryGuarded(context.Background(), ref, listing.Entries[0].ID, func() error { called = true; return errors.New("guard should not be reached") })
	if err == nil || called {
		t.Fatal("replaced child admitted", err, called)
	}
}

func TestFolderBookmarkRejectsParentReplacementAfterSourceRestart(t *testing.T) {
	s, ref := folderFixture(t)
	bytes, err := nativeFolderBookmark(ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.Put(ref.Identity, bytes); err != nil {
		t.Fatal(err)
	}
	fresh := newDarwinFolderSource("")
	fresh.store = s.store
	if _, err = fresh.ListFolder(context.Background(), ref, FolderSort{}); err != nil {
		t.Fatal("bookmark not usable in fresh service", err)
	}
	if err = os.Rename(ref.Path, ref.Path+"-original"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(ref.Path + "-original") }()
	if err = os.Mkdir(ref.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	afterRestart := newDarwinFolderSource("")
	afterRestart.store = s.store
	listing, err := afterRestart.ListFolder(context.Background(), ref, FolderSort{})
	if err == nil || listing.Status != "revoked" {
		t.Fatal("bookmark adopted replacement", listing.Status, err)
	}
}

func TestFolderCancelledGrantNeverPromptsOrPersists(t *testing.T) {
	s, ref := folderFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := s.RequestFolderAccess(ctx, ref)
	if !errors.Is(err, context.Canceled) || result.Status != "permissionRequired" {
		t.Fatal(result, err)
	}
	if bytes, err := s.store.Get(ref.Identity); err != nil || len(bytes) != 0 {
		t.Fatal("cancelled grant persisted", err)
	}
}

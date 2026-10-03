package platform

import "testing"

func TestFolderIdentityNoFilesystemResolution(t *testing.T) {
	got, err := FolderIdentity("/missing/folder with space")
	if err != nil || got != "file:///missing/folder%20with%20space" {
		t.Fatal(got, err)
	}
	for _, p := range []string{"relative", "/a/../b", "/a\x00b"} {
		if _, err := FolderIdentity(p); err == nil {
			t.Fatal(p)
		}
	}
}

func TestFolderSortingDeterministic(t *testing.T) {
	source := []FolderEntry{{ID: "b", Name: "Alpha", Kind: "file", Size: 2, ModifiedAtMs: 1}, {ID: "a", Name: "alpha", Kind: "file", Size: 1, ModifiedAtMs: 2}, {ID: "c", Name: "Z", Kind: "folder", Size: 0}}
	for _, tt := range []struct{ field, direction, want string }{{"name", "asc", "a"}, {"name", "desc", "c"}, {"size", "asc", "c"}, {"size", "desc", "b"}, {"modified", "asc", "c"}, {"modified", "desc", "a"}, {"kind", "asc", "a"}} {
		entries := append([]FolderEntry(nil), source...)
		if err := sortFolderEntries(entries, FolderSort{Field: tt.field, Direction: tt.direction}); err != nil || entries[0].ID != tt.want {
			t.Fatal(tt, entries, err)
		}
	}
	entries := append([]FolderEntry(nil), source...)
	if err := sortFolderEntries(entries, FolderSort{Field: "name", Direction: "desc", FoldersFirst: true}); err != nil || entries[0].Kind != "folder" {
		t.Fatal(entries, err)
	}
	if err := sortFolderEntries(entries, FolderSort{Field: "shell"}); err == nil {
		t.Fatal("unknown sort admitted")
	}
}

package platform

import (
	"cmp"
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

// FolderRef is captured from a canonical Dock file URL, never a frontend path.
type (
	FolderRef  struct{ Identity, Path string }
	FolderSort struct {
		Field        string `json:"field"`
		Direction    string `json:"direction"`
		FoldersFirst bool   `json:"foldersFirst"`
	}
)

type FolderEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Size         int64  `json:"size"`
	ModifiedAtMs int64  `json:"modifiedAtMs"`
	Hidden       bool   `json:"hidden"`
}
type FolderListing struct {
	FolderIdentity string        `json:"folderIdentity"`
	Entries        []FolderEntry `json:"entries"`
	Status         string        `json:"status"`
	Reason         string        `json:"reason"`
}
type FolderGrant struct {
	FolderIdentity string `json:"folderIdentity"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
}

// List is capped and checks cancellation between filesystem operations; native
// filesystem calls themselves may outlive the deadline. List never prompts.
// Open accepts only an
// opaque ID from this source's current listing; guard runs after native identity
// preparation immediately before workspace dispatch, without source locks.
type FolderSource interface {
	ListFolder(context.Context, FolderRef, FolderSort) (FolderListing, error)
	RequestFolderAccess(context.Context, FolderRef) (FolderGrant, error)
	OpenFolderEntryGuarded(context.Context, FolderRef, string, func() error) error
}

// FolderIdentity encodes an already canonical Dock path without accessing disk.
func FolderIdentity(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, 0) {
		return "", errors.New("folder path must be absolute and normalized")
	}
	return (&url.URL{Scheme: "file", Path: path}).String(), nil
}

func sortFolderEntries(entries []FolderEntry, order FolderSort) error {
	if order.Field == "" {
		order.Field = "name"
	}
	if order.Direction == "" {
		order.Direction = "asc"
	}
	if (order.Field != "name" && order.Field != "modified" && order.Field != "size" && order.Field != "kind") || (order.Direction != "asc" && order.Direction != "desc") {
		return errors.New("invalid folder sort")
	}
	slices.SortFunc(entries, func(a, b FolderEntry) int {
		if order.FoldersFirst && (a.Kind == "folder") != (b.Kind == "folder") {
			if a.Kind == "folder" {
				return -1
			}
			return 1
		}
		value := 0
		switch order.Field {
		case "name":
			value = cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "modified":
			value = cmp.Compare(a.ModifiedAtMs, b.ModifiedAtMs)
		case "size":
			value = cmp.Compare(a.Size, b.Size)
		case "kind":
			value = cmp.Compare(a.Kind, b.Kind)
		}
		if order.Direction == "desc" {
			value = -value
		}
		if value != 0 {
			return value
		}
		if value = cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); value != 0 {
			return value
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return nil
}

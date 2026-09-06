//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_folder.h"
*/
import "C"

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/cgo"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"option-tab/internal/folder"
)

type (
	folderFileIdentity struct{ Device, Inode uint64 }
	folderOpenEntry    struct {
		path     string
		identity folderFileIdentity
		kind     string
	}
	folderSnapshot struct {
		parent  folderFileIdentity
		entries map[string]folderOpenEntry
	}
	darwinFolderSource struct {
		testOpen  bool // native preparation seam; production leaves false
		mu        sync.Mutex
		store     *folder.BookmarkStore
		known     map[string]folderFileIdentity
		snapshots map[string]folderSnapshot
		revisions map[string]uint64
	}
)

func newDarwinFolderSource(path string) *darwinFolderSource {
	return &darwinFolderSource{store: folder.NewBookmarkStore(path), known: map[string]folderFileIdentity{}, snapshots: map[string]folderSnapshot{}, revisions: map[string]uint64{}}
}

// NewFolderSource permits an isolated bookmark store for an application instance.
func NewFolderSource(bookmarkPath string) FolderSource { return newDarwinFolderSource(bookmarkPath) }

var defaultFolders = sync.OnceValue(func() *darwinFolderSource {
	dir, err := os.UserConfigDir()
	if err != nil {
		return newDarwinFolderSource("")
	}
	return newDarwinFolderSource(filepath.Join(dir, "option-tab", "folder-bookmarks.json"))
})

func (p *darwinPlatform) ListFolder(ctx context.Context, ref FolderRef, order FolderSort) (FolderListing, error) {
	return defaultFolders().ListFolder(ctx, ref, order)
}

func (p *darwinPlatform) RequestFolderAccess(ctx context.Context, ref FolderRef) (FolderGrant, error) {
	return defaultFolders().RequestFolderAccess(ctx, ref)
}

func (p *darwinPlatform) OpenFolderEntryGuarded(ctx context.Context, ref FolderRef, id string, guard func() error) error {
	return defaultFolders().OpenFolderEntryGuarded(ctx, ref, id, guard)
}

func validateFolderRef(ref FolderRef) error {
	id, err := FolderIdentity(ref.Path)
	if err != nil {
		return err
	}
	if id != ref.Identity {
		return errors.New("folder identity does not match captured Dock URL")
	}
	return nil
}

func folderStat(path string) (folderFileIdentity, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return folderFileIdentity{}, nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return folderFileIdentity{}, nil, errors.New("file identity unavailable")
	}
	return folderFileIdentity{uint64(stat.Dev), stat.Ino}, info, nil
}

func (s *darwinFolderSource) begin(ref FolderRef) (unsafe.Pointer, bool, error) {
	data, err := s.store.Get(ref.Identity)
	if err != nil {
		return nil, false, err
	}
	path := C.CString(ref.Path)
	defer C.free(unsafe.Pointer(path))
	var bytes unsafe.Pointer
	if len(data) > 0 {
		bytes = C.CBytes(data)
		defer C.free(bytes)
	}
	var message, refresh *C.char
	scope := C.ot_folder_begin(path, bytes, C.int(len(data)), &message, &refresh)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
	}
	if refresh != nil {
		defer C.free(unsafe.Pointer(refresh))
	}
	if scope == nil {
		if len(data) > 0 {
			if e := s.store.Delete(ref.Identity); e != nil {
				return nil, true, e
			}
		}
		reason := "folder access unavailable"
		if message != nil {
			reason = C.GoString(message)
		}
		return nil, len(data) > 0, errors.New(reason)
	}
	if refresh != nil {
		b, e := base64.StdEncoding.DecodeString(C.GoString(refresh))
		if e == nil {
			e = s.store.Put(ref.Identity, b)
		}
		if e != nil {
			C.ot_folder_end(scope)
			return nil, len(data) > 0, e
		}
	}
	return scope, len(data) > 0, nil
}

func (s *darwinFolderSource) remember(ref FolderRef, id folderFileIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if known, ok := s.known[ref.Identity]; ok && known != id {
		return errors.New("captured folder was replaced")
	}
	s.known[ref.Identity] = id
	return nil
}

func (s *darwinFolderSource) ListFolder(ctx context.Context, ref FolderRef, order FolderSort) (FolderListing, error) {
	out := FolderListing{FolderIdentity: ref.Identity, Status: "unavailable", Entries: []FolderEntry{}}
	fail := func(status string, err error) (FolderListing, error) {
		out.Status = status
		out.Reason = err.Error()
		return out, err
	}
	if ctx == nil {
		return fail("unavailable", errors.New("missing context"))
	}
	if err := ctx.Err(); err != nil {
		return fail("unavailable", err)
	}
	if err := validateFolderRef(ref); err != nil {
		return fail("unavailable", err)
	}
	if err := sortFolderEntries(nil, order); err != nil {
		return fail("unavailable", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	s.mu.Lock()
	s.revisions[ref.Identity]++
	revision := s.revisions[ref.Identity]
	delete(s.snapshots, ref.Identity)
	s.mu.Unlock()
	scope, bookmarked, err := s.begin(ref)
	if err != nil {
		status := "permissionRequired"
		if bookmarked {
			status = "revoked"
		}
		if _, e := os.Lstat(ref.Path); errors.Is(e, os.ErrNotExist) {
			status = "missing"
		}
		return fail(status, err)
	}
	defer C.ot_folder_end(scope)
	parent, info, err := folderStat(ref.Path)
	if err != nil {
		status := "permissionRequired"
		if errors.Is(err, os.ErrNotExist) {
			status = "missing"
		}
		return fail(status, err)
	}
	if !info.IsDir() {
		return fail("missing", errors.New("captured folder is not a directory"))
	}
	if err = s.remember(ref, parent); err != nil {
		return fail("revoked", err)
	}
	dir, err := os.Open(ref.Path)
	if err != nil {
		return fail("permissionRequired", err)
	}
	defer func() { _ = dir.Close() }()
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return fail("unavailable", err)
	}
	prefix := hex.EncodeToString(token[:])
	snapshot := folderSnapshot{parent: parent, entries: map[string]folderOpenEntry{}}
	partial := false
	for {
		if err = ctx.Err(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				partial = true
				break
			}
			return fail("unavailable", err)
		}
		entries, e := dir.ReadDir(1)
		if len(entries) == 0 && errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return fail("unavailable", e)
		}
		if len(out.Entries) == 500 {
			partial = true
			break
		}
		entry := entries[0]
		child := filepath.Join(ref.Path, entry.Name())
		identity, childInfo, e := folderStat(child)
		if e != nil {
			partial = true
			continue
		}
		kind := "file"
		if childInfo.IsDir() {
			kind = "folder"
		} else if childInfo.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
		} else if !childInfo.Mode().IsRegular() {
			kind = "other"
		}
		id := fmt.Sprintf("%s-%d", prefix, len(out.Entries))
		out.Entries = append(out.Entries, FolderEntry{ID: id, Name: entry.Name(), Kind: kind, Size: childInfo.Size(), ModifiedAtMs: childInfo.ModTime().UnixMilli(), Hidden: strings.HasPrefix(entry.Name(), ".")})
		snapshot.entries[id] = folderOpenEntry{child, identity, kind}
	}
	if err = sortFolderEntries(out.Entries, order); err != nil {
		return fail("unavailable", err)
	}
	current, _, err := folderStat(ref.Path)
	if err != nil || current != parent {
		return fail("revoked", errors.New("folder identity changed while listing"))
	}
	s.mu.Lock()
	if s.revisions[ref.Identity] != revision {
		s.mu.Unlock()
		return fail("unavailable", errors.New("folder listing retired"))
	}
	if len(s.snapshots) >= 32 {
		for key := range s.snapshots {
			delete(s.snapshots, key)
			break
		}
	}
	s.snapshots[ref.Identity] = snapshot
	s.mu.Unlock()
	out.Status = "ready"
	if partial {
		out.Status = "partial"
		out.Reason = "folder listing reached its entry or time limit"
	}
	return out, nil
}

type folderFinalGuard struct {
	call func() error
	err  error
}

//export goFolderFinalGuard
func goFolderFinalGuard(token C.uintptr_t) C.int {
	state := cgo.Handle(token).Value().(*folderFinalGuard)
	state.err = state.call()
	if state.err != nil {
		return 0
	}
	return 1
}

func (s *darwinFolderSource) OpenFolderEntryGuarded(ctx context.Context, ref FolderRef, id string, guard func() error) error {
	if ctx == nil || guard == nil {
		return errors.New("folder open requires context and final guard")
	}
	if err := validateFolderRef(ref); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	snapshot, ok := s.snapshots[ref.Identity]
	entry, exists := snapshot.entries[id]
	s.mu.Unlock()
	if !ok || !exists || (entry.kind != "file" && entry.kind != "folder") {
		return errors.New("folder entry is stale or unsupported")
	}
	scope, _, err := s.begin(ref)
	if err != nil {
		return err
	}
	defer C.ot_folder_end(scope)
	state := &folderFinalGuard{call: func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		latest, ok := s.snapshots[ref.Identity]
		current, exists := latest.entries[id]
		valid := ok && exists && latest.parent == snapshot.parent && current == entry
		s.mu.Unlock()
		if !valid {
			return errors.New("folder entry retired before open")
		}
		return guard()
	}}
	handle := cgo.NewHandle(state)
	defer handle.Delete()
	path := C.CString(entry.path)
	defer C.free(unsafe.Pointer(path))
	var code C.int
	if s.testOpen {
		code = C.ot_folder_open_probe(scope, path, C.uint64_t(snapshot.parent.Device), C.uint64_t(snapshot.parent.Inode), C.uint64_t(entry.identity.Device), C.uint64_t(entry.identity.Inode), C.uintptr_t(handle))
	} else {
		code = C.ot_folder_open(scope, path, C.uint64_t(snapshot.parent.Device), C.uint64_t(snapshot.parent.Inode), C.uint64_t(entry.identity.Device), C.uint64_t(entry.identity.Inode), C.uintptr_t(handle))
	}
	if state.err != nil {
		return state.err
	}
	if code != 0 {
		return fmt.Errorf("folder entry open refused (%d)", code)
	}
	return nil
}

func (s *darwinFolderSource) RequestFolderAccess(ctx context.Context, ref FolderRef) (FolderGrant, error) {
	out := FolderGrant{FolderIdentity: ref.Identity, Status: "permissionRequired"}
	fail := func(err error) (FolderGrant, error) { out.Reason = err.Error(); return out, err }
	if ctx == nil {
		return fail(errors.New("missing context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := validateFolderRef(ref); err != nil {
		return fail(err)
	}
	if id, info, err := folderStat(ref.Path); err == nil {
		if !info.IsDir() {
			return fail(errors.New("captured folder is not a directory"))
		}
		if err = s.remember(ref, id); err != nil {
			return fail(err)
		}
	}
	path := C.CString(ref.Path)
	native := C.ot_folder_grant_start(path)
	C.free(unsafe.Pointer(path))
	if native == nil {
		return fail(errors.New("folder chooser unavailable"))
	}
	defer C.ot_folder_grant_release(native)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	cancelled := false
	for {
		if ctx.Err() != nil && !cancelled {
			C.ot_folder_grant_cancel(native)
			cancelled = true
		}
		raw := C.ot_folder_grant_poll(native)
		if raw != nil {
			var result struct {
				Status, Reason, Bookmark string
				Device, Inode            uint64
			}
			err := json.Unmarshal([]byte(C.GoString(raw)), &result)
			C.free(unsafe.Pointer(raw))
			if cancelled {
				return fail(ctx.Err())
			}
			if err != nil {
				return fail(err)
			}
			if result.Status != "ready" {
				return fail(errors.New(result.Reason))
			}
			if err = s.remember(ref, folderFileIdentity{result.Device, result.Inode}); err != nil {
				return fail(err)
			}
			bytes, e := base64.StdEncoding.DecodeString(result.Bookmark)
			if e != nil {
				return fail(e)
			}
			if err = s.store.Put(ref.Identity, bytes); err != nil {
				return fail(err)
			}
			out.Status = "ready"
			return out, nil
		}
		<-ticker.C
	}
}

func nativeFolderBookmark(path string) ([]byte, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	data := C.ot_folder_test_bookmark(p)
	if data == nil {
		return nil, errors.New("fixture bookmark creation unavailable")
	}
	defer C.free(unsafe.Pointer(data))
	return base64.StdEncoding.DecodeString(C.GoString(data))
}

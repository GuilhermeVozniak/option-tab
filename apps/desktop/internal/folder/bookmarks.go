// Package folder owns opaque bookmark persistence, without platform dependencies.
package folder

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const maxBookmarkBytes = 4 << 20

// Serialize stores in this process, including separately constructed handles.
var bookmarkMu sync.Mutex

type BookmarkStore struct{ path string }

func NewBookmarkStore(path string) *BookmarkStore { return &BookmarkStore{path: path} }
func (s *BookmarkStore) read() (map[string][]byte, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxBookmarkBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBookmarkBytes {
		return nil, errors.New("bookmark store exceeds limit")
	}
	var out map[string][]byte
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string][]byte{}
	}
	return out, nil
}

func (s *BookmarkStore) Get(key string) ([]byte, error) {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	all, err := s.read()
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), all[key]...), nil
}
func (s *BookmarkStore) Put(key string, data []byte) error { return s.change(key, data) }
func (s *BookmarkStore) Delete(key string) error           { return s.change(key, nil) }
func (s *BookmarkStore) change(key string, data []byte) error {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	if key == "" || s.path == "" {
		return errors.New("missing bookmark identity or store path")
	}
	all, err := s.read()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		delete(all, key)
	} else {
		all[key] = append([]byte(nil), data...)
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		return err
	}
	if len(encoded) > maxBookmarkBytes {
		return errors.New("bookmark store exceeds limit")
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".bookmarks-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(encoded); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.path)
}

package launcheritems

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	maxRecords    = 128
	maxStoreBytes = 4 << 20
)

var (
	ErrInvalid    = errors.New("launcher items: invalid record")
	ErrStale      = errors.New("launcher items: stale record")
	ErrNotFound   = errors.New("launcher items: record not found")
	ErrClosed     = errors.New("launcher items: store closed")
	ErrCorrupt    = errors.New("launcher items: corrupt store")
	ErrUnsafePath = errors.New("launcher items: unsafe store path")
	ErrTooLarge   = errors.New("launcher items: store limit exceeded")
	privateID     = regexp.MustCompile(`^[a-f0-9]{32}$`)
	bundleID      = regexp.MustCompile(`^[A-Za-z0-9._-]{1,255}$`)
)

type Record struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	Bookmark     []byte `json:"bookmark"`
	SelectedPath string `json:"selectedPath"`
	BundleID     string `json:"bundleID,omitempty"`
	Revision     uint64 `json:"revision"`
}
type diskRecord struct {
	Schema int `json:"schema"`
	Record
}
type Store struct {
	mu      sync.Mutex
	dir     *os.Root
	records map[string]Record
	sizes   map[string]int
	closed  bool
	rename  func(string, string) error
}

func clone(r Record) Record { r.Bookmark = append([]byte(nil), r.Bookmark...); return r }
func valid(r Record, requireID bool) bool {
	if requireID && (!privateID.MatchString(r.ID) || r.Revision == 0) ||
		!requireID && (r.ID != "" || r.Revision != 0) ||
		!utf8.ValidString(r.Label) || utf8.RuneCountInString(r.Label) < 1 || utf8.RuneCountInString(r.Label) > 80 || strings.ContainsRune(r.Label, 0) ||
		len(r.Bookmark) == 0 || len(r.Bookmark) > maxStoreBytes || !utf8.ValidString(r.SelectedPath) || !filepath.IsAbs(r.SelectedPath) || filepath.Clean(r.SelectedPath) != r.SelectedPath || strings.ContainsRune(r.SelectedPath, 0) {
		return false
	}
	switch r.Kind {
	case "app":
		return bundleID.MatchString(r.BundleID)
	case "folder", "file":
		return r.BundleID == ""
	}
	return false
}

func OpenStore(root string) (*Store, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, ErrUnsafePath
	}
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafePath
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafePath
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, err
	}
	s := &Store{records: map[string]Record{}, sizes: map[string]int{}}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	openedInfo, openedErr := dir.Stat(".")
	pathInfo, pathErr := os.Lstat(root)
	if openedErr != nil || pathErr != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, pathInfo) {
		_ = dir.Close()
		return nil, ErrUnsafePath
	}
	s.dir = dir
	s.rename = dir.Rename
	opened := false
	defer func() {
		if !opened {
			_ = dir.Close()
		}
	}()
	directory, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := directory.ReadDir(maxRecords + 1)
	_ = directory.Close()
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > maxRecords {
		return nil, ErrTooLarge
	}
	total := 0
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			return nil, ErrCorrupt
		}
		b, e := readPrivateRecord(dir, entry.Name(), maxStoreBytes-total)
		if e != nil {
			return nil, e
		}
		var d diskRecord
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if dec.Decode(&d) != nil || d.Schema != 1 || !valid(d.Record, true) || entry.Name() != d.ID+".json" {
			return nil, ErrCorrupt
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			return nil, ErrCorrupt
		}
		if _, ok := s.records[d.ID]; ok {
			return nil, ErrCorrupt
		}
		s.records[d.ID] = clone(d.Record)
		s.sizes[d.ID] = len(b)
		total += len(b)
	}
	if len(s.records) > maxRecords || total > maxStoreBytes {
		return nil, ErrTooLarge
	}
	opened = true
	return s, nil
}

// Bound allocation by the remaining aggregate budget, independently of a
// potentially stale filesystem size. The held descriptor, initial name and
// final name must identify the same private regular file throughout the read.
func readPrivateRecord(dir *os.Root, name string, budget int) ([]byte, error) {
	return readPrivateRecordUsing(dir, name, budget, readBoundedRecord)
}

func readPrivateRecordUsing(dir *os.Root, name string, budget int, read func(io.Reader, int) ([]byte, error)) ([]byte, error) {
	before, err := dir.Lstat(name)
	if err != nil || !privateRecordInfo(before) {
		return nil, ErrCorrupt
	}
	if before.Size() > int64(budget) {
		return nil, ErrTooLarge
	}
	f, err := dir.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil || !privateRecordInfo(opened) || !sameRecordInfo(before, opened) {
		return nil, ErrCorrupt
	}
	b, err := read(f, budget)
	if err != nil {
		return nil, err
	}
	after, afterErr := f.Stat()
	named, namedErr := dir.Lstat(name)
	if afterErr != nil || namedErr != nil || !privateRecordInfo(after) || !privateRecordInfo(named) || !sameRecordInfo(opened, after) || !sameRecordInfo(after, named) || int64(len(b)) != after.Size() {
		return nil, ErrCorrupt
	}
	return b, nil
}

func privateRecordInfo(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode().Perm() == 0o600 && info.Size() >= 0
}

func sameRecordInfo(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func readBoundedRecord(r io.Reader, budget int) ([]byte, error) {
	if budget < 0 || budget > maxStoreBytes {
		return nil, ErrTooLarge
	}
	b, err := io.ReadAll(io.LimitReader(r, int64(budget)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > budget {
		return nil, ErrTooLarge
	}
	return b, nil
}

func (s *Store) writeLocked(ctx context.Context, r Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	b, err := json.Marshal(diskRecord{Schema: 1, Record: r})
	if err != nil {
		return Record{}, err
	}
	total := len(b)
	for id, n := range s.sizes {
		if id != r.ID {
			total += n
		}
	}
	if total > maxStoreBytes {
		return Record{}, ErrTooLarge
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return Record{}, err
	}
	name := ".record-" + hex.EncodeToString(raw)
	tmp, err := s.dir.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = s.dir.Remove(name) }()
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(b)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = s.rename(name, r.ID+".json")
	}
	if err != nil {
		return Record{}, err
	}
	s.records[r.ID] = clone(r)
	s.sizes[r.ID] = len(b)
	return clone(r), nil
}

func (s *Store) Create(ctx context.Context, r Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Record{}, ErrClosed
	}
	if !valid(r, false) {
		return Record{}, ErrInvalid
	}
	if len(s.records) >= maxRecords {
		return Record{}, ErrTooLarge
	}
	for {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return Record{}, err
		}
		r.ID = hex.EncodeToString(raw)
		if _, ok := s.records[r.ID]; !ok {
			break
		}
	}
	r.Revision = 1
	return s.writeLocked(ctx, r)
}

func (s *Store) Put(ctx context.Context, r Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Record{}, ErrClosed
	}
	if !valid(r, true) {
		return Record{}, ErrInvalid
	}
	old, ok := s.records[r.ID]
	if !ok {
		return Record{}, ErrNotFound
	}
	if old.Revision != r.Revision {
		return Record{}, ErrStale
	}
	if r.Revision == ^uint64(0) {
		return Record{}, ErrTooLarge
	}
	r.Revision++
	return s.writeLocked(ctx, r)
}

func (s *Store) Get(id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Record{}, ErrClosed
	}
	r, ok := s.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return clone(r), nil
}

func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	out := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, clone(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Remove(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := s.records[id]; !ok {
		return ErrNotFound
	}
	if err := s.dir.Remove(id + ".json"); err != nil {
		return err
	}
	delete(s.records, id)
	delete(s.sizes, id)
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.records = nil
	s.sizes = nil
	return s.dir.Close()
}

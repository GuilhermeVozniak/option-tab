package widgets

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
)

// Store owns only an app-private content directory. Close releases its rooted
// directory handle. No manifest path is ever used as a filesystem destination.
type Store struct {
	mu     sync.Mutex
	root   *os.Root
	closed bool
}

func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, ErrInvalid
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.root.Close()
}

func (s *Store) Get(digest string) (*Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, os.ErrClosed
	}
	return s.get(digest)
}

func (s *Store) get(digest string) (*Package, error) {
	if !hash.MatchString(digest) {
		return nil, ErrInvalid
	}
	st, err := s.root.Lstat(digest)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	name := digest + "/package.zip"
	before, err := s.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > MaxCompressed {
		return nil, ErrInvalid
	}
	f, err := s.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	actual, err := f.Stat()
	if err != nil || !os.SameFile(before, actual) {
		return nil, ErrInvalid
	}
	p, err := Preview(context.Background(), f)
	if err != nil {
		return nil, err
	}
	if p.Digest() != digest {
		return nil, ErrInvalid
	}
	return p, nil
}

func (s *Store) Install(ctx context.Context, r io.Reader) (*Package, error) {
	raw, err := boundedRead(ctx, r, MaxCompressed)
	if err != nil {
		return nil, err
	}
	p, err := Preview(ctx, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, os.ErrClosed
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	existing, err := s.get(p.Digest())
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// An existing malformed digest directory must never be overwritten.
	if _, err = s.root.Lstat(p.Digest()); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil, ErrInvalid
	}
	names, err := s.catalogNames(ctx)
	if err != nil {
		return nil, err
	}
	if len(names) >= MaxInstalledPackages {
		return nil, ErrInvalid
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return nil, err
	}
	stage := ".stage-" + hex.EncodeToString(token[:])
	if err = s.root.Mkdir(stage, 0o700); err != nil {
		return nil, err
	}
	defer func() { _ = s.root.RemoveAll(stage) }()
	f, err := s.root.OpenFile(stage+"/package.zip", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	_, writeErr := f.Write(raw)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if syncErr != nil {
		return nil, syncErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = s.root.Rename(stage, p.Digest()); err != nil {
		// Another Store may have installed the same content concurrently.
		if existing, e := s.get(p.Digest()); e == nil {
			return existing, nil
		}
		return nil, err
	}
	return p, nil
}

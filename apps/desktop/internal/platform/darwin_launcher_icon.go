//go:build darwin

package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type darwinLauncherIconSource struct {
	life     *launcherItemLifetime
	mu       sync.Mutex
	root     *os.Root
	picker   chan struct{}
	once     sync.Once
	closeErr error
	choose   func(context.Context, string) (launcherNativeRecord, error)
}

func NewLauncherIconSource(path string) (LauncherIconSource, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, launcherItemError("invalidArgument")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return nil, launcherItemError("unsafePath")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &darwinLauncherIconSource{life: newLauncherItemLifetime(), root: root, picker: make(chan struct{}, 1), choose: chooseLauncherNative}, nil
}

func launcherIconID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (s *darwinLauncherIconSource) ChooseLauncherIcon(ctx context.Context) (string, error) {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return "", err
	}
	defer end()
	select {
	case s.picker <- struct{}{}:
		defer func() { <-s.picker }()
	default:
		return "", launcherItemError("busy")
	}
	raw, err := s.choose(ctx, "icon")
	if err != nil {
		return "", err
	}
	data, err := normalizeLauncherIcon(ctx, raw.Archive)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if existing, err := s.read(id); err == nil {
		if string(existing) != string(data) {
			return "", launcherItemError("changed")
		}
		return id, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return "", err
	}
	entries, err := dir.ReadDir(129)
	closeErr := dir.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if len(entries) >= 128 {
		return "", launcherItemError("tooLarge")
	}
	if closeErr != nil {
		return "", closeErr
	}
	total := 0
	for _, entry := range entries {
		if !launcherIconID(entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
			return "", launcherItemError("unsafePath")
		}
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Size() > 512<<10 {
			return "", launcherItemError("unsafePath")
		}
		total += int(info.Size())
	}
	if total+len(data) > 8<<20 {
		return "", launcherItemError("tooLarge")
	}
	// Exclusive publication writes a digest name only after bytes have been normalized;
	// the source mutex prevents in-process readers observing the write.
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return "", err
	}
	stage := ".icon-" + hex.EncodeToString(nonce[:]) + ".tmp"
	file, err := s.root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	created, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			if named, e := s.root.Lstat(stage); e == nil && os.SameFile(created, named) {
				_ = s.root.Remove(stage)
			}
		}
	}()
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if e := file.Close(); err == nil {
		err = e
	}
	if err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if named, e := s.root.Lstat(stage); e != nil || !os.SameFile(created, named) || !named.Mode().IsRegular() {
		return "", launcherItemError("changed")
	}
	if err = s.root.Rename(stage, id); err != nil {
		return "", err
	}
	committed = true
	return id, nil
}

func (s *darwinLauncherIconSource) read(id string) ([]byte, error) {
	if !launcherIconID(id) {
		return nil, launcherItemError("invalidArgument")
	}
	info, err := s.root.Lstat(id)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 512<<10 || info.Mode().Perm() != 0o600 {
		return nil, launcherItemError("unsafePath")
	}
	f, err := s.root.Open(id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, launcherItemError("changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, 512<<10+1))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	after, err := s.root.Lstat(id)
	if err != nil || !os.SameFile(info, after) || hex.EncodeToString(sum[:]) != id {
		return nil, launcherItemError("changed")
	}
	return data, nil
}

func (s *darwinLauncherIconSource) LauncherIcon(ctx context.Context, id string) ([]byte, error) {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read(id)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return data, err
}

func (s *darwinLauncherIconSource) RemoveLauncherIcon(ctx context.Context, id string) error {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	if !launcherIconID(id) {
		return launcherItemError("invalidArgument")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	if _, err = s.read(id); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return s.root.Remove(id)
}

func (s *darwinLauncherIconSource) Close() error {
	s.once.Do(func() { s.life.close(); s.mu.Lock(); defer s.mu.Unlock(); s.closeErr = s.root.Close() })
	return s.closeErr
}

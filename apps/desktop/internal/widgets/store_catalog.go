package widgets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

const MaxInstalledPackages = 64

// catalogNames bounds directory enumeration, including abandoned staging entries.
// The caller holds mu. Malformed entries are reported without following links.
func (s *Store) catalogNames(ctx context.Context) ([]string, error) {
	f, err := s.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(129)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > 128 {
		return nil, fmt.Errorf("%w: catalog directory limit", ErrInvalid)
	}
	var names []string
	var failures []error
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".stage-") && len(name) == 39 && hash.MatchString(strings.TrimPrefix(name, ".stage-")+strings.Repeat("0", 32)) && entry.IsDir() {
			continue
		}
		if !hash.MatchString(name) || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			failures = append(failures, ErrInvalid)
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) > MaxInstalledPackages {
		return nil, fmt.Errorf("%w: installed package limit", ErrInvalid)
	}
	return names, errors.Join(failures...)
}

// List returns digest-sorted, independently verified immutable packages. Invalid
// entries are omitted with an error; callers must not treat partial results as a
// complete catalog. At most 64 packages and 128 directory entries are inspected.
func (s *Store) List(ctx context.Context) ([]*Package, error) {
	if ctx == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, os.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	names, err := s.catalogNames(ctx)
	var failures []error
	if err != nil {
		failures = append(failures, err)
	}
	packages := make([]*Package, 0, len(names))
	for _, name := range names {
		if err = ctx.Err(); err != nil {
			return packages, err
		}
		p, e := s.get(name)
		if e != nil {
			failures = append(failures, e)
			continue
		}
		if err = ctx.Err(); err != nil {
			return packages, err
		}
		packages = append(packages, p)
	}
	return packages, errors.Join(failures...)
}

// Remove deletes only the exact digest directory inside the app-owned root.
// The host must retire live instances/actions first. Existing Package values
// remain immutable and usable; removing an absent directory returns NotExist.
func (s *Store) Remove(ctx context.Context, digest string) error {
	if ctx == nil || !hash.MatchString(digest) {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := s.root.Lstat(digest)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return s.root.RemoveAll(digest)
}

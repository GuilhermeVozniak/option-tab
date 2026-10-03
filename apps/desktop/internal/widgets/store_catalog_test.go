package widgets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func catalogStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func catalogArchive(t *testing.T, n int) []byte {
	t.Helper()
	m := testManifest()
	m.Version = fmt.Sprintf("1.0.%d", n)
	return testZIP(t, m, nil)
}

func TestStoreCatalogSortedCopiesAndRemoval(t *testing.T) {
	s, dir := catalogStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, ".stage-"+strings.Repeat("a", 32)), 0o700); err != nil {
		t.Fatal(err)
	}
	packages, err := s.List(ctx)
	if err != nil || len(packages) != 3 {
		t.Fatalf("list %d %v", len(packages), err)
	}
	for i := 1; i < len(packages); i++ {
		if packages[i-1].Digest() >= packages[i].Digest() {
			t.Fatal("not sorted")
		}
	}
	digest := packages[0].Digest()
	m := packages[0].Manifest()
	m.Name["en"] = "changed"
	again, err := s.List(ctx)
	if err != nil || again[0].Manifest().Name["en"] == "changed" {
		t.Fatal("copy", err)
	}
	if err = s.Remove(ctx, digest); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(digest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("not removed", err)
	}
	if packages[0].Manifest().Name["en"] != "Clock" {
		t.Fatal("live immutable package changed")
	}
}

func TestStoreCatalogRejectsMalformedAndLinks(t *testing.T) {
	for _, kind := range []string{"name", "directoryLink", "archiveLink", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			s, dir := catalogStore(t)
			ctx := context.Background()
			p, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, 0)))
			if err != nil {
				t.Fatal(err)
			}
			bad := strings.Repeat("f", 64)
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "keep")
			if err = os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "name":
				err = os.Mkdir(filepath.Join(dir, "unexpected"), 0o700)
			case "directoryLink":
				err = os.Symlink(outside, filepath.Join(dir, bad))
			default:
				if err = os.Mkdir(filepath.Join(dir, bad), 0o700); err != nil {
					t.Fatal(err)
				}
				if kind == "archiveLink" {
					err = os.Symlink(sentinel, filepath.Join(dir, bad, "package.zip"))
				} else {
					err = os.WriteFile(filepath.Join(dir, bad, "package.zip"), []byte("bad"), 0o600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.List(ctx)
			if err == nil || len(got) != 1 || got[0].Digest() != p.Digest() {
				t.Fatalf("partial %d %v", len(got), err)
			}
			if kind == "directoryLink" {
				if err = s.Remove(ctx, bad); err == nil {
					t.Fatal("removed symlink")
				}
			}
			if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "keep" {
				t.Fatal("outside touched", err)
			}
		})
	}
}

func TestStoreCatalogLimitAndIdempotentInstall(t *testing.T) {
	s, _ := catalogStore(t)
	ctx := context.Background()
	for i := 0; i < 64; i++ {
		if _, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, i))); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, 0))); err != nil {
		t.Fatal("idempotent at capacity", err)
	}
	if _, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, 64))); err == nil {
		t.Fatal("65th package admitted")
	}
	got, err := s.List(ctx)
	if err != nil || len(got) != 64 {
		t.Fatal(len(got), err)
	}
}

func TestStoreCatalogContextInvalidAndClose(t *testing.T) {
	s, _ := catalogStore(t)
	ctx := context.Background()
	p, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, 0)))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.List(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = s.Remove(cancelled, p.Digest()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = s.Get(p.Digest()); err != nil {
		t.Fatal("cancel removed", err)
	}
	//nolint:staticcheck // Deliberately exercise invalid public input.
	if _, err = s.List(nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, digest := range []string{"", "../outside", strings.Repeat("A", 64)} {
		if err = s.Remove(ctx, digest); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	//nolint:staticcheck // Deliberately exercise invalid public input.
	if err = s.Remove(nil, p.Digest()); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.List(ctx); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if err = s.Remove(ctx, p.Digest()); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
}

func TestStoreCatalogCancellationAfterWaitingForAdmission(t *testing.T) {
	s, _ := catalogStore(t)
	p, err := s.Install(context.Background(), bytes.NewReader(catalogArchive(t, 0)))
	if err != nil {
		t.Fatal(err)
	}
	for _, remove := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		done := make(chan error, 1)
		go func() {
			if remove {
				done <- s.Remove(ctx, p.Digest())
			} else {
				_, e := s.List(ctx)
				done <- e
			}
		}()
		cancel()
		s.mu.Unlock()
		if err = <-done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if _, err = s.Get(p.Digest()); err != nil {
		t.Fatal("cancelled removal mutated store", err)
	}
}

func TestStoreCatalogBoundsAbandonedStages(t *testing.T) {
	s, dir := catalogStore(t)
	for i := 0; i < 129; i++ {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprintf(".stage-%032x", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.List(context.Background()); err == nil || len(got) != 0 {
		t.Fatal("unbounded stage enumeration", err)
	}
}

func TestStoreRemovalDoesNotFollowNestedLink(t *testing.T) {
	s, dir := catalogStore(t)
	ctx := context.Background()
	p, err := s.Install(ctx, bytes.NewReader(catalogArchive(t, 0)))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	target := filepath.Join(outside, "keep")
	if err = os.WriteFile(target, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(dir, p.Digest(), "foreign")); err != nil {
		t.Fatal(err)
	}
	if err = s.Remove(ctx, p.Digest()); err != nil {
		t.Fatal(err)
	}
	if raw, e := os.ReadFile(target); e != nil || string(raw) != "kept" {
		t.Fatal("followed nested link", e)
	}
}

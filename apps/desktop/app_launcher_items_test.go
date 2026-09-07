package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type launcherReferenceFixture struct {
	mu      sync.Mutex
	records map[string]platform.LauncherReference
	choose  func(context.Context, string) (platform.LauncherReference, error)
	removed []string
	closed  bool
	resolve func(context.Context, string) (platform.LauncherReference, error)
}

func (s *launcherReferenceFixture) ChooseLauncherReference(ctx context.Context, kind string) (platform.LauncherReference, error) {
	return s.choose(ctx, kind)
}

func (s *launcherReferenceFixture) ResolveLauncherReference(ctx context.Context, id string) (platform.LauncherReference, error) {
	if s.resolve != nil {
		return s.resolve(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return r, errors.New("missing")
	}
	return r, nil
}

func (s *launcherReferenceFixture) RelinkLauncherReference(ctx context.Context, id string) (platform.LauncherReference, error) {
	return s.ResolveLauncherReference(ctx, id)
}

func (s *launcherReferenceFixture) RemoveLauncherReference(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, id)
	s.removed = append(s.removed, id)
	return nil
}

func (s *launcherReferenceFixture) PerformLauncherItem(context.Context, platform.LauncherItemAction, func() error) error {
	panic("implicit action")
}

func (s *launcherReferenceFixture) WithLauncherFolder(context.Context, string, uint64, func(platform.FolderRef) error) error {
	panic("implicit folder")
}

func (s *launcherReferenceFixture) OpenLauncherLink(context.Context, string, string, uint64, func() error) error {
	panic("implicit link")
}

func (s *launcherReferenceFixture) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func launcherItemsApp(t *testing.T, source *launcherReferenceFixture) *App {
	t.Helper()
	a := newApp(fake.New(), config.Default(), "")
	a.wireLauncherItems(source, nil)
	a.viewMu.Lock()
	a.prefsOpen = true
	a.syncLauncherItemAdmissionLocked()
	a.viewMu.Unlock()
	a.startLauncherItems()
	pollUntil(t, time.Second, "launcher references load", func() bool { return !a.GetLauncherItemStatus().Busy })
	t.Cleanup(func() {
		a.viewMu.Lock()
		m := a.launcherItems
		a.stopLauncherItemsLocked()
		a.viewMu.Unlock()
		select {
		case <-m.done:
		case <-time.After(time.Second):
			t.Error("item manager did not join")
		}
		a.stopCapture()
	})
	return a
}

func TestLauncherItemsCASAndCopiedSettings(t *testing.T) {
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}}
	a := launcherItemsApp(t, source)
	initial, err := a.GetLauncherItemSettings("default")
	if err != nil {
		t.Fatal(err)
	}
	first := []config.LauncherItem{{ID: "site", Kind: "link", Label: "Site", URL: "https://example.com/"}}
	next, err := a.SetLauncherItems("default", initial.Revision, first)
	if err != nil {
		t.Fatal(err)
	}
	first[0].Label = "mutated"
	if next.Items[0].Label != "Site" {
		t.Fatal("shared input")
	}
	next.Items[0].Label = "mutated"
	if _, err = a.SetLauncherItems("default", initial.Revision, nil); err == nil {
		t.Fatal("stale overwrite")
	}
	current, err := a.GetLauncherItemSettings("default")
	if err != nil || current.Items[0].Label != "Site" {
		t.Fatal("copy", err)
	}
	if launcherItemsRevision(nil) != launcherItemsRevision([]config.LauncherItem{}) {
		t.Fatal("nil/empty hash mismatch")
	}
}

func TestLauncherReferenceSelectionDoesNotSaveOrOpen(t *testing.T) {
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}}
	source.choose = func(context.Context, string) (platform.LauncherReference, error) {
		r := platform.LauncherReference{ID: id, Kind: "app", Label: "Fixture", BundleID: "org.example.fixture", State: "ready", Revision: 1}
		source.mu.Lock()
		source.records[id] = r
		source.mu.Unlock()
		return r, nil
	}
	a := launcherItemsApp(t, source)
	before := a.GetSettings()
	r, err := a.ChooseLauncherItemReference("app")
	if err != nil || r.ID != id {
		t.Fatal(r, err)
	}
	if a.GetSettings() != before {
		t.Fatal("selection saved profile")
	}
	if err = a.RemoveUnusedLauncherReference(id); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.removed) != 1 {
		t.Fatal("private selection not cleaned")
	}
}

func TestLauncherReferenceCancelledChooserRemainsBusyUntilCleanup(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}, choose: func(ctx context.Context, _ string) (platform.LauncherReference, error) {
		close(started)
		<-ctx.Done()
		<-release
		return platform.LauncherReference{}, ctx.Err()
	}}
	a := launcherItemsApp(t, source)
	done := make(chan error, 1)
	go func() { _, err := a.ChooseLauncherItemReference("app"); done <- err }()
	<-started
	if err := a.CancelLauncherItemSelection(); err != nil {
		t.Fatal(err)
	}
	if !a.GetLauncherItemStatus().Busy {
		t.Fatal("released busy before source cleanup")
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("cancelled result accepted")
	}
}

func TestLauncherItemsCleanupOnlyAfterSaveAndAcrossAllProfiles(t *testing.T) {
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{id: {ID: id, Kind: "app", State: "ready"}}}
	a := launcherItemsApp(t, source)
	s := a.settingsSnapshot()
	item := config.LauncherItem{ID: "fixture", Kind: "app", Label: "Fixture", ReferenceID: id}
	s.ReplacementDock.Profiles[0].Items = []config.LauncherItem{item}
	other := s.ReplacementDock.Profiles[0]
	other.ID = "other"
	other.Name = "Other"
	s.ReplacementDock.Profiles = append(s.ReplacementDock.Profiles, other)
	a.saveMu.Lock()
	err := a.saveSettingsLocked(s)
	a.saveMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	initial, err := a.GetLauncherItemSettings("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.SetLauncherItems("default", initial.Revision, nil); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	n := len(source.removed)
	source.mu.Unlock()
	if n != 0 {
		t.Fatal("removed shared reference")
	}
	second, err := a.GetLauncherItemSettings("other")
	if err != nil {
		t.Fatal(err)
	}
	a.settingsPath = t.TempDir()
	if _, err = a.SetLauncherItems("other", second.Revision, nil); err == nil {
		t.Fatal("expected save failure")
	}
	source.mu.Lock()
	n = len(source.removed)
	source.mu.Unlock()
	if n != 0 {
		t.Fatal("cleanup preceded successful persistence")
	}
	a.settingsPath = ""
	if _, err = a.SetLauncherItems("other", second.Revision, nil); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.removed) != 1 || source.removed[0] != id {
		t.Fatal("last reference not cleaned")
	}
}

func TestLauncherReferenceCommittedAfterPrefsRetirementStaysInertAndRetryable(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	id := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}}
	source.choose = func(context.Context, string) (platform.LauncherReference, error) {
		close(started)
		<-release
		r := platform.LauncherReference{ID: id, Kind: "file", State: "ready"}
		source.mu.Lock()
		source.records[id] = r
		source.mu.Unlock()
		return r, context.Canceled
	}
	a := launcherItemsApp(t, source)
	before := a.GetSettings()
	done := make(chan error, 1)
	go func() { _, err := a.ChooseLauncherItemReference("file"); done <- err }()
	<-started
	a.viewMu.Lock()
	a.prefsOpen = false
	a.syncLauncherItemAdmissionLocked()
	a.prefsOpen = true
	a.syncLauncherItemAdmissionLocked()
	a.viewMu.Unlock()
	close(release)
	if err := <-done; err == nil {
		t.Fatal("old chooser accepted after close/reopen")
	}
	if a.GetSettings() != before {
		t.Fatal("committed bookmark changed settings")
	}
	settings, err := a.GetLauncherItemSettings("default")
	if err != nil || len(settings.References) != 1 || settings.References[0].ID != id {
		t.Fatal("committed inert metadata missing", settings, err)
	}
	if err = a.RemoveUnusedLauncherReference(id); err != nil {
		t.Fatal(err)
	}
}

func TestLauncherReferenceOldManagerCannotPublishIntoReplacement(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}, choose: func(context.Context, string) (platform.LauncherReference, error) {
		close(started)
		<-release
		return platform.LauncherReference{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "app", State: "ready"}, nil
	}}
	a := launcherItemsApp(t, source)
	done := make(chan error, 1)
	go func() { _, err := a.ChooseLauncherItemReference("app"); done <- err }()
	<-started
	a.viewMu.Lock()
	old := a.launcherItems
	a.viewMu.Unlock()
	replacement := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}}
	a.wireLauncherItems(replacement, nil)
	a.startLauncherItems()
	close(release)
	if err := <-done; err == nil {
		t.Fatal("replaced owner accepted result")
	}
	select {
	case <-old.done:
	case <-time.After(time.Second):
		t.Fatal("old source not joined")
	}
	pollUntil(t, time.Second, "replacement load", func() bool { return !a.GetLauncherItemStatus().Busy })
	settings, err := a.GetLauncherItemSettings("default")
	if err != nil || len(settings.References) != 0 {
		t.Fatal("old catalog published", settings, err)
	}
}

func TestLauncherReferenceRuntimeReadIndependentOfPrefsAndJoined(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	source := &launcherReferenceFixture{records: map[string]platform.LauncherReference{}, resolve: func(ctx context.Context, _ string) (platform.LauncherReference, error) {
		close(started)
		<-ctx.Done()
		<-release
		return platform.LauncherReference{ID: id, Kind: "app"}, nil
	}}
	a := launcherItemsApp(t, source)
	a.viewMu.Lock()
	a.prefsOpen = false
	a.syncLauncherItemAdmissionLocked()
	m := a.launcherItems
	a.viewMu.Unlock()
	done := make(chan error, 1)
	go func() { _, err := a.resolveLauncherItemReference(context.Background(), id); done <- err }()
	<-started
	a.viewMu.Lock()
	a.stopLauncherItemsLocked()
	a.viewMu.Unlock()
	select {
	case <-m.done:
		t.Fatal("source closed before read cleanup")
	default:
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("late read accepted after shutdown")
	}
	select {
	case <-m.done:
	case <-time.After(time.Second):
		t.Fatal("source read failed to join")
	}
}

func TestLauncherItemCompletionPublishesStatus(t *testing.T) {
	a := launcherItemsApp(t, &launcherReferenceFixture{records: map[string]platform.LauncherReference{}})
	events := make(chan LauncherItemStatus, 4)
	a.viewMu.Lock()
	a.eventSink = func(name string, value any) {
		if name == "launcher:items" {
			events <- value.(LauncherItemStatus)
		}
	}
	a.viewMu.Unlock()
	initial, err := a.GetLauncherItemSettings("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.SetLauncherItems("default", initial.Revision, []config.LauncherItem{{ID: "space", Kind: "spacer"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-events:
		if s.Busy || !s.Available {
			t.Fatal("wrong completion", s)
		}
	case <-time.After(time.Second):
		t.Fatal("no completion")
	}
	a.viewMu.Lock()
	a.eventSink = nil
	a.viewMu.Unlock()
}

type launcherIconFixture struct {
	data         []byte
	id           string
	selectionErr error
	removed      int
}

func (s *launcherIconFixture) ChooseLauncherIcon(context.Context) (string, error) {
	return s.id, s.selectionErr
}

func (s *launcherIconFixture) LauncherIcon(context.Context, string) ([]byte, error) {
	return append([]byte{}, s.data...), nil
}

func (s *launcherIconFixture) RemoveLauncherIcon(context.Context, string) error {
	s.removed++
	return nil
}
func (s *launcherIconFixture) Close() error { return nil }
func TestLauncherItemIconCommittedErrorCatalogAndRuntimeCopy(t *testing.T) {
	a := launcherItemsApp(t, &launcherReferenceFixture{records: map[string]platform.LauncherReference{}})
	data := []byte("normalized-source-png-fixture")
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	icons := &launcherIconFixture{data: data, id: id, selectionErr: context.Canceled}
	a.viewMu.Lock()
	a.launcherItems.icons = icons
	a.viewMu.Unlock()
	if _, err := a.ChooseLauncherItemIcon(); !errors.Is(err, context.Canceled) {
		t.Fatal("selection refusal lost", err)
	}
	state, err := a.GetLauncherItemSettings("default")
	if err != nil || len(state.IconIDs) != 1 || state.IconIDs[0] != id {
		t.Fatal("committed icon undiscoverable", state, err)
	}
	a.viewMu.Lock()
	a.prefsOpen = false
	a.syncLauncherItemAdmissionLocked()
	a.viewMu.Unlock()
	result, err := a.readLauncherItemIcon(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	result[0] = 'X'
	a.viewMu.Lock()
	cached := append([]byte{}, a.launcherItems.images[id]...)
	a.prefsOpen = true
	a.syncLauncherItemAdmissionLocked()
	a.viewMu.Unlock()
	if !bytes.Equal(cached, data) {
		t.Fatal("shared runtime asset")
	}
	if err = a.RemoveUnusedLauncherIcon(id); err != nil || icons.removed != 1 {
		t.Fatal("inert icon removal", err)
	}
}

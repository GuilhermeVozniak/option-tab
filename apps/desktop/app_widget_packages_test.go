package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/widgets"
)

type widgetPackageChooser func(context.Context) (platform.WidgetPackageFile, error)

func (f widgetPackageChooser) ChooseWidgetPackage(ctx context.Context) (platform.WidgetPackageFile, error) {
	return f(ctx)
}

func widgetPackageArchive(t *testing.T, version, minimum string) []byte {
	t.Helper()
	m := widgets.Manifest{SchemaVersion: 1, ID: "org.example.fixture", Version: version, MinimumAppVersion: minimum, Name: widgets.Localized{"en": "Fixture"}, Description: widgets.Localized{"en": "Original inert fixture"}, RequiredCapabilities: []string{"clock.read"}, Root: widgets.Node{Kind: "text", Binding: &widgets.Binding{Provider: "clock", Field: "time", Formatter: "shortTime"}}}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, err := z.Create("widget.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func widgetPackageApp(t *testing.T, source platform.WidgetPackageSource) (*App, string) {
	t.Helper()
	a := newApp(fake.New(), config.Default(), "")
	a.wireWidgets(widgets.Providers{})
	dir := t.TempDir()
	if err := a.wireWidgetPackages(source, dir); err != nil {
		t.Fatal(err)
	}
	a.viewMu.Lock()
	a.prefsOpen = true
	a.syncWidgetPackageAdmissionLocked()
	a.viewMu.Unlock()
	a.startWidgetPackages()
	pollUntil(t, time.Second, "catalog load", func() bool { return !a.GetWidgetPackageStatus().Busy })
	t.Cleanup(func() {
		a.viewMu.Lock()
		a.stopWidgetPackagesLocked()
		m := a.widgetPackages
		a.viewMu.Unlock()
		select {
		case <-m.done:
		case <-time.After(time.Second):
			t.Error("manager did not join")
		}
		a.stopCapture()
	})
	return a, dir
}

func TestWidgetPackageReviewInstallExactInertSnapshot(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, dir := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "fixture.zip", Archive: raw}, nil
	}))
	review, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	if entries, e := os.ReadDir(dir); e != nil || len(entries) != 0 {
		t.Fatal("review wrote", e)
	}
	for i := range raw {
		raw[i] = 0
	}
	review.Package.Name["en"] = "changed"
	installed, err := a.InstallReviewedWidget(review.Token)
	if err != nil || installed.Name["en"] != "Fixture" {
		t.Fatal("snapshot changed", err)
	}
	if _, err = a.InstallReviewedWidget(review.Token); err == nil {
		t.Fatal("token replay")
	}
	for _, profile := range a.settingsSnapshot().ReplacementDock.Profiles {
		for _, w := range profile.Widgets {
			if w.Digest == installed.Digest && (w.Enabled || len(w.Grants) > 0) {
				t.Fatal("install granted authority")
			}
		}
	}
	if _, err = os.Stat(filepath.Join(dir, installed.Digest, "package.zip")); err != nil {
		t.Fatal(err)
	}
}

func TestWidgetPackageReviewExpiryAndPreferenceRetirement(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	now := time.Now()
	a.widgetPackages.now = func() time.Time { return now }
	review, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Minute)
	if _, err = a.InstallReviewedWidget(review.Token); err == nil {
		t.Fatal("expired accepted")
	}
	review, err = a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	a.viewMu.Lock()
	a.prefsOpen = false
	a.syncWidgetPackageAdmissionLocked()
	a.prefsOpen = true
	a.syncWidgetPackageAdmissionLocked()
	a.viewMu.Unlock()
	if _, err = a.InstallReviewedWidget(review.Token); err == nil {
		t.Fatal("close/reopen revived review")
	}
}

func TestWidgetPackageMinimumVersion(t *testing.T) {
	for _, v := range []string{"0.4.8-beta.1", "0.4.9", "999999999999999999999999999999999999999.0.0"} {
		t.Run(v, func(t *testing.T) {
			raw := widgetPackageArchive(t, "1.0.0", v)
			a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
				return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
			}))
			_, err := a.ReviewLocalWidgetPackage()
			if (v == "0.4.8-beta.1") != (err == nil) {
				t.Fatal("compatibility", err)
			}
		})
	}
}

func TestWidgetPackageChooserCancellationRetainsBusyUntilJoin(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(ctx context.Context) (platform.WidgetPackageFile, error) {
		close(started)
		<-ctx.Done()
		<-release
		return platform.WidgetPackageFile{}, ctx.Err()
	}))
	result := make(chan error, 1)
	go func() { _, err := a.ReviewLocalWidgetPackage(); result <- err }()
	<-started
	a.viewMu.Lock()
	a.prefsOpen = false
	a.syncWidgetPackageAdmissionLocked()
	a.prefsOpen = true
	a.syncWidgetPackageAdmissionLocked()
	a.viewMu.Unlock()
	if !a.GetWidgetPackageStatus().Busy {
		t.Fatal("busy released before native join")
	}
	if _, err := a.ReviewLocalWidgetPackage(); err == nil {
		t.Fatal("second chooser")
	}
	close(release)
	if err := <-result; err == nil {
		t.Fatal("cancelled chooser accepted")
	}
}

func installWidgetFixture(t *testing.T, a *App) WidgetCatalogItem {
	t.Helper()
	r, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.InstallReviewedWidget(r.Token)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWidgetPackageRemovalClearsOnlyExactDigest(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, dir := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	first := installWidgetFixture(t, a)
	raw = widgetPackageArchive(t, "2.0.0", "0.4.8")
	second := installWidgetFixture(t, a)
	saveWidgetFixture(t, a, []config.WidgetInstance{{ID: "first", PackageID: first.PackageID, Digest: first.Digest, Enabled: true, Grants: []string{"clock.read"}}, {ID: "second", PackageID: second.PackageID, Digest: second.Digest, Enabled: true, Grants: []string{"clock.read"}}})
	if err := a.RemoveWidgetPackage(first.Digest); err != nil {
		t.Fatal(err)
	}
	refs := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets
	if refs[0].Enabled || len(refs[0].Grants) > 0 || !refs[1].Enabled || len(refs[1].Grants) != 1 {
		t.Fatalf("wrong references %+v", refs)
	}
	if _, err := os.Stat(filepath.Join(dir, first.Digest)); !os.IsNotExist(err) {
		t.Fatal("archive retained", err)
	}
	if _, err := os.Stat(filepath.Join(dir, second.Digest)); err != nil {
		t.Fatal("other digest removed", err)
	}
	builtin, _ := widgets.Builtin("org.optiontab.clock")
	if err := a.RemoveWidgetPackage(builtin.Digest()); err == nil {
		t.Fatal("builtin removal")
	}
}

func TestWidgetPackageRemovalSaveFailureKeepsInstalled(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, dir := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	p := installWidgetFixture(t, a)
	saveWidgetFixture(t, a, []config.WidgetInstance{{ID: "fixture", PackageID: p.PackageID, Digest: p.Digest, Enabled: true, Grants: []string{"clock.read"}}})
	a.settingsPath = t.TempDir()
	if err := a.RemoveWidgetPackage(p.Digest); err == nil {
		t.Fatal("failed save ignored")
	}
	ref := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
	if !ref.Enabled || len(ref.Grants) != 1 {
		t.Fatal("failed save changed references")
	}
	if _, err := os.Stat(filepath.Join(dir, p.Digest, "package.zip")); err != nil {
		t.Fatal("failed save deleted", err)
	}
	a.viewMu.Lock()
	kept := a.widgets.packages[p.Digest] != nil
	a.viewMu.Unlock()
	if !kept {
		t.Fatal("failed save removed cache")
	}
}

func TestWidgetPackageInstallMissingReferencesAndExistingIdempotence(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	pkg, err := widgets.Preview(context.Background(), bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	ref := config.WidgetInstance{ID: "fixture", PackageID: pkg.Manifest().ID, Digest: pkg.Digest(), Enabled: true, Grants: []string{"clock.read"}}
	saveWidgetFixture(t, a, []config.WidgetInstance{ref})
	p := installWidgetFixture(t, a)
	got := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
	if got.Enabled || len(got.Grants) != 0 {
		t.Fatal("missing package install revived grants")
	}
	saveWidgetFixture(t, a, []config.WidgetInstance{ref})
	r, err := a.ReviewLocalWidgetPackage()
	if err != nil || !r.AlreadyInstalled {
		t.Fatal("not recognized", err)
	}
	if _, err = a.InstallReviewedWidget(r.Token); err != nil {
		t.Fatal(err)
	}
	got = a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
	if !got.Enabled || len(got.Grants) != 1 || got.Digest != p.Digest {
		t.Fatal("idempotence cleared grants")
	}
}

func TestWidgetPackageReviewReplacementAndCancellation(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	first, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.InstallReviewedWidget(first.Token); err == nil {
		t.Fatal("old review accepted")
	}
	// A wrong token must not erase the newer review.
	if err = a.CancelWidgetPackageReview(second.Token); err != nil {
		t.Fatal("old-token replay erased current review", err)
	}
	if _, err = a.InstallReviewedWidget(second.Token); err == nil {
		t.Fatal("cancelled accepted")
	}
}

func TestWidgetPackageStartupPreservesSavedGrants(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	dir := t.TempDir()
	store, err := widgets.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.Install(context.Background(), bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	a := newApp(fake.New(), config.Default(), "")
	a.wireWidgets(widgets.Providers{})
	saveWidgetFixture(t, a, []config.WidgetInstance{{ID: "fixture", PackageID: p.Manifest().ID, Digest: p.Digest(), Enabled: true, Grants: []string{"clock.read"}}})
	if err = a.wireWidgetPackages(nil, dir); err != nil {
		t.Fatal(err)
	}
	a.startWidgetPackages()
	pollUntil(t, time.Second, "startup catalog", func() bool { return !a.GetWidgetPackageStatus().Busy })
	a.viewMu.Lock()
	loaded := a.widgets.packages[p.Digest()] != nil
	a.stopWidgetPackagesLocked()
	done := a.widgetPackages.done
	a.viewMu.Unlock()
	<-done
	a.stopCapture()
	if !loaded {
		t.Fatal("startup missing package")
	}
	ref := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
	if !ref.Enabled || len(ref.Grants) != 1 {
		t.Fatal("startup erased explicit grants")
	}
}

func TestWidgetPackageShutdownWaitsForChooserCleanup(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(ctx context.Context) (platform.WidgetPackageFile, error) {
		close(started)
		<-ctx.Done()
		<-release
		return platform.WidgetPackageFile{}, ctx.Err()
	}))
	result := make(chan error, 1)
	go func() { _, err := a.ReviewLocalWidgetPackage(); result <- err }()
	<-started
	a.viewMu.Lock()
	a.stopWidgetPackagesLocked()
	done := a.widgetPackages.done
	a.viewMu.Unlock()
	select {
	case <-done:
		t.Fatal("store closed before native cleanup")
	default:
	}
	close(release)
	if err := <-result; err == nil {
		t.Fatal("shutdown returned success")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not join")
	}
}

func TestWidgetPackageInstallSaveFailureLeavesNoArchive(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	p, err := widgets.Preview(context.Background(), bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	a, dir := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	saveWidgetFixture(t, a, []config.WidgetInstance{{ID: "fixture", PackageID: p.Manifest().ID, Digest: p.Digest(), Enabled: true, Grants: []string{"clock.read"}}})
	review, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	a.settingsPath = t.TempDir()
	if _, err = a.InstallReviewedWidget(review.Token); err == nil {
		t.Fatal("save failure ignored")
	}
	if _, err = os.Stat(filepath.Join(dir, p.Digest())); !os.IsNotExist(err) {
		t.Fatal("archive published after save failure", err)
	}
}

func TestWidgetPackageInstallExpiryWhileWaitingForSettings(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, dir := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	review, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	a.saveMu.Lock()
	done := make(chan error, 1)
	go func() { _, err := a.InstallReviewedWidget(review.Token); done <- err }()
	pollUntil(t, time.Second, "install admission", func() bool { return a.GetWidgetPackageStatus().Busy })
	a.viewMu.Lock()
	a.widgetPackages.now = func() time.Time { return review.ExpiresAt.Add(time.Second) }
	a.viewMu.Unlock()
	a.saveMu.Unlock()
	if err := <-done; err == nil {
		t.Fatal("expired queued install accepted")
	}
	if entries, e := os.ReadDir(dir); e != nil || len(entries) != 0 {
		t.Fatal("expired install wrote", e)
	}
}

func TestWidgetPackageCompletionPublishesCurrentStatus(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a := newApp(fake.New(), config.Default(), "")
	a.wireWidgets(widgets.Providers{})
	events := make(chan WidgetPackageStatus, 16)
	a.eventSink = func(name string, data any) {
		if name == "widgets:packages" {
			events <- data.(WidgetPackageStatus)
		}
	}
	if err := a.wireWidgetPackages(widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	a.viewMu.Lock()
	a.prefsOpen = true
	a.syncWidgetPackageAdmissionLocked()
	a.viewMu.Unlock()
	for len(events) > 0 {
		<-events
	}
	a.startWidgetPackages()
	select {
	case status := <-events:
		if status.Busy || !status.Available {
			t.Fatalf("completion %+v", status)
		}
	case <-time.After(time.Second):
		t.Fatal("no catalog completion event")
	}
	review, err := a.ReviewLocalWidgetPackage()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-events:
		if status.Busy {
			t.Fatal("review completion busy")
		}
	default:
		t.Fatal("no review completion event")
	}
	if err = a.CancelWidgetPackageReview(review.Token); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	default:
		t.Fatal("no cancellation event")
	}
	a.viewMu.Lock()
	a.prefsOpen = false
	a.syncWidgetPackageAdmissionLocked()
	a.viewMu.Unlock()
	select {
	case status := <-events:
		if status.Available {
			t.Fatal("retirement remains available")
		}
	default:
		t.Fatal("no admission event")
	}
	a.viewMu.Lock()
	a.stopWidgetPackagesLocked()
	done := a.widgetPackages.done
	a.viewMu.Unlock()
	<-done
	a.stopCapture()
}

func TestWidgetPackageOldManagerCannotPublishCompletion(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(ctx context.Context) (platform.WidgetPackageFile, error) {
		close(started)
		<-ctx.Done()
		<-release
		return platform.WidgetPackageFile{}, ctx.Err()
	}))
	events := make(chan WidgetPackageStatus, 16)
	a.viewMu.Lock()
	a.eventSink = func(name string, data any) {
		if name == "widgets:packages" {
			events <- data.(WidgetPackageStatus)
		}
	}
	old := a.widgetPackages
	a.viewMu.Unlock()
	result := make(chan error, 1)
	go func() { _, err := a.ReviewLocalWidgetPackage(); result <- err }()
	<-started
	if err := a.wireWidgetPackages(nil, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for len(events) > 0 {
		<-events
	}
	close(release)
	<-result
	<-old.done
	select {
	case status := <-events:
		t.Fatalf("old manager published %+v", status)
	default:
	}
}

type widgetPackageRemovalStore struct {
	widgetPackageStore
	remove func(context.Context, string) error
}

func (s widgetPackageRemovalStore) Remove(ctx context.Context, digest string) error {
	return s.remove(ctx, digest)
}

func TestWidgetPackageRemovalFailureRetainsRevokedCatalogForRetry(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	p := installWidgetFixture(t, a)
	saveWidgetFixture(t, a, []config.WidgetInstance{{ID: "fixture", PackageID: p.PackageID, Digest: p.Digest, Enabled: true, Grants: []string{"clock.read"}}})
	original := a.widgetPackages.store
	first := true
	a.widgetPackages.store = widgetPackageRemovalStore{widgetPackageStore: original, remove: func(ctx context.Context, digest string) error {
		if first {
			first = false
			return os.ErrPermission
		}
		return original.Remove(ctx, digest)
	}}
	if err := a.RemoveWidgetPackage(p.Digest); err == nil {
		t.Fatal("failure hidden")
	}
	ref := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
	if ref.Enabled || len(ref.Grants) > 0 {
		t.Fatal("authority restored")
	}
	a.viewMu.Lock()
	retained := a.widgets.packages[p.Digest] != nil
	a.viewMu.Unlock()
	if !retained {
		t.Fatal("failed deletion disappeared from catalog")
	}
	if err := a.RemoveWidgetPackage(p.Digest); err != nil {
		t.Fatal("retry failed", err)
	}
	a.viewMu.Lock()
	retained = a.widgets.packages[p.Digest] != nil
	a.viewMu.Unlock()
	if retained {
		t.Fatal("successful retry retained catalog")
	}
}

func TestWidgetPackageRemovalAlreadyAbsentIsSuccess(t *testing.T) {
	raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
	a, dir := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
		return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
	}))
	p := installWidgetFixture(t, a)
	if err := os.RemoveAll(filepath.Join(dir, p.Digest)); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveWidgetPackage(p.Digest); err != nil {
		t.Fatal("already absent refused", err)
	}
}

func TestWidgetPackageCancelledRemovalRetainsOnlyCurrentOwner(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(strconv.FormatBool(replace), func(t *testing.T) {
			raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
			a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
				return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
			}))
			p := installWidgetFixture(t, a)
			started, release := make(chan struct{}), make(chan struct{})
			original := a.widgetPackages.store
			a.widgetPackages.store = widgetPackageRemovalStore{widgetPackageStore: original, remove: func(ctx context.Context, _ string) error { close(started); <-ctx.Done(); <-release; return ctx.Err() }}
			done := make(chan error, 1)
			go func() { done <- a.RemoveWidgetPackage(p.Digest) }()
			<-started
			if replace {
				if err := a.wireWidgetPackages(nil, t.TempDir()); err != nil {
					t.Fatal(err)
				}
			} else {
				a.viewMu.Lock()
				a.prefsOpen = false
				a.syncWidgetPackageAdmissionLocked()
				a.viewMu.Unlock()
			}
			close(release)
			if err := <-done; err == nil {
				t.Fatal("cancel accepted")
			}
			a.viewMu.Lock()
			retained := a.widgets.packages[p.Digest] != nil
			a.viewMu.Unlock()
			if retained == replace {
				t.Fatal("incorrect owner restore", replace, retained)
			}
		})
	}
}

type widgetPackageCommitBarrierStore struct {
	widgetPackageStore
	committed, release chan struct{}
}

func (s widgetPackageCommitBarrierStore) Install(ctx context.Context, r io.Reader) (*widgets.Package, error) {
	p, err := s.widgetPackageStore.Install(ctx, r)
	if err != nil {
		return nil, err
	}
	close(s.committed)
	<-s.release
	return p, nil
}

func TestWidgetPackageCommittedInstallReconcilesOnlyCurrentOwner(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(strconv.FormatBool(replace), func(t *testing.T) {
			raw := widgetPackageArchive(t, "1.0.0", "0.4.8")
			a, _ := widgetPackageApp(t, widgetPackageChooser(func(context.Context) (platform.WidgetPackageFile, error) {
				return platform.WidgetPackageFile{Name: "f.zip", Archive: raw}, nil
			}))
			review, err := a.ReviewLocalWidgetPackage()
			if err != nil {
				t.Fatal(err)
			}
			saveWidgetFixture(t, a, []config.WidgetInstance{{ID: "fixture", PackageID: review.Package.PackageID, Digest: review.Package.Digest, Enabled: true, Grants: []string{"clock.read"}}})
			committed, release := make(chan struct{}), make(chan struct{})
			a.widgetPackages.store = widgetPackageCommitBarrierStore{widgetPackageStore: a.widgetPackages.store, committed: committed, release: release}
			done := make(chan error, 1)
			go func() { _, err := a.InstallReviewedWidget(review.Token); done <- err }()
			<-committed
			if replace {
				if err := a.wireWidgetPackages(nil, t.TempDir()); err != nil {
					t.Fatal(err)
				}
			} else {
				a.viewMu.Lock()
				a.prefsOpen = false
				a.syncWidgetPackageAdmissionLocked()
				a.prefsOpen = true
				a.syncWidgetPackageAdmissionLocked()
				a.viewMu.Unlock()
			}
			close(release)
			if err := <-done; err == nil {
				t.Fatal("stale UI install success")
			}
			a.viewMu.Lock()
			retained := a.widgets.packages[review.Package.Digest] != nil
			a.viewMu.Unlock()
			if retained == replace {
				t.Fatal("committed catalog owner mismatch", replace, retained)
			}
			ref := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
			if ref.Enabled || len(ref.Grants) > 0 {
				t.Fatal("commit restored authority")
			}
		})
	}
}

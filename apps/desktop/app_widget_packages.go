package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"option-tab/internal/platform"
	"option-tab/internal/update"
	"option-tab/internal/widgets"
)

type WidgetPackageReview struct {
	Token            string            `json:"token"`
	SourceName       string            `json:"sourceName"`
	Package          WidgetCatalogItem `json:"package"`
	ExpiresAt        time.Time         `json:"expiresAt"`
	AlreadyInstalled bool              `json:"alreadyInstalled"`
}
type WidgetPackageStatus struct {
	Available bool   `json:"available"`
	Busy      bool   `json:"busy"`
	Reason    string `json:"reason"`
}
type widgetPackageReview struct {
	token, name string
	archive     []byte
	pkg         *widgets.Package
	expires     time.Time
}
type widgetPackageJob struct {
	ctx    context.Context
	cancel context.CancelFunc
	epoch  uint64
	kind   string
}

// All admission fields are protected by App.viewMu; Store owns disk serialization.
// No job releases busy until its native/filesystem work has returned.
type widgetPackageStore interface {
	List(context.Context) ([]*widgets.Package, error)
	Install(context.Context, io.Reader) (*widgets.Package, error)
	Remove(context.Context, string) error
	Close() error
}

type appWidgetPackageManager struct {
	source                   platform.WidgetPackageSource
	store                    widgetPackageStore
	ctx                      context.Context
	cancel                   context.CancelFunc
	done                     chan struct{}
	wg                       sync.WaitGroup
	started, closed, allowed bool
	epoch                    uint64
	job                      *widgetPackageJob
	review                   *widgetPackageReview
	reason                   string
	now                      func() time.Time
}

func widgetPackageError(code string) error { return errors.New("widget package: " + code) }
func (a *App) wireWidgetPackages(source platform.WidgetPackageSource, path string) error {
	store, err := widgets.OpenStore(path)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &appWidgetPackageManager{source: source, store: store, ctx: ctx, cancel: cancel, done: make(chan struct{}), now: time.Now}
	a.viewMu.Lock()
	if a.widgetPackages != nil {
		a.stopWidgetPackagesLocked()
	}
	a.widgetPackages = m
	a.syncWidgetPackageAdmissionLocked()
	a.viewMu.Unlock()
	return nil
}

func (a *App) widgetPackagesAllowedLocked() bool {
	if !a.prefsOpen || a.sessionInactive || a.widgets == nil || a.widgets.ctx.Err() != nil {
		return false
	}
	select {
	case <-a.captureStop:
		return false
	default:
		return true
	}
}

func (a *App) syncWidgetPackageAdmissionLocked() {
	m := a.widgetPackages
	if m == nil {
		return
	}
	allowed := !m.closed && a.widgetPackagesAllowedLocked()
	if allowed != m.allowed {
		m.allowed = allowed
		m.epoch++
		m.review = nil
		if !allowed && m.job != nil && m.job.kind != "load" {
			m.job.cancel()
		}
		a.publishWidgetPackagesLocked(m)
	}
}

func (a *App) stopWidgetPackagesLocked() {
	m := a.widgetPackages
	if m == nil || m.closed {
		return
	}
	m.closed = true
	m.allowed = false
	m.epoch++
	m.review = nil
	m.cancel()
	if m.job != nil {
		m.job.cancel()
	}
	a.publishWidgetPackagesLocked(m)
	go func() { m.wg.Wait(); _ = m.store.Close(); close(m.done) }()
}

func (a *App) startWidgetPackages() {
	a.viewMu.Lock()
	m := a.widgetPackages
	if m == nil || m.closed || m.started {
		a.viewMu.Unlock()
		return
	}
	m.started = true
	j := a.beginWidgetPackageJobLocked(m, "load")
	a.viewMu.Unlock()
	go func() {
		defer a.finishWidgetPackageJob(m, j)
		packages, err := m.store.List(j.ctx)
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if a.widgetPackages != m || m.closed || j.ctx.Err() != nil || a.widgets == nil {
			return
		}
		for _, p := range packages {
			if widgetPackageSupported(p) {
				a.widgets.packages[p.Digest()] = p
			}
		}
		if err != nil {
			m.reason = "catalogInvalid"
		}
		a.syncWidgetsLocked()
	}()
}

func (a *App) beginWidgetPackageJobLocked(m *appWidgetPackageManager, kind string) *widgetPackageJob {
	ctx, cancel := context.WithCancel(m.ctx)
	j := &widgetPackageJob{ctx: ctx, cancel: cancel, epoch: m.epoch, kind: kind}
	m.job = j
	m.wg.Add(1)
	return j
}

func (a *App) finishWidgetPackageJob(m *appWidgetPackageManager, j *widgetPackageJob) {
	j.cancel()
	a.viewMu.Lock()
	if m.job == j {
		m.job = nil
		if !m.closed {
			a.publishWidgetPackagesLocked(m)
		}
	}
	a.viewMu.Unlock()
	m.wg.Done()
}

func (a *App) currentWidgetPackageJobLocked(m *appWidgetPackageManager, j *widgetPackageJob) bool {
	a.syncWidgetPackageAdmissionLocked()
	return a.widgetPackages == m && !m.closed && m.allowed && m.epoch == j.epoch && m.job == j && j.ctx.Err() == nil
}

func (a *App) widgetPackageAdmissionLocked() (*appWidgetPackageManager, error) {
	a.syncWidgetPackageAdmissionLocked()
	m := a.widgetPackages
	if m == nil || m.closed || !m.started || !m.allowed {
		return nil, widgetPackageError("unavailable")
	}
	if m.job != nil {
		return nil, widgetPackageError("busy")
	}
	return m, nil
}

func (a *App) GetWidgetPackageStatus() WidgetPackageStatus {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncWidgetPackageAdmissionLocked()
	return a.widgetPackageStatusLocked()
}

func (a *App) widgetPackageStatusLocked() WidgetPackageStatus {
	m := a.widgetPackages
	if m == nil || m.closed {
		return WidgetPackageStatus{Reason: "unavailable"}
	}
	return WidgetPackageStatus{Available: m.allowed && m.source != nil, Busy: m.job != nil, Reason: m.reason}
}

// Like other App publication helpers, this uses the bounded, non-reentrant
// event transport under viewMu so replacement cannot race an old publication.
func (a *App) publishWidgetPackagesLocked(m *appWidgetPackageManager) {
	if a.widgetPackages == m {
		a.emit("widgets:packages", a.widgetPackageStatusLocked())
	}
}

// Package already passed strict manifest validation. Reject core version overflow
// explicitly: update.Newer deliberately treats malformed input as not newer.
func widgetPackageSupported(p *widgets.Package) bool {
	if p == nil {
		return false
	}
	v := p.Manifest().MinimumAppVersion
	core := strings.SplitN(strings.SplitN(v, "+", 2)[0], "-", 2)[0]
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return !update.Newer(appVersion, v)
}

func (a *App) ReviewLocalWidgetPackage() (WidgetPackageReview, error) {
	a.viewMu.Lock()
	m, err := a.widgetPackageAdmissionLocked()
	if err != nil {
		a.viewMu.Unlock()
		return WidgetPackageReview{}, err
	}
	if m.source == nil {
		a.viewMu.Unlock()
		return WidgetPackageReview{}, widgetPackageError("unavailable")
	}
	m.review = nil
	m.reason = ""
	j := a.beginWidgetPackageJobLocked(m, "review")
	a.viewMu.Unlock()
	defer a.finishWidgetPackageJob(m, j)
	selected, err := m.source.ChooseWidgetPackage(j.ctx)
	if err != nil {
		return WidgetPackageReview{}, err
	}
	if len(selected.Archive) > widgets.MaxCompressed || selected.Name == "" || len(selected.Name) > 1024 || !utf8.ValidString(selected.Name) || strings.ContainsRune(selected.Name, 0) || filepath.Base(selected.Name) != selected.Name {
		return WidgetPackageReview{}, widgetPackageError("invalidFile")
	}
	raw := append([]byte(nil), selected.Archive...)
	p, err := widgets.Preview(j.ctx, bytes.NewReader(raw))
	if err != nil {
		return WidgetPackageReview{}, widgetPackageError("invalidPackage")
	}
	if !widgetPackageSupported(p) {
		return WidgetPackageReview{}, widgetPackageError("incompatibleVersion")
	}
	var token [24]byte
	if _, err = rand.Read(token[:]); err != nil {
		return WidgetPackageReview{}, widgetPackageError("unavailable")
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if !a.currentWidgetPackageJobLocked(m, j) {
		return WidgetPackageReview{}, widgetPackageError("retired")
	}
	review := &widgetPackageReview{token: hex.EncodeToString(token[:]), name: selected.Name, archive: raw, pkg: p, expires: m.now().Add(5 * time.Minute)}
	m.review = review
	return WidgetPackageReview{Token: review.token, SourceName: review.name, Package: widgetCatalogItem(p), ExpiresAt: review.expires, AlreadyInstalled: a.widgets.packages[p.Digest()] != nil}, nil
}

func (a *App) CancelWidgetPackageReview(token string) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	m := a.widgetPackages
	if m == nil || m.closed {
		return widgetPackageError("unavailable")
	}
	if m.review != nil && m.review.token == token {
		m.review = nil
		a.publishWidgetPackagesLocked(m)
		return nil
	}
	// Before a chooser returns there is no review token. Empty cancels only that
	// pending chooser, never an installation/removal or a different ready review.
	if token == "" && m.job != nil && m.job.kind == "review" {
		m.job.cancel()
		return nil
	}
	return widgetPackageError("reviewExpired")
}

func (a *App) InstallReviewedWidget(token string) (WidgetCatalogItem, error) {
	a.viewMu.Lock()
	m, err := a.widgetPackageAdmissionLocked()
	if err != nil {
		a.viewMu.Unlock()
		return WidgetCatalogItem{}, err
	}
	r := m.review
	if r == nil || r.token != token || !m.now().Before(r.expires) {
		if r != nil && !m.now().Before(r.expires) {
			m.review = nil
		}
		a.viewMu.Unlock()
		return WidgetCatalogItem{}, widgetPackageError("reviewExpired")
	}
	m.review = nil
	owner := a.widgets
	j := a.beginWidgetPackageJobLocked(m, "install")
	a.viewMu.Unlock()
	defer a.finishWidgetPackageJob(m, j)
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.viewMu.Lock()
	current := a.currentWidgetPackageJobLocked(m, j) && m.now().Before(r.expires)
	existing := a.widgets.packages[r.pkg.Digest()]
	a.viewMu.Unlock()
	if !current {
		return WidgetCatalogItem{}, widgetPackageError("retired")
	}
	if existing == nil {
		if err = a.disableWidgetPackageReferencesLocked(r.pkg.Digest()); err != nil {
			return WidgetCatalogItem{}, err
		}
	}
	a.viewMu.Lock()
	current = a.currentWidgetPackageJobLocked(m, j) && m.now().Before(r.expires)
	a.viewMu.Unlock()
	if !current {
		return WidgetCatalogItem{}, widgetPackageError("retired")
	}
	p, err := m.store.Install(j.ctx, bytes.NewReader(r.archive))
	if err != nil {
		return WidgetCatalogItem{}, err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	// A successful Store return is committed catalog truth, independent of
	// the now-retired Preferences reply. New references were revoked before
	// publication; do not revive an old manager or widgets owner.
	if a.widgetPackages == m && !m.closed && a.widgets == owner && owner.ctx.Err() == nil {
		owner.packages[p.Digest()] = p
	}
	if !a.currentWidgetPackageJobLocked(m, j) || a.widgets != owner {
		return WidgetCatalogItem{}, widgetPackageError("retired")
	}
	m.reason = ""
	return widgetCatalogItem(p), nil
}

// Requires saveMu; never viewMu. Publication uses the existing serialized
// settings path. Failed persistence keeps both references and archive intact.
func (a *App) disableWidgetPackageReferencesLocked(digest string) error {
	s := a.settingsSnapshot()
	changed := false
	for i := range s.ReplacementDock.Profiles {
		for k := range s.ReplacementDock.Profiles[i].Widgets {
			w := &s.ReplacementDock.Profiles[i].Widgets[k]
			if w.Digest == digest && (w.Enabled || len(w.Grants) > 0) {
				w.Enabled = false
				w.Grants = []string{}
				changed = true
			}
		}
	}
	if changed {
		if err := a.saveSettingsLocked(s); err != nil {
			return widgetPackageError("settingsSaveFailed")
		}
	}
	return nil
}

func (a *App) RemoveWidgetPackage(digest string) error {
	a.viewMu.Lock()
	m, err := a.widgetPackageAdmissionLocked()
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	owner := a.widgets
	p := owner.packages[digest]
	if p == nil || widgetCatalogItem(p).Builtin {
		a.viewMu.Unlock()
		return widgetPackageError("invalidPackage")
	}
	if m.review != nil && m.review.pkg.Digest() == digest {
		m.review = nil
	}
	j := a.beginWidgetPackageJobLocked(m, "remove")
	a.viewMu.Unlock()
	defer a.finishWidgetPackageJob(m, j)
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.viewMu.Lock()
	current := a.currentWidgetPackageJobLocked(m, j)
	a.viewMu.Unlock()
	if !current {
		return widgetPackageError("retired")
	}
	if err = a.disableWidgetPackageReferencesLocked(digest); err != nil {
		return err
	}
	a.viewMu.Lock()
	current = a.currentWidgetPackageJobLocked(m, j)
	if current {
		delete(a.widgets.packages, digest)
		a.syncWidgetsLocked()
	}
	a.viewMu.Unlock()
	if !current {
		return widgetPackageError("retired")
	}
	// Disk failure leaves admission retired and references disabled; never restore
	// old grants to compensate for failed deletion.
	if err = m.store.Remove(j.ctx, digest); err != nil && !errors.Is(err, os.ErrNotExist) {
		a.viewMu.Lock()
		if a.widgetPackages == m && !m.closed && a.widgets == owner && owner.ctx.Err() == nil {
			// Restore catalog truth only. Persisted references stay disabled and
			// ungranted, including when Preferences closed during deletion.
			owner.packages[digest] = p
			m.reason = "removeFailed"
		}
		a.viewMu.Unlock()
		return err
	}
	a.viewMu.Lock()
	if a.widgetPackages == m && !m.closed {
		m.reason = ""
	}
	a.viewMu.Unlock()
	return nil
}

package diagnostics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"slices"
	"sync"
	"time"

	"option-tab/internal/platform"
)

type (
	record struct {
		Event
		ElapsedMS int64 `json:"elapsedMs"`
	}
	report struct {
		SchemaVersion     int            `json:"schemaVersion"`
		Status            StatusSnapshot `json:"status"`
		Events            []record       `json:"events"`
		Dropped           uint64         `json:"dropped"`
		CrashTextIncluded bool           `json:"crashTextIncluded"`
	}
	Service struct {
		mu                      sync.Mutex
		deps                    Deps
		closed, recording, busy bool
		started                 time.Time
		events                  []record
		sizes                   []int
		bytes                   int
		dropped                 uint64
		review                  Review
		cancel                  context.CancelFunc
	}
)

func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{deps: d}
}

func (s *Service) activeLocked() bool {
	if s.recording && s.deps.Now().Sub(s.started) >= RecordingLimit {
		s.recording = false
	}
	return s.recording
}
func (s *Service) Recording() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.activeLocked() }
func (s *Service) StartRecording() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if s.activeLocked() {
		return nil
	}
	s.recording = true
	s.started = s.deps.Now()
	s.events = nil
	s.sizes = nil
	s.bytes = 0
	s.dropped = 0
	return nil
}
func (s *Service) StopRecording() { s.mu.Lock(); s.recording = false; s.mu.Unlock() }
func (s *Service) clearLocked() {
	s.recording = false
	s.events = nil
	s.sizes = nil
	s.bytes = 0
	s.dropped = 0
	s.review = Review{}
	if s.cancel != nil {
		s.cancel()
	}
}
func (s *Service) Clear() { s.mu.Lock(); s.clearLocked(); s.mu.Unlock() }
func (s *Service) Close() { s.mu.Lock(); s.closed = true; s.clearLocked(); s.mu.Unlock() }
func validComponent(c Component) bool {
	return slices.Contains([]Component{Capture, Dock, Switcher, Media, Folders, Automation, Updates, Session}, c)
}

func validStatus(s Status) bool {
	return slices.Contains([]Status{Ready, Disabled, Unavailable, Unsupported, Unknown, Granted, Denied, Required}, s)
}

func (s *Service) Record(e Event) error {
	if !validComponent(e.Component) || !slices.Contains([]Code{SourceStarted, SourceStopped, SourceRefused, SourceRetry, CapacityRefused, ActionSucceeded, ActionRefused, PermissionChanged, PresentationRequested, PresentationRetired, ErrorReported}, e.Code) || e.Count < 0 || e.Count > 1_000_000 || e.DurationMS < 0 || e.DurationMS > 600_000 {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if !s.activeLocked() {
		return nil
	}
	elapsed := s.deps.Now().Sub(s.started).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	r := record{Event: e, ElapsedMS: elapsed}
	data, err := json.Marshal(r)
	if err != nil {
		return ErrInvalid
	}
	for len(s.events) >= MaxEvents || s.bytes+len(data) > MaxRingBytes {
		s.bytes -= s.sizes[0]
		s.events = s.events[1:]
		s.sizes = s.sizes[1:]
		s.dropped++
	}
	s.events = append(s.events, r)
	s.sizes = append(s.sizes, len(data))
	s.bytes += len(data)
	return nil
}

var (
	versionPattern = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}(-[a-z0-9.-]{1,24})?$`)
	runtimePattern = regexp.MustCompile(`^go[0-9]{1,3}\.[0-9]{1,3}(\.[0-9]{1,3})?$`)
)

func validSnapshot(v StatusSnapshot) bool {
	if !versionPattern.MatchString(v.Version) || (v.Runtime != "" && !runtimePattern.MatchString(v.Runtime)) || !slices.Contains([]string{"darwin", "linux", "windows"}, v.OS) || !slices.Contains([]string{"arm64", "amd64"}, v.Arch) || len(v.Features) > 8 || len(v.Permissions) > 4 {
		return false
	}
	features := map[Component]bool{}
	for _, f := range v.Features {
		if !validComponent(f.Component) || !validStatus(f.Status) || features[f.Component] {
			return false
		}
		features[f.Component] = true
	}
	permissions := map[Permission]bool{}
	for _, p := range v.Permissions {
		if !slices.Contains([]Permission{Accessibility, ScreenRecording, MusicAutomation, SpotifyAutomation}, p.Permission) || !validStatus(p.Status) || permissions[p.Permission] {
			return false
		}
		permissions[p.Permission] = true
	}
	return true
}

func (s *Service) Preview(v StatusSnapshot) (Review, error) {
	if !validSnapshot(v) {
		return Review{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Review{}, ErrClosed
	}
	if s.busy {
		return Review{}, ErrBusy
	}
	active := s.activeLocked()
	v.Features = slices.Clone(v.Features)
	v.Permissions = slices.Clone(v.Permissions)
	events := append([]record{}, s.events...)
	data, err := json.MarshalIndent(report{SchemaVersion: 1, Status: v, Events: events, Dropped: s.dropped}, "", "  ")
	if err != nil || len(data) > platform.MaxDiagnosticExportBytes {
		return Review{}, ErrInvalid
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return Review{}, ErrUnavailable
	}
	s.review = Review{Token: hex.EncodeToString(token[:]), JSON: string(data), ExpiresAt: s.deps.Now().Add(ReviewLifetime), Recording: active, Dropped: s.dropped}
	return s.review, nil
}

func (s *Service) Export(ctx context.Context, token string) (platform.DiagnosticExportResult, error) {
	if ctx == nil {
		return platform.DiagnosticExportResult{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return platform.DiagnosticExportResult{}, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return platform.DiagnosticExportResult{}, ErrClosed
	}
	if s.busy {
		s.mu.Unlock()
		return platform.DiagnosticExportResult{}, ErrBusy
	}
	if token == "" || token != s.review.Token || !s.deps.Now().Before(s.review.ExpiresAt) {
		s.mu.Unlock()
		return platform.DiagnosticExportResult{}, ErrStale
	}
	if s.deps.Exporter == nil {
		s.mu.Unlock()
		return platform.DiagnosticExportResult{}, ErrUnavailable
	}
	data := []byte(s.review.JSON)
	child, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.busy = true
	s.mu.Unlock()
	result, err := s.deps.Exporter.SaveDiagnosticReport(child, "option-tab-diagnostics.json", data)
	cancel()
	s.mu.Lock()
	s.cancel = nil
	s.busy = false
	if result.Status == "saved" {
		s.review = Review{}
	}
	s.mu.Unlock()
	return result, err
}

// CancelExport is nonblocking and leaves recording/history intact. The export
// slot remains busy until the native chooser/write owner has joined.
func (s *Service) CancelExport() {
	s.mu.Lock()
	s.review = Review{}
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
}

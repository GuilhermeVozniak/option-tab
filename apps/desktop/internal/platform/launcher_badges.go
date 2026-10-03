package platform

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type (
	LauncherBadgeState  string
	LauncherBadgeKind   string
	LauncherBadgeStatus string
)

const (
	BadgeKnown             LauncherBadgeState  = "known"
	BadgeUnavailable       LauncherBadgeState  = "unavailable"
	BadgeUnsupported       LauncherBadgeState  = "unsupported"
	BadgeAbsent            LauncherBadgeKind   = "absent"
	BadgeCount             LauncherBadgeKind   = "count"
	BadgeIndicator         LauncherBadgeKind   = "indicator"
	BadgeReady             LauncherBadgeStatus = "ready"
	BadgeSourceUnavailable LauncherBadgeStatus = "unavailable"
	BadgeSourceUnsupported LauncherBadgeStatus = "unsupported"
)

// Targets are private Go-only identity; paths never belong in renderer state.
type (
	LauncherBadgeTarget struct {
		ItemKey          string          `json:"itemKey"`
		TargetRevision   uint64          `json:"targetRevision"`
		Process          ProcessIdentity `json:"process"`
		BundleID         string          `json:"bundleID"`
		CanonicalAppPath string          `json:"path"`
	}
	LauncherBadgeEntry struct {
		ItemKey        string
		TargetRevision uint64
		State          LauncherBadgeState
		Kind           LauncherBadgeKind
		Count          *uint32
	}
	LauncherBadgeSnapshot struct {
		Generation, Sequence uint64
		ObservedAt           time.Time
		Status               LauncherBadgeStatus
		Entries              []LauncherBadgeEntry
	}
	LauncherBadgeSource interface {
		ObserveLauncherBadges(context.Context, []LauncherBadgeTarget, func(LauncherBadgeSnapshot)) error
	}
	badgeObservation struct {
		Dock    ProcessIdentity
		Status  LauncherBadgeStatus
		Entries []LauncherBadgeEntry
	}
	launcherBadgeSource struct {
		mu         sync.Mutex
		active     bool
		generation uint64
		read       func(context.Context, []LauncherBadgeTarget) (badgeObservation, error)
	}
)

func newLauncherBadgeSource(read func(context.Context, []LauncherBadgeTarget) (badgeObservation, error)) *launcherBadgeSource {
	return &launcherBadgeSource{read: read}
}

func validBadgeTargets(targets []LauncherBadgeTarget) bool {
	if len(targets) == 0 || len(targets) > 256 {
		return false
	}
	seen := map[string]bool{}
	for _, t := range targets {
		if t.ItemKey == "" || len(t.ItemKey) > 128 || seen[t.ItemKey] || t.TargetRevision == 0 || !utf8.ValidString(t.ItemKey) || !utf8.ValidString(t.CanonicalAppPath) || strings.ContainsRune(t.CanonicalAppPath, 0) || len(t.CanonicalAppPath) > 4096 || !filepath.IsAbs(t.CanonicalAppPath) || filepath.Clean(t.CanonicalAppPath) != t.CanonicalAppPath || !strings.EqualFold(filepath.Ext(t.CanonicalAppPath), ".app") || t.BundleID == "" || len(t.BundleID) > 255 || strings.ContainsRune(t.BundleID, 0) {
			return false
		}
		if t.Process.PID < 0 || (t.Process.PID > 0 && (t.Process.StartSeconds == 0 || t.Process.StartMicros >= 1_000_000)) || (t.Process.PID == 0 && (t.Process.StartSeconds != 0 || t.Process.StartMicros != 0)) {
			return false
		}
		seen[t.ItemKey] = true
	}
	return true
}

func classifyLauncherBadge(text string) (LauncherBadgeKind, *uint32) {
	if text == "" {
		return BadgeAbsent, nil
	}
	var count uint32
	if len(text) > 6 {
		return BadgeIndicator, nil
	}
	for _, v := range []byte(text) {
		if v < '0' || v > '9' {
			return BadgeIndicator, nil
		}
		count = count*10 + uint32(v-'0')
	}
	return BadgeCount, &count
}

func copyBadgeEntries(entries []LauncherBadgeEntry) []LauncherBadgeEntry {
	out := append([]LauncherBadgeEntry(nil), entries...)
	for i := range out {
		if out[i].Count != nil {
			n := *out[i].Count
			out[i].Count = &n
		}
	}
	return out
}

func (s *launcherBadgeSource) ObserveLauncherBadges(ctx context.Context, targets []LauncherBadgeTarget, emit func(LauncherBadgeSnapshot)) error {
	if ctx == nil || emit == nil || !validBadgeTargets(targets) {
		return errors.New("invalid launcher badge observation")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return errors.New("launcher badge observation busy")
	}
	s.active = true
	s.generation++
	generation := s.generation
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active = false; s.mu.Unlock() }()
	targets = append([]LauncherBadgeTarget(nil), targets...)
	var previous badgeObservation
	var sequence uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		started := time.Now()
		observation, err := s.read(ctx, targets)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if sequence == 0 || !reflect.DeepEqual(previous, observation) {
			if sequence > 0 && previous.Dock != observation.Dock {
				s.mu.Lock()
				s.generation++
				generation = s.generation
				s.mu.Unlock()
			}
			sequence++
			previous = badgeObservation{Dock: observation.Dock, Status: observation.Status, Entries: copyBadgeEntries(observation.Entries)}
			emit(LauncherBadgeSnapshot{Generation: generation, Sequence: sequence, ObservedAt: time.Now(), Status: observation.Status, Entries: copyBadgeEntries(observation.Entries)})
		}
		delay := time.Until(started.Add(time.Second))
		if delay < 0 {
			delay = 0
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

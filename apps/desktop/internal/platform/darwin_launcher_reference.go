//go:build darwin

package platform

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"option-tab/internal/launcheritems"
)

type darwinLauncherReferenceSource struct {
	life      *launcherItemLifetime
	store     *launcheritems.Store
	picker    chan struct{}
	control   chan struct{}
	closeOnce sync.Once
	closeErr  error
	authority *launcherReferenceAuthority
}

func NewLauncherReferenceSource(path string) (LauncherReferenceSource, error) {
	store, err := launcheritems.OpenStore(path)
	if err != nil {
		return nil, err
	}
	return &darwinLauncherReferenceSource{life: newLauncherItemLifetime(), store: store, picker: make(chan struct{}, 1), control: make(chan struct{}, 1), authority: newLauncherReferenceAuthority()}, nil
}

func referenceDTO(r launcheritems.Record, state string) LauncherReference {
	return LauncherReference{ID: r.ID, Kind: r.Kind, Label: r.Label, BundleID: r.BundleID, Revision: r.Revision, State: state}
}

func nativeRecord(r launcheritems.Record) launcherNativeRecord {
	return launcherNativeRecord{Kind: r.Kind, Label: r.Label, SelectedPath: r.SelectedPath, BundleID: r.BundleID, Bookmark: r.Bookmark}
}

func (s *darwinLauncherReferenceSource) choose(ctx context.Context, kind, id string) (LauncherReference, error) {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return LauncherReference{}, err
	}
	defer end()
	if kind != "app" && kind != "folder" && kind != "file" {
		return LauncherReference{}, launcherItemError("invalidArgument")
	}
	select {
	case s.picker <- struct{}{}:
		defer func() { <-s.picker }()
	default:
		return LauncherReference{}, launcherItemError("busy")
	}
	var old launcheritems.Record
	if id != "" {
		old, err = s.store.Get(id)
		if err != nil {
			return LauncherReference{}, err
		}
		if old.Kind != kind {
			return LauncherReference{}, launcherItemError("changed")
		}
	}
	selected, err := chooseLauncherNative(ctx, kind)
	if err != nil {
		return LauncherReference{}, err
	}
	if err = ctx.Err(); err != nil {
		return LauncherReference{}, err
	}
	r := launcheritems.Record{ID: old.ID, Revision: old.Revision, Kind: kind, Label: boundedLauncherLabel(selected.Label, "Item"), SelectedPath: selected.SelectedPath, BundleID: selected.BundleID, Bookmark: selected.Bookmark}
	if id == "" {
		r, err = s.store.Create(ctx, r)
	} else {
		r, err = s.store.Put(ctx, r)
	}
	if err != nil {
		return LauncherReference{}, err
	}
	s.authority.invalidate(r.ID)
	resolved, resolveErr := s.ResolveLauncherReference(ctx, r.ID)
	if resolveErr != nil && resolved.ID == "" {
		return referenceDTO(r, "unavailable"), resolveErr
	}
	return resolved, resolveErr
}

func (s *darwinLauncherReferenceSource) ChooseLauncherReference(ctx context.Context, kind string) (LauncherReference, error) {
	return s.choose(ctx, kind, "")
}

func (s *darwinLauncherReferenceSource) RelinkLauncherReference(ctx context.Context, id string) (LauncherReference, error) {
	r, err := s.store.Get(id)
	if err != nil {
		return LauncherReference{}, err
	}
	return s.choose(ctx, r.Kind, id)
}

func (s *darwinLauncherReferenceSource) ResolveLauncherReference(ctx context.Context, id string) (LauncherReference, error) {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return LauncherReference{}, err
	}
	defer end()
	r, err := s.store.Get(id)
	if err != nil {
		return LauncherReference{ID: id, State: "needsSelection"}, err
	}
	ticket := s.authority.start(id)
	scope, resolveErr := resolveLauncherNative(nativeRecord(r))
	if scope != nil {
		defer scope.close()
	}
	if err = s.current(ctx, r); err != nil {
		return LauncherReference{}, err
	}
	dto := referenceDTO(r, "ready")
	fingerprint := ""
	if resolveErr != nil {
		dto.State = "unavailable"
		var native *LauncherItemError
		if errors.As(resolveErr, &native) {
			dto.State = native.Code
			dto.Reason = native.Code
		}
		fingerprint = "error:" + dto.State
	} else {
		fingerprint = scope.fingerprint
		dto.Process = scope.process
		if r.Kind == "app" {
			dto.Label = scope.label
		}
	}
	revision, ok := s.authority.publish(id, ticket, r.Revision, fingerprint)
	if !ok {
		return LauncherReference{}, launcherItemError("changed")
	}
	dto.Revision = revision
	return dto, nil
}

func (s *darwinLauncherReferenceSource) current(ctx context.Context, r launcheritems.Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now, err := s.store.Get(r.ID)
	if err != nil {
		return err
	}
	if now.Revision != r.Revision {
		return launcherItemError("changed")
	}
	return ctx.Err()
}

func (s *darwinLauncherReferenceSource) PerformLauncherItem(ctx context.Context, a LauncherItemAction, guard func() error) error {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	select {
	case s.control <- struct{}{}:
		defer func() { <-s.control }()
	default:
		return launcherItemError("busy")
	}
	if guard == nil || a.PanelToken == 0 || a.DisplayUUID == "" || a.ReferenceRevision == 0 || (a.Kind != "open" && a.Kind != "relaunch") {
		return launcherItemError("invalidArgument")
	}
	r, err := s.store.Get(a.ReferenceID)
	if err != nil {
		return err
	}
	if r.BundleID != a.BundleID {
		return launcherItemError("changed")
	}
	if a.Kind == "relaunch" && (r.Kind != "app" || a.Process.PID <= 0 || a.Process.StartSeconds == 0 || a.Process.StartMicros >= 1_000_000) {
		return launcherItemError("invalidArgument")
	}
	scope, err := resolveLauncherNative(nativeRecord(r))
	if err != nil {
		return err
	}
	defer scope.close()
	final := func() error {
		if !s.authority.current(r.ID, a.ReferenceRevision, r.Revision, scope.fingerprint) {
			return launcherItemError("changed")
		}
		if err := s.current(ctx, r); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		if err := s.current(ctx, r); err != nil {
			return err
		}
		if !s.authority.current(r.ID, a.ReferenceRevision, r.Revision, scope.fingerprint) {
			return launcherItemError("changed")
		}
		return nil
	}
	if err = final(); err != nil {
		return err
	}
	return openLauncherNative(ctx, scope, a, "", final)
}

func (s *darwinLauncherReferenceSource) WithLauncherFolder(ctx context.Context, id string, revision uint64, use func(FolderRef) error) error {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	if use == nil {
		return launcherItemError("invalidArgument")
	}
	r, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if r.Kind != "folder" {
		return launcherItemError("changed")
	}
	scope, err := resolveLauncherNative(nativeRecord(r))
	if err != nil {
		return err
	}
	defer scope.close()
	if err = s.current(ctx, r); err != nil {
		return err
	}
	if !s.authority.current(id, revision, r.Revision, scope.fingerprint) {
		return launcherItemError("changed")
	}
	identity, err := FolderIdentity(filepath.Clean(scope.path))
	if err != nil {
		return err
	}
	if !scope.current() {
		return launcherItemError("changed")
	}
	err = use(FolderRef{Identity: identity, Path: scope.path})
	if err != nil {
		return err
	}
	return s.current(ctx, r)
}

func (s *darwinLauncherReferenceSource) OpenLauncherLink(ctx context.Context, url, display string, token uint64, guard func() error) error {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	select {
	case s.control <- struct{}{}:
		defer func() { <-s.control }()
	default:
		return launcherItemError("busy")
	}
	if !validLauncherLink(url) || display == "" || token == 0 || guard == nil {
		return launcherItemError("invalidArgument")
	}
	return openLauncherNative(ctx, nil, LauncherItemAction{Kind: "open", DisplayUUID: display, PanelToken: token}, url, guard)
}

func (s *darwinLauncherReferenceSource) RemoveLauncherReference(ctx context.Context, id string) error {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	if len(id) != 32 {
		return launcherItemError("invalidArgument")
	}
	if decoded, e := hex.DecodeString(id); e != nil || hex.EncodeToString(decoded) != id {
		return launcherItemError("invalidArgument")
	}
	s.authority.invalidate(id)
	err = s.store.Remove(ctx, id)
	if errors.Is(err, launcheritems.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *darwinLauncherReferenceSource) Close() error {
	s.closeOnce.Do(func() { s.life.close(); s.closeErr = s.store.Close() })
	return s.closeErr
}

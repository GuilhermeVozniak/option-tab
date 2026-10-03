package platform

import (
	"context"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// These optional resolvers return private copied identity only. ItemKey and
// TargetRevision remain the caller's presentation authority. No security grant
// survives return; a later unreadable badge target remains unavailable.
type LauncherBadgeReferenceResolver interface {
	ResolveLauncherBadgeTarget(context.Context, LauncherReference) (LauncherBadgeTarget, error)
}
type LauncherRunningBadgeResolver interface {
	ResolveRunningLauncherBadgeTarget(context.Context, ProcessIdentity, string) (LauncherBadgeTarget, error)
}
type badgeReferenceResolution struct {
	Reference LauncherReference
	Path      string
	Current   func() bool
	Close     func()
}

func validBadgeIdentity(v LauncherBadgeTarget) bool {
	return utf8.ValidString(v.CanonicalAppPath) && len(v.CanonicalAppPath) <= 4096 && !strings.ContainsRune(v.CanonicalAppPath, 0) && filepath.IsAbs(v.CanonicalAppPath) && filepath.Clean(v.CanonicalAppPath) == v.CanonicalAppPath && strings.EqualFold(filepath.Ext(v.CanonicalAppPath), ".app") && v.BundleID != "" && len(v.BundleID) <= 255 && utf8.ValidString(v.BundleID) && !strings.ContainsRune(v.BundleID, 0)
}

func resolveReferenceBadgeTarget(ctx context.Context, expected LauncherReference, resolve func() (badgeReferenceResolution, error)) (LauncherBadgeTarget, error) {
	if ctx == nil || expected.ID == "" || expected.Kind != "app" || expected.State != "ready" || expected.Revision == 0 {
		return LauncherBadgeTarget{}, launcherItemError("invalidArgument")
	}
	if err := ctx.Err(); err != nil {
		return LauncherBadgeTarget{}, err
	}
	r, err := resolve()
	if r.Close != nil {
		defer r.Close()
	}
	if err != nil {
		return LauncherBadgeTarget{}, err
	}
	if err = ctx.Err(); err != nil {
		return LauncherBadgeTarget{}, err
	}
	if r.Reference != expected || r.Current == nil || !r.Current() {
		return LauncherBadgeTarget{}, launcherItemError("changed")
	}
	v := LauncherBadgeTarget{Process: r.Reference.Process, BundleID: r.Reference.BundleID, CanonicalAppPath: r.Path}
	if !validBadgeIdentity(v) || !r.Current() {
		return LauncherBadgeTarget{}, launcherItemError("changed")
	}
	if err = ctx.Err(); err != nil {
		return LauncherBadgeTarget{}, err
	}
	return v, nil
}

func resolveRunningBadgeTarget(ctx context.Context, process ProcessIdentity, bundle string, read func(context.Context, ProcessIdentity, string) (LauncherBadgeTarget, error)) (LauncherBadgeTarget, error) {
	if ctx == nil || process.PID <= 0 || process.StartSeconds == 0 || process.StartMicros >= 1_000_000 || bundle == "" || len(bundle) > 255 || !utf8.ValidString(bundle) || strings.ContainsRune(bundle, 0) {
		return LauncherBadgeTarget{}, launcherItemError("invalidArgument")
	}
	if err := ctx.Err(); err != nil {
		return LauncherBadgeTarget{}, err
	}
	first, err := read(ctx, process, bundle)
	if err != nil {
		return LauncherBadgeTarget{}, err
	}
	if first.Process != process || first.BundleID != bundle || !validBadgeIdentity(first) {
		return LauncherBadgeTarget{}, launcherItemError("changed")
	}
	if err = ctx.Err(); err != nil {
		return LauncherBadgeTarget{}, err
	}
	last, err := read(ctx, process, bundle)
	if err != nil {
		return LauncherBadgeTarget{}, err
	}
	if last != first {
		return LauncherBadgeTarget{}, launcherItemError("changed")
	}
	if err = ctx.Err(); err != nil {
		return LauncherBadgeTarget{}, err
	}
	return first, nil
}

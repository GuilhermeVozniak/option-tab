//go:build !darwin

package platform

import "context"

func NewLauncherBadgeSource() LauncherBadgeSource {
	return newLauncherBadgeSource(func(_ context.Context, targets []LauncherBadgeTarget) (badgeObservation, error) {
		entries := make([]LauncherBadgeEntry, len(targets))
		for i, t := range targets {
			entries[i] = LauncherBadgeEntry{ItemKey: t.ItemKey, TargetRevision: t.TargetRevision, State: BadgeUnsupported}
		}
		return badgeObservation{Status: BadgeSourceUnsupported, Entries: entries}, nil
	})
}

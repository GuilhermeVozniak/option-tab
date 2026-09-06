package actions

import (
	"errors"
	"fmt"
	"os"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// PerformOthers preserves the explicit kept root and protected native surfaces.
// A gesture caller uses PerformOthersGuarded to validate its movement evidence.
func (s *Service) PerformOthers(kind string, keepWindow domain.WindowID, keepApp domain.AppID) (Result, error) {
	return s.PerformOthersGuarded(kind, keepWindow, keepApp, nil)
}

// PerformOthersGuarded snapshots classified roots and requires a native action
// capability which rechecks identity and modal relationships at dispatch. It
// deliberately cannot fall back to the ordinary close/minimize methods.
func (s *Service) PerformOthersGuarded(kind string, keepWindow domain.WindowID, keepApp domain.AppID, guard func() error) (Result, error) {
	r := Result{Failures: []Failure{}}
	nativeKind := ""
	switch kind {
	case "closeOthers":
		nativeKind = "close"
	case "minimizeOthers":
		nativeKind = "setMinimized"
	default:
		return r, fmt.Errorf("unsupported other-window action %q", kind)
	}
	self := domain.AppID(os.Getpid())
	if keepWindow == 0 || keepApp <= 0 || keepApp == self {
		return r, errors.New("invalid or self kept-window identity")
	}
	source, hasRoles := s.backend.(platform.WindowRoleSource)
	performer, hasActions := s.backend.(platform.OtherWindowPerformer)
	if !hasRoles || !hasActions {
		return r, errors.New("classified other-window actions are unavailable")
	}
	guarded, hasGuardedActions := s.backend.(platform.GuardedOtherWindowPerformer)
	if guard != nil && !hasGuardedActions {
		return r, errors.New("guarded other-window actions are unavailable")
	}
	if guard != nil {
		if err := guard(); err != nil {
			return r, err
		}
	}
	roots, err := source.ActionWindowRoles()
	if err != nil {
		if len(roots) == 0 {
			return r, err
		}
		r.Failures = append(r.Failures, Failure{Error: err.Error()})
	}
	byID := make(map[domain.WindowID]platform.ActionWindowRole, len(roots))
	ambiguous := make(map[domain.WindowID]bool)
	order := make([]domain.WindowID, 0, len(roots))
	for _, root := range roots {
		if existing, seen := byID[root.WindowID]; seen {
			if existing != root {
				ambiguous[root.WindowID] = true
			}
			continue
		}
		byID[root.WindowID] = root
		order = append(order, root.WindowID)
	}
	keep, found := byID[keepWindow]
	if !found || ambiguous[keepWindow] || keep.AppID != keepApp || !safeOtherRoot(keep) {
		return r, errors.New("kept window is no longer an unambiguous standard root")
	}
	if guard != nil {
		if err := guard(); err != nil {
			return r, err
		}
	}
	for _, id := range order {
		root := byID[id]
		if id == keepWindow || root.AppID == self || root.SelfApplication {
			continue
		}
		if ambiguous[id] {
			r.Failures = append(r.Failures, Failure{id, "window classification has conflicting identities or relationships"})
			continue
		}
		// Positive nonstandard role evidence is enough to preserve a surface.
		// Missing modal metadata on a known AXSheet does not make it an action
		// candidate or a failed attempt. Standard roots still require every fact.
		if id != 0 && root.AppID > 0 && root.Role != "" && (root.Role != "AXWindow" || (root.Subrole != "" && root.Subrole != "AXStandardWindow")) {
			continue
		}
		if id == 0 || root.AppID <= 0 || root.Reason != "" || root.Role == "" {
			reason := root.Reason
			if reason == "" {
				reason = "window identity or role is unresolved"
			}
			r.Failures = append(r.Failures, Failure{id, reason})
			continue
		}
		if root.Subrole == "" || !root.RootConfirmed || !root.RelationshipsKnown {
			r.Failures = append(r.Failures, Failure{id, "window root or modal relationships are unresolved"})
			continue
		}
		if !safeOtherRoot(root) {
			continue
		}
		if guard != nil {
			if err := guard(); err != nil {
				return r, err
			}
		}
		var actionErr error
		if guard != nil {
			var finalGuardErr error
			actionErr = guarded.PerformOtherWindowActionGuarded(nativeKind, id, root.AppID, func() error {
				finalGuardErr = guard()
				return finalGuardErr
			})
			if finalGuardErr != nil {
				return r, finalGuardErr
			}
		} else {
			actionErr = performer.PerformOtherWindowAction(nativeKind, id, root.AppID)
		}
		if actionErr != nil {
			r.Failures = append(r.Failures, Failure{id, actionErr.Error()})
		} else {
			r.Succeeded++
		}
	}
	return r, nil
}

func safeOtherRoot(root platform.ActionWindowRole) bool {
	return root.WindowID != 0 && root.AppID > 0 && root.Role == "AXWindow" && root.Subrole == "AXStandardWindow" && root.RootConfirmed && root.RelationshipsKnown && !root.SelfApplication && !root.Modal && !root.HasAttachedSheet && !root.HasModalChild && root.ParentWindowID == 0 && root.Reason == ""
}

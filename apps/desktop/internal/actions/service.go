// Package actions applies operations to explicit, freshly validated identities.
package actions

import (
	"errors"
	"fmt"
	"os"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// Failure uses WindowID zero for a snapshot-wide incomplete-enumeration error.
type Failure struct {
	WindowID domain.WindowID `json:"windowId"`
	Error    string          `json:"error"`
}
type Result struct {
	Succeeded int       `json:"succeeded"`
	Failures  []Failure `json:"failures"`
}
type Backend interface {
	platform.WindowSource
	platform.Focuser
}

// TargetPerformer revalidates ownership at native lookup and reports AX errors.
type TargetPerformer interface {
	PerformTargetAction(string, domain.WindowID, domain.AppID) error
}

// ActionWindowSource provides native AX window roots for bulk snapshots,
// excluding confirmed non-window surfaces without visibility/user filters.
// It may return known roots with an error to report incomplete enumeration.
type ActionWindowSource interface {
	ActionWindows(domain.AppID) ([]domain.Window, error)
}

type (
	NewWindower  interface{ NewWindow(domain.AppID) error }
	ForceQuitter interface{ ForceQuitApp(domain.AppID) error }
	Service      struct{ backend Backend }
)

func New(b Backend) *Service { return &Service{backend: b} }

func (s *Service) Perform(kind string, id domain.WindowID, app domain.AppID) (Result, error) {
	return s.perform(kind, id, app, false)
}

func (s *Service) perform(kind string, id domain.WindowID, app domain.AppID, minimizeOnly bool) (Result, error) {
	r := Result{Failures: []Failure{}}
	window := kind == "focus" || kind == "close" || kind == "minimize" || kind == "fullscreen"
	bulk := kind == "closeAll" || kind == "minimizeAll"
	if !window && !bulk && kind != "hide" && kind != "quit" && kind != "forceQuit" && kind != "newWindow" {
		return r, fmt.Errorf("unsupported action %q", kind)
	}
	if app < 0 || (!window && app == 0) || app == domain.AppID(os.Getpid()) {
		return r, errors.New("invalid or self application identity")
	}
	if window || bulk {
		var wins []domain.Window
		var err error
		if source, ok := s.backend.(ActionWindowSource); bulk && ok {
			wins, err = source.ActionWindows(app)
		} else {
			wins, err = s.backend.Windows()
		}
		if err != nil {
			if !bulk || len(wins) == 0 {
				return r, err
			}
			r.Failures = append(r.Failures, Failure{WindowID: 0, Error: err.Error()})
		}
		if bulk {
			op := "close"
			if kind == "minimizeAll" {
				op = "minimize"
			}
			for _, w := range wins {
				if w.AppID != app {
					continue
				}
				_, err := s.perform(op, w.ID, app, kind == "minimizeAll")
				if err != nil {
					r.Failures = append(r.Failures, Failure{w.ID, err.Error()})
				} else {
					r.Succeeded++
				}
			}
			return r, nil
		}
		var owner domain.AppID
		alreadyMinimized := false
		for _, w := range wins {
			if w.ID == id && id != 0 {
				owner = w.AppID
				alreadyMinimized = w.Minimized
				break
			}
		}
		if owner <= 0 || (app != 0 && app != owner) || owner == domain.AppID(os.Getpid()) {
			return r, errors.New("window no longer exists or application identity mismatches")
		}
		if minimizeOnly && alreadyMinimized {
			r.Succeeded = 1
			return r, nil
		}
		app = owner
	}
	var err error
	if p, ok := s.backend.(TargetPerformer); ok {
		nativeKind := kind
		if minimizeOnly {
			nativeKind = "setMinimized"
		}
		err = p.PerformTargetAction(nativeKind, id, app)
	} else {
		switch kind {
		case "focus":
			err = s.backend.Focus(id)
		case "close":
			err = s.backend.Close(id)
		case "minimize":
			err = s.backend.Minimize(id)
		case "fullscreen":
			err = s.backend.Fullscreen(id)
		case "hide":
			err = s.backend.HideApp(app)
		case "quit":
			err = s.backend.QuitApp(app)
		case "forceQuit":
			if p, ok := s.backend.(ForceQuitter); ok {
				err = p.ForceQuitApp(app)
			} else {
				err = errors.New("force quit is unsupported by this platform")
			}
		case "newWindow":
			if p, ok := s.backend.(NewWindower); ok {
				err = p.NewWindow(app)
			} else {
				err = errors.New("new window is unsupported by this platform")
			}
		}
	}
	if err != nil {
		r.Failures = append(r.Failures, Failure{id, err.Error()})
		return r, err
	}
	r.Succeeded = 1
	return r, nil
}

// Package launcher owns the opt-in running-application launcher, without native UI or capture.
package launcher

import (
	"context"
	"errors"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

var (
	ErrRetired     = errors.New("launcher: retired scope")
	ErrUnavailable = errors.New("launcher: unavailable")
	ErrBusy        = errors.New("launcher: action busy")
)

type Scope struct {
	Epoch       uint64 `json:"epoch"`
	DisplayUUID string `json:"displayUUID"`
	Session     uint64 `json:"session"`
	Revision    uint64 `json:"revision"`
}
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}
type WidgetNode struct {
	Kind     string       `json:"kind"`
	Text     string       `json:"text,omitempty"`
	Children []WidgetNode `json:"children,omitempty"`
}
type Widget struct {
	ID        string     `json:"id"`
	PackageID string     `json:"packageID"`
	Digest    string     `json:"digest"`
	Status    string     `json:"status"`
	Root      WidgetNode `json:"root"`
}
type Presentation struct {
	Edge       string                    `json:"edge"`
	Layout     string                    `json:"layout"`
	Appearance config.LauncherAppearance `json:"appearance"`
	Scope
	Visible   bool          `json:"visible"`
	Reason    string        `json:"reason"`
	ProfileID string        `json:"profileID"`
	Bounds    domain.Bounds `json:"bounds"`
	IconPx    int           `json:"iconPx"`
	Items     []Item        `json:"items"`
	Widgets   []Widget      `json:"widgets"`
}
type DisplayState struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Main      bool   `json:"main"`
	BindingID string `json:"bindingID"`
	ProfileID string `json:"profileID"`
	SpaceKind string `json:"spaceKind"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}
type State struct {
	PointerOwned  bool           `json:"-"`
	Epoch         uint64         `json:"epoch"`
	Enabled       bool           `json:"enabled"`
	Status        string         `json:"status"`
	Displays      []DisplayState `json:"displays"`
	Presentations []Presentation `json:"presentations"`
}
type (
	View       interface{ Publish(State) }
	Identities interface {
		ProcessIdentity(domain.AppID) (platform.ProcessIdentity, error)
	}
	Deps struct {
		SelfAppID    domain.AppID
		SelfBundleID string
		Eligible     func(domain.App) bool
		Environment  platform.LauncherEnvironmentSource
		Applications platform.ApplicationSource
		Identities   Identities
		View         View
		Now          func() time.Time
		Activate     func(context.Context, Scope, platform.LauncherAppTarget, func() error) error
	}
)

func cloneNode(n WidgetNode) WidgetNode {
	n.Children = append([]WidgetNode(nil), n.Children...)
	for i := range n.Children {
		n.Children[i] = cloneNode(n.Children[i])
	}
	return n
}

func cloneState(s State) State {
	s.Displays = append([]DisplayState{}, s.Displays...)
	s.Presentations = append([]Presentation{}, s.Presentations...)
	for i := range s.Presentations {
		p := &s.Presentations[i]
		p.Items = append([]Item{}, p.Items...)
		p.Widgets = append([]Widget{}, p.Widgets...)
		for j := range p.Widgets {
			p.Widgets[j].Root = cloneNode(p.Widgets[j].Root)
		}
	}
	return s
}

func granted(w config.WidgetInstance) bool {
	return w.Enabled && w.PackageID == config.BuiltinClockPackage && w.Digest == config.BuiltinClockDigest && len(w.Grants) == 1 && w.Grants[0] == "clock.read"
}

// Package diagnostics builds allowlisted local support reports. It never reads
// raw logs, errors, settings, crash files, native data, or arbitrary attributes.
package diagnostics

import (
	"errors"
	"time"

	"option-tab/internal/platform"
)

const (
	MaxEvents      = 1024
	MaxRingBytes   = 256 * 1024
	RecordingLimit = 10 * time.Minute
	ReviewLifetime = 5 * time.Minute
)

var (
	ErrInvalid     = errors.New("diagnostics: invalid typed data")
	ErrBusy        = errors.New("diagnostics: export busy")
	ErrStale       = errors.New("diagnostics: review expired or retired")
	ErrClosed      = errors.New("diagnostics: service closed")
	ErrUnavailable = errors.New("diagnostics: export unavailable")
)

type Component string

const (
	Capture    Component = "capture"
	Dock       Component = "dock"
	Switcher   Component = "switcher"
	Media      Component = "media"
	Folders    Component = "folders"
	Automation Component = "automation"
	Updates    Component = "updates"
	Session    Component = "session"
)

type Code string

const (
	SourceStarted     Code = "sourceStarted"
	SourceStopped     Code = "sourceStopped"
	SourceRefused     Code = "sourceRefused"
	SourceRetry       Code = "sourceRetry"
	CapacityRefused   Code = "capacityRefused"
	ActionSucceeded   Code = "actionSucceeded"
	ActionRefused     Code = "actionRefused"
	PermissionChanged Code = "permissionChanged"
)

const (
	PresentationRequested Code = "presentationRequested"
	PresentationRetired   Code = "presentationRetired"
	ErrorReported         Code = "errorReported"
)

type Status string

const (
	Ready       Status = "ready"
	Disabled    Status = "disabled"
	Unavailable Status = "unavailable"
	Unsupported Status = "unsupported"
	Unknown     Status = "unknown"
	Granted     Status = "granted"
	Denied      Status = "denied"
	Required    Status = "required"
)

type (
	Event struct {
		Component  Component `json:"component"`
		Code       Code      `json:"code"`
		Count      int64     `json:"count,omitempty"`
		DurationMS int64     `json:"durationMs,omitempty"`
	}
	FeatureStatus struct {
		Component Component `json:"component"`
		Enabled   bool      `json:"enabled"`
		Status    Status    `json:"status"`
	}
	Permission string
)

const (
	Accessibility     Permission = "accessibility"
	ScreenRecording   Permission = "screenRecording"
	MusicAutomation   Permission = "musicAutomation"
	SpotifyAutomation Permission = "spotifyAutomation"
)

type (
	PermissionStatus struct {
		Permission Permission `json:"permission"`
		Status     Status     `json:"status"`
	}
	StatusSnapshot struct {
		Version     string             `json:"version"`
		Runtime     string             `json:"runtime,omitempty"`
		OS          string             `json:"os"`
		Arch        string             `json:"arch"`
		Features    []FeatureStatus    `json:"features"`
		Permissions []PermissionStatus `json:"permissions"`
	}
	Review struct {
		Token     string    `json:"token"`
		JSON      string    `json:"json"`
		ExpiresAt time.Time `json:"expiresAt"`
		Recording bool      `json:"recording"`
		Dropped   uint64    `json:"dropped"`
	}
	Deps struct {
		Now      func() time.Time
		Exporter platform.DiagnosticExportSource
	}
)

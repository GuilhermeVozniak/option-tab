package widgets

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRetired     = errors.New("widgets: retired lease")
	ErrBusy        = errors.New("widgets: action busy")
	ErrUnavailable = errors.New("widgets: unavailable")
)

type (
	Value struct {
		Text    *string  `json:"text,omitempty"`
		Number  *float64 `json:"number,omitempty"`
		Boolean *bool    `json:"boolean,omitempty"`
	}
	Request struct {
		Package         *Package
		ControllerEpoch uint64
		DisplayUUID     string
		Session         uint64
		ProfileID       string
		InstanceID      string
		Enabled         bool
		Grants          []string
		Settings        map[string]Value
	}
)

type Lease struct {
	ControllerEpoch uint64 `json:"controllerEpoch"`
	DisplayUUID     string `json:"displayUUID"`
	Session         uint64 `json:"session"`
	ProfileID       string `json:"profileID"`
	InstanceID      string `json:"instanceID"`
	Digest          string `json:"digest"`
	AdmissionEpoch  uint64 `json:"admissionEpoch"`
	Revision        uint64 `json:"revision"`
}
type RenderNode struct {
	Key         string       `json:"key"`
	Kind        string       `json:"kind"`
	Text        string       `json:"text,omitempty"`
	Status      string       `json:"status"`
	Progress    *float64     `json:"progress,omitempty"`
	History     []float64    `json:"history,omitempty"`
	AssetToken  string       `json:"assetToken,omitempty"`
	ActionToken string       `json:"actionToken,omitempty"`
	Children    []RenderNode `json:"children,omitempty"`
}
type (
	InstanceState struct {
		Lease  Lease      `json:"lease"`
		Status string     `json:"status"`
		Reason string     `json:"reason,omitempty"`
		Root   RenderNode `json:"root"`
	}
	ProviderOption struct{ ID, Label string }
	NumberRange    struct {
		Min  float64 `json:"min"`
		Max  float64 `json:"max"`
		Step float64 `json:"step"`
	}
	ActionSpec struct {
		Enabled bool
		Options []ProviderOption
		Range   *NumberRange
	}
	Sample struct {
		Generation, Sequence uint64
		ObservedAt           time.Time
		Status, Reason       string
		Fields               map[string]Value
		Actions              map[string]ActionSpec
	}
	ProviderAction struct {
		Generation uint64
		Action     string
		OptionID   string
		Value      *float64
	}
	Provider interface {
		Observe(context.Context, []string, func(Sample)) error
	}
	ActionProvider interface {
		Perform(context.Context, ProviderAction, func() error) error
	}
	Providers struct{ Battery, Network, Audio, Music, Spotify Provider }
	Deps      struct {
		Providers Providers
		Changed   func([]InstanceState)
		Now       func() time.Time
	}
	ActionOption struct {
		Token string `json:"token"`
		Label string `json:"label"`
	}
	ActionOptions struct {
		Options []ActionOption `json:"options"`
		Range   *NumberRange   `json:"range,omitempty"`
	}
)

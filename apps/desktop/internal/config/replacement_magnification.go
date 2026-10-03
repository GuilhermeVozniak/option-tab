package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
)

// LauncherMagnification is the H06-only profile setting. Nil is legacy/off.
type LauncherMagnification struct {
	Enabled bool    `json:"enabled"`
	Scale   float64 `json:"scale"`
	Reach   int     `json:"reach"`
}

func DefaultLauncherMagnification() *LauncherMagnification {
	return &LauncherMagnification{Scale: 1.35, Reach: 2}
}

func (m *LauncherMagnification) UnmarshalJSON(raw []byte) error {
	type plain LauncherMagnification
	value := plain(*DefaultLauncherMagnification())
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("config: invalid magnification")
	}
	for _, field := range fields {
		if bytes.Equal(bytes.TrimSpace(field), []byte("null")) {
			return errors.New("config: invalid magnification")
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	*m = LauncherMagnification(value)
	return nil
}

func validLauncherMagnification(m *LauncherMagnification) bool {
	return m == nil || (!math.IsNaN(m.Scale) && !math.IsInf(m.Scale, 0) && m.Scale >= 1 && m.Scale <= 2 && m.Reach >= 0 && m.Reach <= 4)
}

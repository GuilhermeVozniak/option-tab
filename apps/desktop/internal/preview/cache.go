package preview

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"slices"
	"strings"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type captureIdentitySource interface {
	WindowIdentity(domain.WindowID) (platform.AutomationWindowIdentity, error)
	WindowIdentityCurrent(platform.AutomationWindowIdentity) bool
}

// CachedFrame is an immutable snapshot of a previously admitted capture.
// Its timestamp describes capture receipt, not freshness of the current window.
type CachedFrame struct {
	Window     platform.AutomationWindowIdentity
	CapturedAt time.Time
	PNG        []byte
}

func (m *Manager) captureIdentity(id domain.WindowID) platform.AutomationWindowIdentity {
	source, ok := m.source.(captureIdentitySource)
	if !ok {
		return platform.AutomationWindowIdentity{}
	}
	identity, err := source.WindowIdentity(id)
	if err != nil || identity.ID != id || identity.Process.PID <= 0 || identity.Process.StartSeconds == 0 || !source.WindowIdentityCurrent(identity) {
		return platform.AutomationWindowIdentity{}
	}
	return identity
}

func (m *Manager) exportFrame(identity platform.AutomationWindowIdentity, url string) *CachedFrame {
	const prefix = "data:image/png;base64,"
	if identity.ID == 0 || len(url) > 512*1024 || !strings.HasPrefix(url, prefix) {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		return nil
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 {
		return nil
	}
	// Never perform AX/native lookups in a frame callback. The capture job owns
	// this identity; positive source retirement clears it, and the query service
	// revalidates the exact identity immediately before exporting the bytes.
	return &CachedFrame{Window: identity, CapturedAt: time.Now(), PNG: data}
}

// CachedFrames copies existing data without native lookups, capture, or decode.
// The query consumer must revalidate identity and enforce its freshness policy.
func (m *Manager) CachedFrames() []CachedFrame {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]CachedFrame, 0, len(m.exportCache))
	for _, frame := range m.exportCache {
		frame.PNG = slices.Clone(frame.PNG)
		result = append(result, frame)
	}
	return result
}

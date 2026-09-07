//go:build darwin

package platform

import "context"

// This check uses only numeric CG/process ownership and already-observed
// destruction. Never perform AX messaging from the capture delivery callback.
func captureIdentityCurrent(id AutomationWindowIdentity) bool {
	expected := windowIdentity{Window: id.ID, PID: int(id.Process.PID), StartSec: id.Process.StartSeconds, StartUsec: id.Process.StartMicros}
	if !expected.valid() || nativeWindowIdentity(id.ID) != expected {
		return false
	}
	retiredWindows.mu.Lock()
	retired := retiredWindows.windows[id.ID]
	retiredWindows.mu.Unlock()
	return retired.identity != expected || !retired.retired
}

func (p *darwinPlatform) StreamWindowWithIdentity(parent context.Context, id AutomationWindowIdentity, px int, frame func(string)) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if !captureIdentityCurrent(id) {
		return WindowUnavailableError{}
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	retired := false
	err := p.StreamWindow(ctx, id.ID, px, func(url string) {
		if !captureIdentityCurrent(id) {
			retired = true
			cancel()
			return
		}
		if ctx.Err() == nil {
			frame(url)
		}
	})
	if retired {
		return WindowUnavailableError{}
	}
	return err
}

func (p *darwinPlatform) ThumbnailDataURLWithIdentity(ctx context.Context, id AutomationWindowIdentity, px int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !captureIdentityCurrent(id) {
		return "", WindowUnavailableError{}
	}
	url := p.ThumbnailDataURL(id.ID, px)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !captureIdentityCurrent(id) {
		return "", WindowUnavailableError{}
	}
	return url, nil
}

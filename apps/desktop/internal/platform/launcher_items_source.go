package platform

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/url"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/draw"
)

type LauncherItemError struct{ Code string }

func (e *LauncherItemError) Error() string { return "launcher item: " + e.Code }
func launcherItemError(code string) error  { return &LauncherItemError{Code: code} }

type launcherItemLifetime struct {
	mu       sync.Mutex
	closed   bool
	sequence uint64
	cancels  map[uint64]context.CancelFunc
	jobs     sync.WaitGroup
}

func newLauncherItemLifetime() *launcherItemLifetime {
	return &launcherItemLifetime{cancels: make(map[uint64]context.CancelFunc)}
}

func (l *launcherItemLifetime) begin(parent context.Context) (context.Context, func(), error) {
	if parent == nil {
		return nil, nil, launcherItemError("invalidArgument")
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		cancel()
		return nil, nil, launcherItemError("retired")
	}
	l.sequence++
	id := l.sequence
	l.cancels[id] = cancel
	l.jobs.Add(1)
	l.mu.Unlock()
	return ctx, func() { cancel(); l.mu.Lock(); delete(l.cancels, id); l.mu.Unlock(); l.jobs.Done() }, nil
}

func (l *launcherItemLifetime) close() {
	l.mu.Lock()
	l.closed = true
	for _, cancel := range l.cancels {
		cancel()
	}
	l.mu.Unlock()
	l.jobs.Wait()
}

func validLauncherLink(raw string) bool {
	if len(raw) == 0 || len(raw) > 2048 || !utf8.ValidString(raw) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil && u.Opaque == ""
}

func normalizeLauncherIcon(ctx context.Context, data []byte) ([]byte, error) {
	if ctx == nil {
		return nil, launcherItemError("invalidArgument")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > 2<<20 {
		return nil, launcherItemError("tooLarge")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 1024 || cfg.Height > 1024 || cfg.Width*cfg.Height > 1_000_000 {
		return nil, launcherItemError("invalidIcon")
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, launcherItemError("invalidIcon")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height
	if max(w, h) > 256 {
		if w >= h {
			h = max(1, h*256/w)
			w = 256
		} else {
			w = max(1, w*256/h)
			h = 256
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var out bytes.Buffer
	if err = png.Encode(&out, dst); err != nil {
		return nil, errors.New("launcher icon encoding failed")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if out.Len() > 512<<10 {
		return nil, launcherItemError("tooLarge")
	}
	return out.Bytes(), nil
}

func boundedLauncherLabel(label, fallback string) string {
	label = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, label))
	if label == "" {
		label = fallback
	}
	runes := []rune(label)
	return string(runes[:min(len(runes), 80)])
}

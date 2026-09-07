package media

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"image/png"
	"slices"
	"sync"
	"time"

	"option-tab/internal/platform"
)

var ErrArtworkBusy = errors.New("artwork requests are busy")

type artworkKey struct {
	scope platform.MediaScope
	token string
}
type artworkEntry struct {
	key  artworkKey
	data []byte
}

// ArtworkCache retains normalized PNG bytes in memory only. Callers supply the
// current presentation guard, including on cache hits; no cache entry grants
// authority to publish into a retired panel.
type ArtworkCache struct {
	mu               sync.Mutex
	source           platform.MediaProviderSource
	entries          map[artworkKey]*list.Element
	order            *list.List
	bytes, limit     int
	generation, next uint64
	pending          map[uint64]context.CancelFunc
	slots            chan struct{}
}

func NewArtworkCache(source platform.MediaProviderSource) *ArtworkCache {
	return newArtworkCache(source, 20*1024*1024)
}

func newArtworkCache(source platform.MediaProviderSource, limit int) *ArtworkCache {
	return &ArtworkCache{source: source, entries: map[artworkKey]*list.Element{}, order: list.New(), limit: max(0, limit), pending: map[uint64]context.CancelFunc{}, slots: make(chan struct{}, 2)}
}

func (c *ArtworkCache) Read(ctx context.Context, scope platform.MediaScope, token string, remoteAllowed bool, guard func() error) (platform.MediaArtwork, error) {
	if ctx == nil || guard == nil {
		return platform.MediaArtwork{}, errors.New("artwork context and presentation guard are required")
	}
	if err := ctx.Err(); err != nil {
		return platform.MediaArtwork{}, err
	}
	if err := guard(); err != nil {
		return platform.MediaArtwork{}, err
	}
	if scope.Provider != platform.MediaMusic && scope.Provider != platform.MediaSpotify {
		return platform.MediaArtwork{}, errors.New("unsupported artwork provider")
	}
	if scope.Provider == platform.MediaSpotify && !remoteAllowed {
		return platform.MediaArtwork{Status: "networkDisabled"}, nil
	}
	if scope.Process.PID <= 0 || scope.Process.LaunchID == "" || scope.Generation == 0 || scope.TrackEpoch == 0 || scope.TrackID == "" || token == "" {
		return platform.MediaArtwork{Status: "missing"}, nil
	}
	key := artworkKey{scope, token}
	c.mu.Lock()
	generation := c.generation
	if entry := c.entries[key]; entry != nil {
		c.order.MoveToFront(entry)
		data := slices.Clone(entry.Value.(artworkEntry).data)
		c.mu.Unlock()
		if err := guard(); err != nil {
			return platform.MediaArtwork{}, err
		}
		if err := ctx.Err(); err != nil {
			return platform.MediaArtwork{}, err
		}
		c.mu.Lock()
		current := c.generation == generation
		c.mu.Unlock()
		if !current {
			return platform.MediaArtwork{}, ErrRetired
		}
		return platform.MediaArtwork{PNG: data, Status: "ready"}, nil
	}
	if c.source == nil {
		c.mu.Unlock()
		return platform.MediaArtwork{Status: "unavailable"}, errors.New("artwork source is unavailable")
	}
	select {
	case c.slots <- struct{}{}:
	default:
		c.mu.Unlock()
		return platform.MediaArtwork{}, ErrArtworkBusy
	}
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	c.next++
	operation := c.next
	c.pending[operation] = cancel
	c.mu.Unlock()
	defer func() { cancel(); c.mu.Lock(); delete(c.pending, operation); c.mu.Unlock(); <-c.slots }()
	result, err := c.source.ReadMediaArtwork(opCtx, scope, token)
	if opCtx.Err() != nil {
		return platform.MediaArtwork{}, opCtx.Err()
	}
	if err := guard(); err != nil {
		return platform.MediaArtwork{}, err
	}
	if opCtx.Err() != nil {
		return platform.MediaArtwork{}, opCtx.Err()
	}
	if err == nil && result.Status == "ready" {
		if len(result.PNG) == 0 || len(result.PNG) > 5*1024*1024 {
			return platform.MediaArtwork{}, errors.New("artwork source returned an oversized image")
		}
		config, err := png.DecodeConfig(bytes.NewReader(result.PNG))
		if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 512 || config.Height > 512 {
			return platform.MediaArtwork{}, errors.New("artwork source returned an invalid image")
		}
	} else {
		result.PNG = nil
	}
	result.PNG = slices.Clone(result.PNG)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != generation {
		return platform.MediaArtwork{}, ErrRetired
	}
	if opCtx.Err() != nil {
		return platform.MediaArtwork{}, opCtx.Err()
	}
	if err != nil {
		return result, err
	}
	if result.Status == "ready" {
		c.putLocked(key, result.PNG)
	}
	return result, nil
}

func (c *ArtworkCache) putLocked(key artworkKey, data []byte) {
	if len(data) > c.limit {
		return
	}
	if previous := c.entries[key]; previous != nil {
		c.removeLocked(previous)
	}
	for c.bytes+len(data) > c.limit {
		c.removeLocked(c.order.Back())
	}
	entry := artworkEntry{key, slices.Clone(data)}
	c.entries[key] = c.order.PushFront(entry)
	c.bytes += len(data)
}

func (c *ArtworkCache) removeLocked(entry *list.Element) {
	value := entry.Value.(artworkEntry)
	c.bytes -= len(value.data)
	delete(c.entries, value.key)
	c.order.Remove(entry)
}

// Clear retires in-flight results before requesting cancellation. Existing
// requests hold their bounded slots until the source has finished cleanup.
func (c *ArtworkCache) Clear() {
	c.mu.Lock()
	c.generation++
	c.entries = map[artworkKey]*list.Element{}
	c.order.Init()
	c.bytes = 0
	cancels := make([]context.CancelFunc, 0, len(c.pending))
	for _, cancel := range c.pending {
		cancels = append(cancels, cancel)
	}
	c.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

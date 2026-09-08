//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "darwin_media_lyrics.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unsafe"
)

type nativeMediaLyrics struct{}

func NewMediaLyricsSource(path string) MediaLyricsSource {
	return newMediaLyricsSource(path, nativeMediaLyrics{})
}

func decodeLyricsReply(raw *C.char) (lyricsSelection, error) {
	if raw == nil {
		return lyricsSelection{}, errors.New("lyric file unavailable")
	}
	defer C.free(unsafe.Pointer(raw))
	var reply struct {
		Data, Bookmark []byte
		Device, Inode  uint64
		Error          string
		Cancelled      bool
	}
	if err := json.Unmarshal([]byte(C.GoString(raw)), &reply); err != nil {
		return lyricsSelection{}, err
	}
	if reply.Cancelled {
		return lyricsSelection{}, context.Canceled
	}
	if reply.Error != "" {
		return lyricsSelection{}, errors.New(reply.Error)
	}
	return lyricsSelection{Record: lyricsRecord{Bookmark: reply.Bookmark, Device: reply.Device, Inode: reply.Inode}, Data: reply.Data}, nil
}

func (nativeMediaLyrics) choose(ctx context.Context) (lyricsSelection, error) {
	if C.ot_lyrics_main_thread() == 1 {
		return lyricsSelection{}, errors.New("lyric selection must run off the native UI thread")
	}
	if err := ctx.Err(); err != nil {
		return lyricsSelection{}, err
	}
	handle := C.ot_lyrics_choose_start()
	if handle == nil {
		return lyricsSelection{}, errors.New("lyric chooser unavailable")
	}
	defer C.ot_lyrics_choose_release(handle)
	timer := time.NewTicker(20 * time.Millisecond)
	defer timer.Stop()
	cancelled := false
	for {
		if ctx.Err() != nil && !cancelled {
			cancelled = true
			C.ot_lyrics_choose_cancel(handle)
		}
		raw := C.ot_lyrics_choose_poll(handle)
		if raw != nil {
			selected, err := decodeLyricsReply(raw)
			if ctx.Err() != nil {
				return lyricsSelection{}, ctx.Err()
			}
			return selected, err
		}
		<-timer.C
	}
}

func (nativeMediaLyrics) read(ctx context.Context, record lyricsRecord) ([]byte, error) {
	if C.ot_lyrics_main_thread() == 1 {
		return nil, errors.New("lyric reading must run off the native UI thread")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bytes := C.CBytes(record.Bookmark)
	defer C.free(bytes)
	selected, err := decodeLyricsReply(C.ot_lyrics_read(bytes, C.int(len(record.Bookmark)), C.uint64_t(record.Device), C.uint64_t(record.Inode)))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return selected.Data, err
}

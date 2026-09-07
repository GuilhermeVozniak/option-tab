//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework Carbon
#include <stdlib.h>
#include "darwin_media.h"
*/
import "C"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"runtime/cgo"
	"unsafe"
)

type darwinMediaTransport struct{}

func NewMediaSource() MediaProviderSource { return newMediaSource(darwinMediaTransport{}) }
func mediaCString(s string) (*C.char, func()) {
	p := C.CString(s)
	return p, func() { C.free(unsafe.Pointer(p)) }
}

func mediaJSON(p *C.char, v any) error {
	if p == nil {
		return errors.New("media native reply missing")
	}
	defer C.free(unsafe.Pointer(p))
	return json.Unmarshal([]byte(C.GoString(p)), v)
}

func (darwinMediaTransport) read(ctx context.Context, p MediaProvider) (MediaSample, string, error) {
	if err := ctx.Err(); err != nil {
		return MediaSample{}, "", err
	}
	name, free := mediaCString(string(p))
	defer free()
	var reply struct {
		MediaSample
		Artwork string `json:"artwork"`
	}
	err := mediaJSON(C.ot_media_read(name), &reply)
	return reply.MediaSample, reply.Artwork, err
}

func (darwinMediaTransport) permission(ctx context.Context, p MediaProvider, ask bool) (MediaPermission, error) {
	if err := ctx.Err(); err != nil {
		return MediaPermission{}, err
	}
	name, free := mediaCString(string(p))
	defer free()
	flag := 0
	if ask {
		flag = 1
	}
	var reply MediaPermission
	err := mediaJSON(C.ot_media_permission(name, C.int(flag)), &reply)
	if ctx.Err() != nil {
		return MediaPermission{}, ctx.Err()
	}
	return reply, err
}

type mediaGuardCall struct {
	guard func() error
	err   error
}

//export ot_media_guard
func ot_media_guard(token C.uintptr_t) C.int {
	call := cgo.Handle(token).Value().(*mediaGuardCall)
	call.err = call.guard()
	if call.err != nil {
		return 0
	}
	return 1
}

func (darwinMediaTransport) command(ctx context.Context, c MediaCommand, guard func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, fp := mediaCString(string(c.Scope.Provider))
	defer fp()
	b, fb := mediaCString(c.Scope.Process.LaunchID)
	defer fb()
	t, ft := mediaCString(c.Scope.TrackID)
	defer ft()
	k, fk := mediaCString(c.Kind)
	defer fk()
	call := &mediaGuardCall{guard: guard}
	handle := cgo.NewHandle(call)
	defer handle.Delete()
	result := C.ot_media_command(p, C.int(c.Scope.Process.PID), b, t, k, C.int64_t(c.PositionMS), C.uintptr_t(handle))
	if result != nil {
		defer C.free(unsafe.Pointer(result))
		if call.err != nil {
			return call.err
		}
		return errors.New(C.GoString(result))
	}
	return nil
}

func (darwinMediaTransport) artwork(ctx context.Context, scope MediaScope, token string) ([]byte, error) {
	if scope.Provider == MediaSpotify {
		return readRemoteMediaArtwork(ctx, token)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, fp := mediaCString(string(scope.Provider))
	defer fp()
	b, fb := mediaCString(scope.Process.LaunchID)
	defer fb()
	t, ft := mediaCString(scope.TrackID)
	defer ft()
	result := C.ot_media_artwork(p, C.int(scope.Process.PID), b, t)
	if result == nil {
		return nil, errors.New("player artwork missing")
	}
	defer C.free(unsafe.Pointer(result))
	data, err := base64.StdEncoding.DecodeString(C.GoString(result))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return normalizeMediaArtworkPNG(data)
}

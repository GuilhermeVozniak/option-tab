//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework Cocoa -framework Carbon
#include <stdlib.h>
#include "darwin_automation.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"

	"option-tab/internal/domain"
)

type automationPacket struct {
	Deadline    time.Time
	Request     AutomationRequest
	RemainingMS int64
}
type automationNative interface {
	start() uint64
	status(uint64) int
	pop(uint64) *automationPacket
	current(uint64, uint64) bool
	complete(uint64, uint64, AutomationReply)
	stop(uint64)
	drain(uint64)
}
type automationServer struct {
	mu               sync.Mutex
	native           automationNative
	started, stopped bool
	token            uint64
	cancel           context.CancelFunc
}

func NewAutomationServer() AutomationServer { return newAutomationServer(automationDarwinNative{}) }

func newAutomationServer(n automationNative) *automationServer { return &automationServer{native: n} }

func (s *automationServer) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	cancel, id := s.cancel, s.token
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if id != 0 {
		s.native.stop(id)
	}
}

func (s *automationServer) DrainOnMainThread() {
	s.Stop()
	s.mu.Lock()
	id := s.token
	s.mu.Unlock()
	if id != 0 {
		s.native.drain(id)
	}
}

func (s *automationServer) Run(ctx context.Context, handler AutomationHandler) error {
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.stopped {
		s.mu.Unlock()
		return context.Canceled
	}
	if s.started || handler == nil {
		s.mu.Unlock()
		return errors.New("automation server already started or missing handler")
	}
	s.started = true
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	// start only allocates a token and queues registration; it never waits for main.
	id := s.native.start()
	s.token = id
	s.mu.Unlock()
	defer cancel()
	if id == 0 {
		s.Stop()
		return errors.New("automation native owner unavailable")
	}
	var workers sync.WaitGroup
	defer func() {
		s.Stop()
		workers.Wait()
		for s.native.status(id) >= 0 {
			time.Sleep(time.Millisecond)
		}
	}()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	slots := make(chan struct{}, 16)
	for {
		if runCtx.Err() != nil {
			return runCtx.Err()
		}
		status := s.native.status(id)
		if status < 0 {
			return errors.New("automation native transport stopped or handler conflict")
		}
		if status == 1 {
			if packet := s.native.pop(id); packet != nil {
				if packet.RemainingMS <= 0 || !s.native.current(id, packet.Request.ID) {
					continue
				}
				select {
				case slots <- struct{}{}:
					workers.Add(1)
					go func(p automationPacket) {
						defer workers.Done()
						defer func() { <-slots }()
						s.execute(runCtx, id, p, handler)
					}(*packet)
				default:
					s.native.complete(id, packet.Request.ID, AutomationReply{ErrorCode: "busy", ErrorMessage: "Automation worker capacity reached"})
				}
				continue
			}
		}
		select {
		case <-runCtx.Done():
			return runCtx.Err()
		case <-tick.C:
		}
	}
}

type automationRequestContext struct {
	context.Context
	current func() bool
	cancel  context.CancelFunc
}

func (c automationRequestContext) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	if !c.current() {
		c.cancel()
	}
	return c.Context.Err()
}

func (s *automationServer) execute(parent context.Context, server uint64, p automationPacket, handler AutomationHandler) {
	budget := time.Duration(p.RemainingMS) * time.Millisecond
	if budget > 5*time.Second {
		budget = 5 * time.Second
	}
	deadline := p.Deadline
	if deadline.IsZero() {
		deadline = time.Now().Add(budget)
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	scoped := automationRequestContext{Context: ctx, cancel: cancel, current: func() bool { return s.native.current(server, p.Request.ID) }}
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !scoped.current() {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-monitorDone }()
	if scoped.Err() != nil {
		return
	}
	reply := func() (result AutomationReply) {
		defer func() {
			if recover() != nil {
				result = AutomationReply{ErrorCode: "internal", ErrorMessage: "Automation handler failed"}
			}
		}()
		return handler(scoped, p.Request)
	}()
	if scoped.Err() != nil {
		return
	}
	if len(reply.JSON) > 4<<20 || (reply.ErrorCode == "" && (!json.Valid(reply.JSON) || !utf8.Valid(reply.JSON))) {
		reply = AutomationReply{ErrorCode: "internal", ErrorMessage: "Invalid or oversized automation reply"}
	}
	if len(reply.ErrorMessage) > 1024 {
		reply.ErrorMessage = reply.ErrorMessage[:1024]
	}
	reply.ErrorMessage = strings.ToValidUTF8(reply.ErrorMessage, "?")
	switch reply.ErrorCode {
	case "", "invalidArgument", "ambiguous", "notFound", "unavailable", "permissionDenied", "unsupported", "staleIdentity", "retired", "cancelled", "timeout", "busy", "internal":
	default:
		reply = AutomationReply{ErrorCode: "internal", ErrorMessage: "Unknown automation failure"}
	}
	reply.JSON = append([]byte(nil), reply.JSON...)
	s.native.complete(server, p.Request.ID, reply)
}

type automationDarwinNative struct{}

func (automationDarwinNative) start() uint64 { return uint64(C.ot_automation_start()) }
func (automationDarwinNative) status(id uint64) int {
	return int(C.ot_automation_status(C.uint64_t(id)))
}
func (automationDarwinNative) stop(id uint64)  { C.ot_automation_stop(C.uint64_t(id)) }
func (automationDarwinNative) drain(id uint64) { C.ot_automation_drain_main(C.uint64_t(id)) }
func (automationDarwinNative) current(server, id uint64) bool {
	return C.ot_automation_current(C.uint64_t(server), C.uint64_t(id)) != 0
}

func (automationDarwinNative) pop(id uint64) *automationPacket {
	beforePop := time.Now()
	raw := C.ot_automation_pop(C.uint64_t(id))
	if raw == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(raw))
	var wire struct {
		ID                                                        uint64
		RemainingMS                                               int64
		Operation                                                 AutomationOperation
		Name, BundleID, WindowID, Action, Mode, PresentationToken string
		PID                                                       int
		ActiveWindow, IncludeImages                               bool
		Fullscreen                                                *bool
		Position                                                  *AutomationPoint
	}
	if json.Unmarshal([]byte(C.GoString(raw)), &wire) != nil {
		return nil
	}
	req := AutomationRequest{ID: wire.ID, Operation: wire.Operation, Action: wire.Action, Mode: wire.Mode, PresentationToken: wire.PresentationToken, ActiveWindow: wire.ActiveWindow, IncludeImages: wire.IncludeImages, Fullscreen: wire.Fullscreen, Position: wire.Position}
	if wire.Name != "" || wire.BundleID != "" || wire.PID != 0 {
		req.App = &AutomationAppSelector{Name: wire.Name, BundleID: wire.BundleID, PID: domain.AppID(wire.PID)}
	}
	if wire.WindowID != "" {
		value, err := strconv.ParseUint(wire.WindowID, 10, 32)
		if err != nil {
			return nil
		}
		req.WindowID = domain.WindowID(value)
	}
	return &automationPacket{Request: req, RemainingMS: wire.RemainingMS, Deadline: beforePop.Add(time.Duration(wire.RemainingMS) * time.Millisecond)}
}

func (automationDarwinNative) complete(server, id uint64, r AutomationReply) {
	data := C.CString(string(r.JSON))
	code := C.CString(r.ErrorCode)
	message := C.CString(r.ErrorMessage)
	defer C.free(unsafe.Pointer(data))
	defer C.free(unsafe.Pointer(code))
	defer C.free(unsafe.Pointer(message))
	C.ot_automation_complete(C.uint64_t(server), C.uint64_t(id), data, code, message)
}

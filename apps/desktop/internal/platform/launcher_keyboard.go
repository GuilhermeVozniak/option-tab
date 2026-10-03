package platform

import (
	"errors"
	"strings"
	"sync"
)

type LauncherKeyboardPolicy struct {
	Epoch, Session, Revision, Admission uint64
	DisplayUUID                         string
	Enabled                             bool
}

// finalGuard executes synchronously on AppKit after native preparation. It must
// be bounded Go-only work, with no lock whose owner can wait for AppKit.
type LauncherKeyboardSource interface {
	SetLauncherKeyboardPolicy(LauncherKeyboardPolicy, func() bool) error
}
type LauncherKeyboardValidator interface {
	ValidateLauncherKeyboard(epoch, session, revision, admission uint64) bool
}
type launcherKeyboardNative interface {
	Set(uint64, LauncherKeyboardPolicy, func() bool) error
	Valid(uint64, LauncherKeyboardPolicy) bool
}
type launcherKeyboardOwner struct {
	mu            sync.Mutex
	token, serial uint64
	closed        bool
	policy        LauncherKeyboardPolicy
	native        launcherKeyboardNative
}

func newLauncherKeyboardOwner(token uint64, n launcherKeyboardNative) *launcherKeyboardOwner {
	return &launcherKeyboardOwner{token: token, native: n}
}

func (o *launcherKeyboardOwner) set(p LauncherKeyboardPolicy, guard func() bool) error {
	if p.Epoch == 0 || p.Session == 0 || p.Revision == 0 || p.Admission == 0 || p.DisplayUUID == "" || len(p.DisplayUUID) > 127 || strings.ContainsRune(p.DisplayUUID, 0) || (p.Enabled && guard == nil) {
		return errors.New("invalid launcher keyboard policy")
	}
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return ErrDockPanelClosed
	}
	serial := o.serial
	o.mu.Unlock()
	current := func() bool {
		o.mu.Lock()
		ok := !o.closed && o.serial == serial && p.Admission >= o.policy.Admission
		o.mu.Unlock()
		if !ok || (guard != nil && !guard()) {
			return false
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		return !o.closed && o.serial == serial && p.Admission >= o.policy.Admission
	}
	if err := o.native.Set(o.token, p, current); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.serial != serial || p.Admission < o.policy.Admission {
		return ErrDockPanelClosed
	}
	o.policy = p
	return nil
}

func (o *launcherKeyboardOwner) retire(closed bool) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.serial++
	o.policy.Enabled = false
	o.closed = o.closed || closed
}

func (o *launcherKeyboardOwner) valid(p LauncherKeyboardPolicy) bool {
	o.mu.Lock()
	serial := o.serial
	ok := !o.closed && o.policy.Enabled && o.policy == p
	o.mu.Unlock()
	if !ok || !o.native.Valid(o.token, p) {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.closed && o.serial == serial && o.policy == p
}

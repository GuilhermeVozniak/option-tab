// Package hotkey parses and canonicalizes keyboard-shortcut chords such as
// "option+tab" into a structured Chord (a set of modifiers plus one key). It is
// pure: the platform layer maps a Chord to OS keycodes for registration.
package hotkey

import (
	"fmt"
	"strings"
)

// Modifier is a single modifier key.
type Modifier uint8

const (
	ModControl Modifier = iota
	ModOption
	ModShift
	ModCommand
)

// canonicalOrder is the stable serialization order of modifiers.
var canonicalOrder = []Modifier{ModControl, ModOption, ModShift, ModCommand}

var modNames = map[Modifier]string{
	ModControl: "control",
	ModOption:  "option",
	ModShift:   "shift",
	ModCommand: "command",
}

// modAliases maps accepted spellings to a Modifier.
var modAliases = map[string]Modifier{
	"control": ModControl, "ctrl": ModControl,
	"option": ModOption, "alt": ModOption, "opt": ModOption,
	"shift":   ModShift,
	"command": ModCommand, "cmd": ModCommand, "super": ModCommand, "meta": ModCommand, "win": ModCommand,
}

// ModSet is a set of modifiers.
type ModSet uint8

// Has reports whether m is in the set.
func (s ModSet) Has(m Modifier) bool { return s&(1<<m) != 0 }

// With returns a copy with m added.
func (s ModSet) With(m Modifier) ModSet { return s | (1 << m) }

// Without returns a copy with m removed.
func (s ModSet) Without(m Modifier) ModSet { return s &^ (1 << m) }

// Len returns the number of modifiers set.
func (s ModSet) Len() int {
	n := 0
	for _, m := range canonicalOrder {
		if s.Has(m) {
			n++
		}
	}
	return n
}

// Key is a normalized, non-modifier key name (e.g. "tab", "grave", "a", "1").
type Key string

// validKeys is the set of keys a chord may bind to. It covers the keys AltTab
// users actually bind: letters, digits, tab/space/escape/return, grave, and
// arrows.
var validKeys = func() map[Key]bool {
	m := map[Key]bool{
		"tab": true, "space": true, "escape": true, "return": true,
		"grave": true, "left": true, "right": true, "up": true, "down": true,
	}
	for c := 'a'; c <= 'z'; c++ {
		m[Key(string(c))] = true
	}
	for c := '0'; c <= '9'; c++ {
		m[Key(string(c))] = true
	}
	return m
}()

// keyAliases normalizes alternative key spellings.
var keyAliases = map[string]Key{
	"`": "grave", "backtick": "grave", "esc": "escape", "enter": "return",
	"arrowleft": "left", "arrowright": "right", "arrowup": "up", "arrowdown": "down",
}

// Chord is a set of modifiers plus exactly one key.
type Chord struct {
	Mods ModSet
	Key  Key
}

// Parse converts a string like "control+option+a" into a Chord. It is
// case-insensitive, tolerates surrounding spaces, accepts modifier aliases
// (alt=option, cmd/super=command, ctrl=control), and requires at least one
// modifier and exactly one non-modifier key.
func Parse(s string) (Chord, error) {
	parts := strings.Split(s, "+")
	var c Chord
	keyCount := 0
	for _, raw := range parts {
		tok := strings.ToLower(strings.TrimSpace(raw))
		if tok == "" {
			return Chord{}, fmt.Errorf("hotkey: empty token in %q", s)
		}
		if m, ok := modAliases[tok]; ok {
			if c.Mods.Has(m) {
				return Chord{}, fmt.Errorf("hotkey: duplicate modifier %q in %q", tok, s)
			}
			c.Mods = c.Mods.With(m)
			continue
		}
		if alias, ok := keyAliases[tok]; ok {
			tok = string(alias)
		}
		if !validKeys[Key(tok)] {
			return Chord{}, fmt.Errorf("hotkey: unknown key %q in %q", tok, s)
		}
		keyCount++
		c.Key = Key(tok)
	}
	if keyCount != 1 {
		return Chord{}, fmt.Errorf("hotkey: chord %q must have exactly one key, got %d", s, keyCount)
	}
	if c.Mods.Len() == 0 {
		return Chord{}, fmt.Errorf("hotkey: chord %q must have at least one modifier", s)
	}
	return c, nil
}

// String serializes the chord in canonical form: modifiers in fixed order
// (control, option, shift, command) followed by the key, joined by "+".
func (c Chord) String() string {
	var sb []string
	for _, m := range canonicalOrder {
		if c.Mods.Has(m) {
			sb = append(sb, modNames[m])
		}
	}
	sb = append(sb, string(c.Key))
	return strings.Join(sb, "+")
}

// Modifiers returns the chord's modifiers in canonical order.
func (c Chord) Modifiers() []Modifier {
	var out []Modifier
	for _, m := range canonicalOrder {
		if c.Mods.Has(m) {
			out = append(out, m)
		}
	}
	return out
}

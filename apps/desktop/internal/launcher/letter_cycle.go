package launcher

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const LetterCycleIdle = 750 * time.Millisecond

type (
	LetterItem struct {
		ID, Name            string
		Visible, Actionable bool
	}
	LetterPacket struct {
		Scope
		Admission, Sequence uint64
		Timestamp           time.Time
		Key                 string
		Modifiers           uint64
		Composing           bool
	}
	LetterCycle struct {
		scope               Scope
		enabled             bool
		admission, sequence uint64
		last                time.Time
		key, selected       string
	}
)

func (r *LetterCycle) SetScope(scope Scope, enabled bool) uint64 {
	if r.admission != 0 && r.scope == scope && r.enabled == enabled {
		return r.admission
	}
	r.scope = scope
	r.enabled = enabled
	r.admission++
	r.sequence = 0
	r.last = time.Time{}
	r.key, r.selected = "", ""
	return r.admission
}

func (r *LetterCycle) Step(p LetterPacket, items []LetterItem) string {
	if p.Scope != r.scope || p.Admission != r.admission || !validGestureScope(p.Scope) || p.Sequence == 0 || p.Sequence <= r.sequence {
		return ""
	}
	r.sequence = p.Sequence
	key, n := utf8.DecodeRuneInString(p.Key)
	if !r.enabled || p.Composing || p.Modifiers != 0 || n != len(p.Key) || !unicode.IsLetter(key) || p.Timestamp.IsZero() || (!r.last.IsZero() && p.Timestamp.Before(r.last)) {
		return ""
	}
	matches := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		first, _ := utf8.DecodeRuneInString(item.Name)
		if item.ID != "" && item.Visible && item.Actionable && strings.EqualFold(string(first), p.Key) && !seen[item.ID] {
			matches = append(matches, item.ID)
			seen[item.ID] = true
		}
	}
	selected := ""
	if len(matches) > 0 {
		index := 0
		if strings.EqualFold(r.key, p.Key) && !r.last.IsZero() && p.Timestamp.Sub(r.last) < LetterCycleIdle {
			for i, id := range matches {
				if id == r.selected {
					index = (i + 1) % len(matches)
					break
				}
			}
		}
		selected = matches[index]
	}
	r.last, r.key, r.selected = p.Timestamp, p.Key, selected
	return selected
}

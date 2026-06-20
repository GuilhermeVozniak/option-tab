// Package search provides fuzzy, case-insensitive subsequence matching used by
// the switcher's type-to-filter box. It ranks matches so the most relevant
// windows appear first, and never mutates its inputs.
package search

import (
	"sort"
	"strings"

	"option-tab/internal/domain"
)

const (
	matchBonus       = 1  // each matched rune
	consecutiveBonus = 5  // matched rune immediately follows the previous match
	prefixBonus      = 10 // first match is at the very start of the text
)

// Match reports whether query is a case-insensitive subsequence of text and, if
// so, a relevance score (higher is better). An empty query matches everything
// with score 0. Contiguous matches and matches starting at the beginning of the
// text score higher.
func Match(text, query string) (score int, ok bool) {
	if query == "" {
		return 0, true
	}
	t := []rune(strings.ToLower(text))
	q := []rune(strings.ToLower(query))
	if len(q) > len(t) {
		return 0, false
	}

	qi := 0
	prevMatch := -2
	first := -1
	for i := 0; i < len(t) && qi < len(q); i++ {
		if t[i] != q[qi] {
			continue
		}
		if first < 0 {
			first = i
		}
		if i == prevMatch+1 {
			score += consecutiveBonus
		} else {
			score += matchBonus
		}
		prevMatch = i
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	if first == 0 {
		score += prefixBonus
	}
	score -= first // earlier first match is better
	return score, true
}

// Filter returns the windows whose app name or title fuzzy-matches query,
// ranked best-first. An empty query returns a copy of all windows in their
// original order.
func Filter(wins []domain.Window, query string) []domain.Window {
	if strings.TrimSpace(query) == "" {
		out := make([]domain.Window, len(wins))
		copy(out, wins)
		return out
	}

	type scored struct {
		w     domain.Window
		score int
		order int
	}
	var matches []scored
	for i, w := range wins {
		best, ok := bestScore(w, query)
		if !ok {
			continue
		}
		matches = append(matches, scored{w: w, score: best, order: i})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].order < matches[j].order
	})

	out := make([]domain.Window, len(matches))
	for i, m := range matches {
		out[i] = m.w
	}
	return out
}

// bestScore returns the higher of the app-name and title match scores.
func bestScore(w domain.Window, query string) (int, bool) {
	appScore, appOK := Match(w.AppName, query)
	titleScore, titleOK := Match(w.Title, query)
	switch {
	case appOK && titleOK:
		if appScore >= titleScore {
			return appScore, true
		}
		return titleScore, true
	case appOK:
		return appScore, true
	case titleOK:
		return titleScore, true
	default:
		return 0, false
	}
}

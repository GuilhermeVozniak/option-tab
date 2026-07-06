package search

import (
	"testing"

	"option-tab/internal/domain"
)

func TestMatch_Subsequence(t *testing.T) {
	tests := []struct {
		text, query string
		want        bool
	}{
		{"Safari", "afi", true},
		{"Safari", "sfr", true}, // subsequence
		{"Safari", "xyz", false},
		{"Safari", "", true}, // empty matches
		{"Visual Studio Code", "vsc", true},
		{"Visual Studio Code", "code", true},
		{"abc", "abcd", false}, // query longer than text
	}
	for _, tt := range tests {
		t.Run(tt.text+"/"+tt.query, func(t *testing.T) {
			_, ok := Match(tt.text, tt.query)
			if ok != tt.want {
				t.Errorf("Match(%q,%q) ok = %v, want %v", tt.text, tt.query, ok, tt.want)
			}
		})
	}
}

func TestMatch_CaseInsensitive(t *testing.T) {
	if _, ok := Match("Safari", "SAF"); !ok {
		t.Error("match should be case-insensitive")
	}
}

func TestMatch_ContiguousScoresHigher(t *testing.T) {
	contig, _ := Match("Safari", "saf")
	spread, _ := Match("Safari", "sfi")
	if contig <= spread {
		t.Errorf("contiguous match (%d) should score higher than spread (%d)", contig, spread)
	}
}

func TestMatch_PrefixScoresHigher(t *testing.T) {
	prefix, _ := Match("Terminal", "ter")
	mid, _ := Match("iTerminal", "ter")
	if prefix <= mid {
		t.Errorf("prefix match (%d) should score higher than mid match (%d)", prefix, mid)
	}
}

func win(id domain.WindowID, app, title string) domain.Window {
	return domain.Window{ID: id, AppName: app, Title: title}
}

func ids(ws []domain.Window) []domain.WindowID {
	out := make([]domain.WindowID, len(ws))
	for i, w := range ws {
		out[i] = w.ID
	}
	return out
}

func TestFilter_EmptyQueryReturnsAllUnchanged(t *testing.T) {
	ws := []domain.Window{win(1, "A", "x"), win(2, "B", "y")}
	got := Filter(ws, "")
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("empty query should return all in order, got %v", ids(got))
	}
}

func TestFilter_WhitespaceQueryReturnsAll(t *testing.T) {
	ws := []domain.Window{win(1, "A", "x"), win(2, "B", "y"), win(3, "C", "z")}
	got := Filter(ws, "   ")
	if len(got) != 3 || got[0].ID != 1 || got[1].ID != 2 || got[2].ID != 3 {
		t.Errorf("whitespace query should return all in order, got %v", ids(got))
	}
}

func TestFilter_TitleScoreCanOutrankAppName(t *testing.T) {
	ws := []domain.Window{
		win(1, "iTunes remote", "x"), // loose app-name subsequence of "term"
		win(2, "Notes", "Terminal"),  // title prefix-matches "term"
	}
	got := Filter(ws, "term")
	if len(got) != 2 || got[0].ID != 2 {
		t.Errorf("title prefix match should outrank loose app-name match, got %v", ids(got))
	}
}

func TestFilter_MatchesTitleOrAppName(t *testing.T) {
	ws := []domain.Window{
		win(1, "Safari", "GitHub"),
		win(2, "Terminal", "zsh"),
		win(3, "Notes", "groceries"),
	}
	got := Filter(ws, "git") // matches Safari's title
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("title match wrong: %v", ids(got))
	}
	got = Filter(ws, "term") // matches Terminal app name
	if len(got) != 1 || got[0].ID != 2 {
		t.Errorf("app-name match wrong: %v", ids(got))
	}
}

func TestFilter_RanksBestMatchFirst(t *testing.T) {
	ws := []domain.Window{
		win(1, "iTerminal", "x"),
		win(2, "Terminal", "x"),
	}
	got := Filter(ws, "term")
	if len(got) != 2 || got[0].ID != 2 {
		t.Errorf("best (prefix) match should rank first, got %v", ids(got))
	}
}

func TestFilter_DropsNonMatches(t *testing.T) {
	ws := []domain.Window{win(1, "Safari", "x"), win(2, "Notes", "y")}
	got := Filter(ws, "zzz")
	if len(got) != 0 {
		t.Errorf("no windows should match, got %v", ids(got))
	}
}

package config

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var launcherIconID = regexp.MustCompile(`^[a-f0-9]{64}$`)

type LauncherItem struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Label       string   `json:"label,omitempty"`
	ReferenceID string   `json:"referenceID,omitempty"`
	URL         string   `json:"url,omitempty"`
	IconID      string   `json:"iconID,omitempty"`
	Members     []string `json:"members,omitempty"`
	FolderView  string   `json:"folderView,omitempty"`
}

func validLauncherLabel(s string, required bool) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && n <= 80 && (!required || n > 0)
}

func validLauncherURL(raw string) bool {
	if len(raw) == 0 || len(raw) > 2048 || !utf8.ValidString(raw) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.IsAbs() && u.Hostname() != "" && u.Opaque == "" && u.User == nil && u.String() == raw
}

func validLauncherItems(items []LauncherItem) bool {
	if len(items) > 16 {
		return false
	}
	byID := make(map[string]LauncherItem, len(items))
	for _, x := range items {
		if !launcherID.MatchString(x.ID) || byID[x.ID].ID != "" || !slices.Contains([]string{"app", "folder", "file", "link", "group", "spacer", "separator"}, x.Kind) || (x.IconID != "" && !launcherIconID.MatchString(x.IconID)) {
			return false
		}
		byID[x.ID] = x
	}
	members := map[string]bool{}
	for _, x := range items {
		emptyMembers := len(x.Members) == 0
		switch x.Kind {
		case "app", "file":
			if !validLauncherLabel(x.Label, true) || !launcherID.MatchString(x.ReferenceID) || x.URL != "" || !emptyMembers || x.FolderView != "" {
				return false
			}
		case "folder":
			if !validLauncherLabel(x.Label, true) || !launcherID.MatchString(x.ReferenceID) || x.URL != "" || !emptyMembers || !slices.Contains([]string{"list", "grid"}, x.FolderView) {
				return false
			}
		case "link":
			if !validLauncherLabel(x.Label, true) || x.ReferenceID != "" || !validLauncherURL(x.URL) || !emptyMembers || x.FolderView != "" {
				return false
			}
		case "group":
			if !validLauncherLabel(x.Label, true) || x.ReferenceID != "" || x.URL != "" || x.FolderView != "" || emptyMembers {
				return false
			}
			seen := map[string]bool{}
			for _, id := range x.Members {
				target, ok := byID[id]
				if !ok || target.Kind != "app" || seen[id] || members[id] {
					return false
				}
				seen[id] = true
				members[id] = true
			}
		case "spacer", "separator":
			if x.Label != "" || x.ReferenceID != "" || x.URL != "" || x.IconID != "" || !emptyMembers || x.FolderView != "" {
				return false
			}
		}
	}
	return true
}

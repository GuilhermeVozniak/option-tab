// Package update checks GitHub releases for a newer version. Only the pure
// parsing and version-comparison logic lives here (unit-tested); the wiring
// layer (app.go) performs the HTTP fetch and surfaces the result to the UI.
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Release is the subset of a GitHub release the checker needs.
type Release struct {
	Version string  `json:"tag_name"`
	URL     string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// AssetFor prefers one exact architecture asset, then one macOS universal asset.
// Ambiguous candidates refuse selection; filenames must match this release tag.
func (r Release) AssetFor(platformArch string) string {
	extensions := map[string]string{"darwin_arm64": "dmg", "darwin_amd64": "dmg", "darwin_universal": "dmg", "windows_amd64": "zip", "windows_arm64": "zip", "linux_amd64": "tar.gz", "linux_arm64": "tar.gz"}
	ext, ok := extensions[platformArch]
	version := strings.TrimPrefix(r.Version, "v")
	if !ok || version == "" || strings.ContainsAny(version, "/\\ \t\n") {
		return ""
	}
	find := func(arch string) (string, int) {
		name := "option-tab_" + version + "_" + arch + "." + ext
		url, count := "", 0
		for _, asset := range r.Assets {
			if asset.Name == name {
				url = asset.DownloadURL
				count++
			}
		}
		return url, count
	}
	if url, count := find(platformArch); count > 0 {
		if count == 1 {
			return url
		}
		return ""
	}
	if platformArch == "darwin_arm64" || platformArch == "darwin_amd64" {
		if url, count := find("darwin_universal"); count == 1 {
			return url
		}
	}
	return ""
}

// ParseLatest decodes a GitHub "latest release" API response.
func ParseLatest(body []byte) (Release, error) {
	var r Release
	if err := json.Unmarshal(body, &r); err != nil {
		return Release{}, fmt.Errorf("update: parse: %w", err)
	}
	if r.Version == "" {
		return Release{}, errors.New("update: response has no tag_name")
	}
	return r, nil
}

// Newer reports whether latest is a strictly newer semantic version than
// current. Tags may carry a leading "v" and a prerelease/build suffix.
// Malformed versions compare as not newer, so a bad response never prompts.
func Newer(current, latest string) bool {
	c, okC := parse(current)
	l, okL := parse(latest)
	if !okC || !okL {
		return false
	}
	for i := range 3 {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 { // strip prerelease/build metadata
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if v == "" || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

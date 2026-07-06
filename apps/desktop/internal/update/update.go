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

// AssetFor returns the download URL of the asset matching the platform/arch
// suffix (e.g. "darwin_arm64"), or "" when the release carries none.
func (r Release) AssetFor(platformArch string) string {
	for _, a := range r.Assets {
		if strings.Contains(a.Name, platformArch) {
			return a.DownloadURL
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

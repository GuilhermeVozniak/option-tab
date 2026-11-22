//go:build !darwin

package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestUnsupportedFolderListingMarshalsEmptyEntries(t *testing.T) {
	listing, err := NewFolderSource("").ListFolder(context.Background(), FolderRef{}, FolderSort{})
	if err == nil || listing.Status != "unavailable" {
		t.Fatalf("unsupported listing: %+v %v", listing, err)
	}
	// The frontend iterates entries; the darwin source never sends null.
	raw, err := json.Marshal(listing)
	if err != nil || !strings.Contains(string(raw), `"entries":[]`) {
		t.Fatalf("entries must marshal as an empty array: %s %v", raw, err)
	}
}

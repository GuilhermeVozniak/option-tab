package platform

import (
	"context"
	"errors"
	"testing"
)

func TestJSONExportValidation(t *testing.T) {
	for _, name := range []string{"option-tab-settings.json", "option-tab-launcher-profile.json"} {
		if err := validateJSONExport(context.Background(), name, []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"../private.json", []byte("{}")},
		{"option-tab-diagnostics.json", []byte("{}")},
		{"option-tab-settings.json", nil},
		{"option-tab-settings.json", []byte("invalid")},
		{"option-tab-settings.json", make([]byte, MaxDiagnosticExportBytes+1)},
	} {
		if err := validateJSONExport(context.Background(), tc.name, tc.data); err == nil {
			t.Fatal("invalid JSON export admitted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateJSONExport(ctx, "option-tab-settings.json", []byte("{}")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

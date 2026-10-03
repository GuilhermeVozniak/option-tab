package platform

import (
	"context"
	"errors"
	"testing"
)

func TestDiagnosticExportValidationBoundsAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{{"../private.json", []byte("{}")}, {"option-tab-diagnostics.json", nil}, {"option-tab-diagnostics.json", make([]byte, MaxDiagnosticExportBytes+1)}} {
		if err := validateDiagnosticExport(context.Background(), tc.name, tc.data); err == nil {
			t.Fatal("invalid destination/data admitted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateDiagnosticExport(ctx, "option-tab-diagnostics.json", []byte("{}")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

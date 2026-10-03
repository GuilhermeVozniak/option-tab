//go:build !darwin

package platform

import (
	"context"
	"errors"
	"testing"
)

func TestDiagnosticExportStubRefuses(t *testing.T) {
	result, err := NewDiagnosticExportSource().SaveDiagnosticReport(context.Background(), "option-tab-diagnostics.json", []byte("{}"))
	var failure *DiagnosticExportError
	if !errors.As(err, &failure) || failure.Code != "unsupported" || result.Status == "saved" {
		t.Fatalf("false portable save %+v %v", result, err)
	}
}

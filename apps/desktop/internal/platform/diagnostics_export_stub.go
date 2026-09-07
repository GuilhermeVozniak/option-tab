//go:build !darwin

package platform

import "context"

type diagnosticExportStub struct{}

func NewDiagnosticExportSource() DiagnosticExportSource { return diagnosticExportStub{} }
func (diagnosticExportStub) SaveDiagnosticReport(ctx context.Context, name string, data []byte) (DiagnosticExportResult, error) {
	if err := validateDiagnosticExport(ctx, name, data); err != nil {
		return DiagnosticExportResult{}, err
	}
	return DiagnosticExportResult{}, &DiagnosticExportError{Code: "unsupported"}
}

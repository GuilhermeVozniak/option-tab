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

func NewJSONExportSource() JSONExportSource { return diagnosticExportStub{} }
func (diagnosticExportStub) SaveJSONExport(ctx context.Context, name string, data []byte) (DiagnosticExportResult, error) {
	if err := validateJSONExport(ctx, name, data); err != nil {
		return DiagnosticExportResult{}, err
	}
	return DiagnosticExportResult{}, &DiagnosticExportError{Code: "unsupported"}
}

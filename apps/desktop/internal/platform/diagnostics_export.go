package platform

import "context"

const MaxDiagnosticExportBytes = 512 * 1024

// DiagnosticExportSource saves already reviewed, bounded bytes only. It never
// collects logs, sends data, or accepts a frontend-selected filesystem path.
// Cancellation joins its chooser/write owner before returning; call off AppKit.
type DiagnosticExportSource interface {
	SaveDiagnosticReport(context.Context, string, []byte) (DiagnosticExportResult, error)
}
type DiagnosticExportResult struct {
	Status string `json:"status"`
}

// Codes contain no selected path or native error description.
type DiagnosticExportError struct{ Code string }

func (e *DiagnosticExportError) Error() string                     { return "diagnostics export: " + e.Code }
func (e *DiagnosticExportError) DiagnosticExportErrorCode() string { return e.Code }
func validateDiagnosticExport(ctx context.Context, name string, data []byte) error {
	if ctx == nil || name != "option-tab-diagnostics.json" || len(data) == 0 || len(data) > MaxDiagnosticExportBytes {
		return &DiagnosticExportError{Code: "invalidArgument"}
	}
	return ctx.Err()
}

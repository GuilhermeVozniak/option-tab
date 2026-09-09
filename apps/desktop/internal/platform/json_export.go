package platform

import (
	"context"
	"encoding/json"
)

// JSONExportSource presents a native chooser for backend-owned JSON. Suggested
// names are fixed; no renderer-selected path or arbitrary filename is accepted.
// It shares the diagnostics writer's bounded, exclusive atomic write and joins
// chooser/write cancellation before returning. Call off AppKit.
type JSONExportSource interface {
	SaveJSONExport(context.Context, string, []byte) (DiagnosticExportResult, error)
}

func validateJSONExport(ctx context.Context, name string, data []byte) error {
	if ctx == nil || (name != "option-tab-settings.json" && name != "option-tab-launcher-profile.json") || len(data) == 0 || len(data) > MaxDiagnosticExportBytes || !json.Valid(data) {
		return &DiagnosticExportError{Code: "invalidArgument"}
	}
	return ctx.Err()
}

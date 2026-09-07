//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "darwin_diagnostics_export.h"
*/
import "C"

import (
	"context"
	"runtime/cgo"
	"time"
)

type diagnosticExportSource struct{ slot chan struct{} }

func NewDiagnosticExportSource() DiagnosticExportSource {
	return &diagnosticExportSource{slot: make(chan struct{}, 1)}
}

func (s *diagnosticExportSource) SaveDiagnosticReport(ctx context.Context, name string, data []byte) (DiagnosticExportResult, error) {
	if err := validateDiagnosticExport(ctx, name, data); err != nil {
		return DiagnosticExportResult{}, err
	}
	if C.ot_diagnostics_main_thread() != 0 {
		return DiagnosticExportResult{}, &DiagnosticExportError{Code: "unavailable"}
	}
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	default:
		return DiagnosticExportResult{}, &DiagnosticExportError{Code: "busy"}
	}
	admissionCtx, admissionCancel := context.WithCancel(ctx)
	defer admissionCancel()
	admission := cgo.NewHandle(admissionCtx)
	defer admission.Delete()
	bytes := C.CBytes(data)
	handle := C.ot_diagnostics_save_start(bytes, C.size_t(len(data)), C.uintptr_t(admission))
	C.free(bytes)
	if handle == nil {
		return DiagnosticExportResult{}, &DiagnosticExportError{Code: "unavailable"}
	}
	defer C.ot_diagnostics_save_release(handle)
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	cancelled := false
	for {
		if ctx.Err() != nil && !cancelled {
			cancelled = true
			C.ot_diagnostics_save_cancel(handle)
		}
		status := int(C.ot_diagnostics_save_poll(handle))
		if status != 0 {
			switch status {
			case 1:
				return DiagnosticExportResult{Status: "saved"}, nil
			case 2:
				if ctx.Err() != nil {
					return DiagnosticExportResult{Status: "cancelled"}, ctx.Err()
				}
				return DiagnosticExportResult{Status: "cancelled"}, nil
			case 3:
				return DiagnosticExportResult{}, &DiagnosticExportError{Code: "destinationExists"}
			default:
				return DiagnosticExportResult{}, &DiagnosticExportError{Code: "ioFailure"}
			}
		}
		<-timer.C
	}
}

//export goDiagnosticContextCurrent
func goDiagnosticContextCurrent(token C.uintptr_t) C.int {
	if token == 0 {
		return 0
	}
	ctx, ok := cgo.Handle(token).Value().(context.Context)
	if ok && ctx.Err() == nil {
		return 1
	}
	return 0
}

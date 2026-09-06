package platform

import (
	"context"

	"option-tab/internal/domain"
)

// WindowStreamSource is an optional continuous capture capability. It blocks
// until cancellation or a native error; callers must invoke it off event paths.
// Frame callbacks must return promptly. Errors permit honest snapshot fallback.
type WindowStreamSource interface {
	StreamWindow(context.Context, domain.WindowID, int, func(string)) error
}

// WindowUnavailableError reports an ended window identity; snapshot retry is unsafe.
type WindowUnavailableError struct{}

func (WindowUnavailableError) Error() string           { return "captured window closed or changed owner" }
func (WindowUnavailableError) WindowUnavailable() bool { return true }

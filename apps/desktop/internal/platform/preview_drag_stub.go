//go:build !darwin

package platform

import (
	"context"
	"errors"
)

func (s *stub) DragPreview(context.Context, PreviewDragRequest) (PreviewDragResult, error) {
	return PreviewDragResult{}, errors.New("native preview drag unsupported on this platform")
}

package platform

import (
	"context"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type (
	WidgetPackageFile struct {
		Name    string
		Archive []byte
	}
	WidgetPackageSource interface {
		ChooseWidgetPackage(context.Context) (WidgetPackageFile, error)
	}
	WidgetPackageError struct{ Code string }
)

func (e *WidgetPackageError) Error() string { return "widget package: " + e.Code }

type widgetPackageSource struct {
	slot   chan struct{}
	choose func(context.Context) (WidgetPackageFile, error)
}

func newWidgetPackageSource(choose func(context.Context) (WidgetPackageFile, error)) *widgetPackageSource {
	return &widgetPackageSource{slot: make(chan struct{}, 1), choose: choose}
}

func (s *widgetPackageSource) ChooseWidgetPackage(ctx context.Context) (WidgetPackageFile, error) {
	if ctx == nil {
		return WidgetPackageFile{}, &WidgetPackageError{Code: "unavailable"}
	}
	if err := ctx.Err(); err != nil {
		return WidgetPackageFile{}, err
	}
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	default:
		return WidgetPackageFile{}, &WidgetPackageError{Code: "busy"}
	}
	if s.choose == nil {
		return WidgetPackageFile{}, &WidgetPackageError{Code: "unavailable"}
	}
	selected, err := s.choose(ctx)
	if ctx.Err() != nil {
		return WidgetPackageFile{}, ctx.Err()
	}
	if err != nil {
		return WidgetPackageFile{}, err
	}
	if len(selected.Archive) > 4*1024*1024 {
		return WidgetPackageFile{}, &WidgetPackageError{Code: "tooLarge"}
	}
	name := selected.Name
	ext := strings.ToLower(filepath.Ext(name))
	if name == "" || len(name) > 1024 || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || filepath.Base(name) != name || (ext != ".zip" && ext != ".otwidget") {
		return WidgetPackageFile{}, &WidgetPackageError{Code: "invalidFile"}
	}
	selected.Archive = append([]byte(nil), selected.Archive...)
	if err := ctx.Err(); err != nil {
		return WidgetPackageFile{}, err
	}
	return selected, nil
}

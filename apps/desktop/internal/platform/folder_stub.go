//go:build !darwin

package platform

import (
	"context"
	"errors"
)

type unsupportedFolderSource struct{}

func NewFolderSource(string) FolderSource { return unsupportedFolderSource{} }
func (unsupportedFolderSource) ListFolder(context.Context, FolderRef, FolderSort) (FolderListing, error) {
	return FolderListing{Status: "unavailable"}, errors.New("folder access is unsupported on this platform")
}

func (unsupportedFolderSource) RequestFolderAccess(context.Context, FolderRef) (FolderGrant, error) {
	return FolderGrant{Status: "unavailable"}, errors.New("folder access is unsupported on this platform")
}

func (unsupportedFolderSource) OpenFolderEntryGuarded(context.Context, FolderRef, string, func() error) error {
	return errors.New("folder access is unsupported on this platform")
}

package platform

import "context"

// LauncherReference describes a private user-selected resource. Persistent
// bookmark bytes and resolved filesystem paths never cross the renderer bridge.
type LauncherReference struct {
	ID, Kind, Label, BundleID, State, Reason string
	Revision                                 uint64
	Process                                  ProcessIdentity // Exact selected app's unique running instance, if known.
}

type LauncherItemAction struct {
	ReferenceID           string
	ReferenceRevision     uint64
	Kind                  string // open|relaunch
	Process               ProcessIdentity
	BundleID, DisplayUUID string
	PanelToken            uint64
}

type LauncherReferenceSource interface {
	ChooseLauncherReference(context.Context, string) (LauncherReference, error)
	ResolveLauncherReference(context.Context, string) (LauncherReference, error)
	RelinkLauncherReference(context.Context, string) (LauncherReference, error)
	RemoveLauncherReference(context.Context, string) error // Private record only.
	PerformLauncherItem(context.Context, LauncherItemAction, func() error) error
	WithLauncherFolder(context.Context, string, uint64, func(FolderRef) error) error
	OpenLauncherLink(context.Context, string, string, uint64, func() error) error
	Close() error
}

type LauncherIconSource interface {
	ChooseLauncherIcon(context.Context) (string, error)
	LauncherIcon(context.Context, string) ([]byte, error)
	RemoveLauncherIcon(context.Context, string) error // Private normalized asset only.
	Close() error
}

package platform

// LauncherPanelStyle is a closed set of host styling choices. It carries no
// resource paths, renderer CSS or native window handle.
type LauncherPanelStyle struct {
	Material       string
	Theme          string
	CornerRadiusPx int
}

// LauncherPanelStyler applies only to a dedicated launcher host. Callers apply
// the immutable profile style before showing the host, outside owner locks.
type LauncherPanelStyler interface {
	SetLauncherStyle(LauncherPanelStyle) error
}

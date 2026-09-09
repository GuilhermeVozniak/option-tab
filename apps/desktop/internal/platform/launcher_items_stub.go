//go:build !darwin

package platform

func NewLauncherReferenceSource(string) (LauncherReferenceSource, error) {
	return nil, launcherItemError("unsupported")
}

func NewLauncherIconSource(string) (LauncherIconSource, error) {
	return nil, launcherItemError("unsupported")
}

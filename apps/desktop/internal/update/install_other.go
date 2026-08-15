//go:build !darwin

package update

import "errors"

// errUnsupported marks self-install as unimplemented off macOS: the Windows
// and Linux builds are stub-platform demo builds with no real bundle layout.
var errUnsupported = errors.New("update: self-install is only supported on macOS")

func InstallDMG(dmgPath string) (string, error) { return "", errUnsupported }

func RelaunchSelf(appPath string, pid int) error { return errUnsupported }

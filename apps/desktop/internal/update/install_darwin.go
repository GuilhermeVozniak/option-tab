//go:build darwin

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// InstallDMG mounts the release dmg, copies the contained .app over the
// running app's bundle, and unmounts. It returns the installed bundle path.
// The copy is staged next to the target so the final swap is rename-only.
//
// The dmg was downloaded over plain HTTP, so neither it nor the copied bundle
// carries a quarantine xattr — the relaunched app hits no new Gatekeeper
// prompt. (The app is unsigned; there is no signature to verify or preserve.)
func InstallDMG(dmgPath string) (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	target, err := AppBundleFromExecutable(execPath)
	if err != nil {
		return "", err
	}

	tmp, err := os.MkdirTemp("", "option-tab-update")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	mnt := filepath.Join(tmp, "mnt")
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mnt, dmgPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("update: mount dmg: %w (%s)", err, out)
	}
	defer func() { _ = exec.Command("hdiutil", "detach", "-quiet", mnt).Run() }()

	apps, err := filepath.Glob(filepath.Join(mnt, "*.app"))
	if err != nil || len(apps) == 0 {
		return "", fmt.Errorf("update: no .app found in %s", dmgPath)
	}

	// Stage on the target's volume (its directory) so SwapApp is rename-only.
	staging := filepath.Join(filepath.Dir(target), ".option-tab-new")
	_ = os.RemoveAll(staging)
	if out, err := exec.Command("ditto", apps[0], staging).CombinedOutput(); err != nil {
		return "", fmt.Errorf("update: copy new app: %w (%s)", err, out)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	if err := SwapApp(staging, target); err != nil {
		return "", err
	}
	return target, nil
}

// RelaunchSelf starts a detached helper that reopens the app once this
// process has exited. The caller should quit immediately after.
func RelaunchSelf(appPath string, pid int) error {
	cmd := exec.Command("/bin/sh", "-c", relaunchScript(pid, appPath))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

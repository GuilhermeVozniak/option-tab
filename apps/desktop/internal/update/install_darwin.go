//go:build darwin

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// expectedTeamID is the Apple Developer Team ID every release is signed
// with. verifyBundle refuses to install a bundle signed by anyone else, so a
// substituted or tampered download cannot gain code execution.
const expectedTeamID = "CT22R575UG"

// InstallDMG mounts the release dmg, verifies the contained .app (signature
// intact, signed by our Team ID, accepted by Gatekeeper), copies it over the
// running app's bundle, and unmounts. It returns the installed bundle path.
// The copy is staged next to the target so the final swap is rename-only.
//
// The dmg is downloaded by the app itself (not a browser), so neither it nor
// the copied bundle carries a quarantine xattr — the relaunched app hits no
// new Gatekeeper prompt.
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

	if err := verifyBundle(apps[0]); err != nil {
		return "", err
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

// verifyBundle refuses anything that is not a well-formed bundle signed by
// our team and accepted by Gatekeeper. This is the security boundary of the
// updater: past this point the bundle is copied over the installed app and
// executed.
func verifyBundle(app string) error {
	// 1. Signature intact and unmodified since signing.
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("update: signature verification failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// 2. Signed by us, not merely by somebody.
	out, err := exec.Command("codesign", "-dv", "--verbose=4", app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("update: read signature: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	team, err := parseTeamID(out)
	if err != nil {
		return err
	}
	if team != expectedTeamID {
		return fmt.Errorf("update: bundle signed by team %s, expected %s", team, expectedTeamID)
	}

	// 3. Notarized and accepted by Gatekeeper: catches a validly-signed but
	// unreleased build, and matches what the user's Mac would enforce.
	if out, err := exec.Command("spctl", "--assess", "--type", "exec", app).CombinedOutput(); err != nil {
		return fmt.Errorf("update: gatekeeper rejected the update: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RelaunchSelf starts a detached helper that reopens the app once this
// process has exited. The caller should quit immediately after.
func RelaunchSelf(appPath string, pid int) error {
	cmd := exec.Command("/bin/sh", "-c", relaunchScript(pid, appPath))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

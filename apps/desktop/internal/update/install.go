package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppBundleFromExecutable resolves the .app bundle containing the running
// executable (…/X.app/Contents/MacOS/x). It errors when the app is not
// running from a bundle (e.g. `go run` or a bare `go build` output in dev),
// in which case self-install is not possible.
func AppBundleFromExecutable(execPath string) (string, error) {
	dir := filepath.Dir(execPath)
	for {
		if strings.HasSuffix(filepath.Base(dir), ".app") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("update: executable is not inside a .app bundle")
		}
		dir = parent
	}
}

// SwapApp replaces targetApp with newApp: the target is renamed aside, the
// new bundle renamed into place, and the backup removed. Both paths must be
// on the same volume (InstallDMG stages the copy next to the target for this
// reason). A failure mid-swap rolls back so the old install keeps working.
func SwapApp(newApp, targetApp string) error {
	backup := targetApp + ".old"
	_ = os.RemoveAll(backup) // stale backup from a crashed earlier update
	if err := os.Rename(targetApp, backup); err != nil {
		return fmt.Errorf("update: move current app aside: %w", err)
	}
	if err := os.Rename(newApp, targetApp); err != nil {
		_ = os.Rename(backup, targetApp) // roll back
		return fmt.Errorf("update: move new app into place: %w", err)
	}
	return os.RemoveAll(backup)
}

// parseTeamID extracts the TeamIdentifier from `codesign -dv` output. It
// errors when the bundle carries none (unsigned, or ad-hoc: "not set").
func parseTeamID(out []byte) (string, error) {
	for _, line := range strings.Split(string(out), "\n") {
		team, ok := strings.CutPrefix(strings.TrimSpace(line), "TeamIdentifier=")
		if !ok {
			continue
		}
		if team == "" || team == "not set" {
			break
		}
		return team, nil
	}
	return "", errors.New("update: bundle has no Team Identifier")
}

// relaunchScript waits for pid to exit, then opens the (replaced) app bundle.
// The updater runs it detached right before quitting: a plain `open` while we
// are still alive would be swallowed by the single-instance guard.
func relaunchScript(pid int, appPath string) string {
	return fmt.Sprintf("while kill -0 %d 2>/dev/null; do sleep 0.2; done; open -n %q", pid, appPath)
}

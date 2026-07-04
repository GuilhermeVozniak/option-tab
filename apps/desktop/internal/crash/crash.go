// Package crash captures fatal runtime crashes to a local log file and manages
// the "pending crash report" lifecycle across launches. Nothing is ever
// transmitted automatically: reporting opens a prefilled GitHub issue in the
// user's browser, honoring the crash-reports policy in config.Behavior.
package crash

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

const (
	currentName = "crash-current.log"
	pendingName = "crash-pending.log"
)

// Rotate promotes a non-empty crash log left behind by the previous run to
// pending, where it waits for the user to report or dismiss it. An empty or
// missing current log (the normal case: clean exit) is a no-op.
func Rotate(dir string) error {
	cur := filepath.Join(dir, currentName)
	info, err := os.Stat(cur)
	if err != nil || info.Size() == 0 {
		return nil
	}
	return os.Rename(cur, filepath.Join(dir, pendingName))
}

// Arm creates/truncates the current crash file and routes fatal runtime
// crashes (unrecovered panics on any goroutine) to it via debug.SetCrashOutput.
func Arm(dir string) error {
	f, err := os.Create(filepath.Join(dir, currentName))
	if err != nil {
		return err
	}
	return debug.SetCrashOutput(f, debug.CrashOptions{})
}

// Pending returns the pending crash log's content, or "" when there is none.
func Pending(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, pendingName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Dismiss deletes any pending crash log. A missing file is not an error.
func Dismiss(dir string) error {
	err := os.Remove(filepath.Join(dir, pendingName))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

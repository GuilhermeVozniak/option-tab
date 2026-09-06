package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestAppBundleFromExecutable(t *testing.T) {
	tests := []struct {
		name string
		exec string
		want string
	}{
		{"standard install", "/Applications/Option Tab.app/Contents/MacOS/option-tab", "/Applications/Option Tab.app"},
		{"user applications", "/home/u/Applications/Option Tab.app/Contents/MacOS/option-tab", "/home/u/Applications/Option Tab.app"},
		{"directly in bundle root", "/tmp/X.app/x", "/tmp/X.app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AppBundleFromExecutable(tt.exec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppBundleFromExecutableNotInBundle(t *testing.T) {
	for _, exec := range []string{
		"/usr/local/bin/option-tab",
		"option-tab",
		"/tmp/go-build1234/exe/main",
	} {
		if got, err := AppBundleFromExecutable(exec); err == nil {
			t.Fatalf("%s: expected error, got %q", exec, got)
		}
	}
}

func TestSwapApp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Option Tab.app")
	staged := filepath.Join(dir, ".option-tab-new")
	writeFile(t, filepath.Join(target, "Contents", "MacOS", "option-tab"), "old-binary")
	writeFile(t, filepath.Join(staged, "Contents", "MacOS", "option-tab"), "new-binary")

	if err := SwapApp(staged, target); err != nil {
		t.Fatalf("SwapApp: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "Contents", "MacOS", "option-tab"))
	if err != nil || string(data) != "new-binary" {
		t.Fatalf("target holds %q, err=%v; want new-binary", data, err)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed, stat err=%v", err)
	}
}

func TestSwapAppRollsBackWhenNewAppMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Option Tab.app")
	writeFile(t, filepath.Join(target, "Contents", "MacOS", "option-tab"), "old-binary")

	err := SwapApp(filepath.Join(dir, "does-not-exist"), target)
	if err == nil {
		t.Fatal("expected an error when the staged bundle is missing")
	}
	data, readErr := os.ReadFile(filepath.Join(target, "Contents", "MacOS", "option-tab"))
	if readErr != nil || string(data) != "old-binary" {
		t.Fatalf("rollback failed: target holds %q, err=%v", data, readErr)
	}
}

func TestParseTeamID(t *testing.T) {
	out := "Executable=/Applications/Option Tab.app/Contents/MacOS/option-tab\n" +
		"Identifier=com.optiontab.app\n" +
		"TeamIdentifier=CT22R575UG\n" +
		"Sealed Resources version=2 rules=13 files=3\n"
	got, err := parseTeamID([]byte(out))
	if err != nil {
		t.Fatalf("parseTeamID: %v", err)
	}
	if got != "CT22R575UG" {
		t.Fatalf("got %q, want CT22R575UG", got)
	}
}

func TestParseTeamIDRejectsUnsignedOrMissing(t *testing.T) {
	for name, out := range map[string]string{
		"ad-hoc":   "Identifier=com.optiontab.app\nTeamIdentifier=not set\n",
		"absent":   "Identifier=com.optiontab.app\n",
		"empty":    "",
		"blank-id": "TeamIdentifier=\n",
	} {
		if got, err := parseTeamID([]byte(out)); err == nil {
			t.Errorf("%s: expected error, got team %q", name, got)
		}
	}
}

func TestRelaunchScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relaunch script requires a POSIX shell")
	}

	tmp := t.TempDir()
	marker := filepath.Join(tmp, "expanded")
	argsFile := filepath.Join(tmp, "open-args")
	fakeOpen := filepath.Join(tmp, "open")
	writeFile(t, fakeOpen, "#!/bin/sh\nif kill -0 \"$EXPECTED_PID\" 2>/dev/null; then exit 42; fi\nprintf '%s\\n' \"$@\" > \"$OPEN_ARGS\"\n")
	if err := os.Chmod(fakeOpen, 0o755); err != nil {
		t.Fatal(err)
	}

	appPath := filepath.Join(tmp, "Option Tab $(touch "+marker+") `quoted` \"double\" 'bundle'.app\nline")
	waitFor := exec.Command("/bin/sleep", "0.05")
	err := waitFor.Start()
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = waitFor.Wait() }()

	cmd := exec.Command("/bin/sh", "-c", relaunchScript(), "updater", strconv.Itoa(waitFor.Process.Pid), appPath)
	cmd.Env = append(os.Environ(), "EXPECTED_PID="+strconv.Itoa(waitFor.Process.Pid), "OPEN_ARGS="+argsFile, "PATH="+tmp+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("relaunch script failed: %v (%s)", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("app path was interpreted by the shell; marker stat error: %v", err)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "-n\n" + appPath + "\n"
	if string(args) != want {
		t.Fatalf("open received %q, want %q", args, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

#!/usr/bin/env python3
"""Build and test an exact-PID disposable Cocoa fixture; never target user apps."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--log", type=Path, help="Optional path for the smoke test output")
args = parser.parse_args()
if sys.platform != "darwin":
    parser.error("requires macOS, Go, clang, and Accessibility access for the launching terminal")
sources = Path(__file__).resolve().parent
desktop = sources.parents[2]
with tempfile.TemporaryDirectory(prefix="option-tab-native-actions-") as directory:
    root = Path(directory)
    bundle = root / "ActionSmoke.app/Contents"
    (bundle / "MacOS").mkdir(parents=True)
    shutil.copyfile(sources / "native_smoke_Info.plist", bundle / "Info.plist")
    executable = bundle / "MacOS/ActionSmoke"
    subprocess.run(["clang", "-fobjc-arc", "-framework", "Cocoa", str(sources / "native_smoke_fixture.m"), "-o", str(executable)], check=True)
    binary = root / "actions.test"
    subprocess.run(["go", "test", "-c", "./internal/actions", "-o", str(binary)], cwd=desktop, check=True)
    state = root / "state.json"
    environment = os.environ.copy()
    environment["OPTION_TAB_ACTION_FIXTURE_STATE"] = str(state)
    with (root / "fixture.log").open("w") as fixture_log:
        fixture = subprocess.Popen([str(executable)], env=environment, stdout=fixture_log, stderr=fixture_log)
        try:
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                if state.exists() and len(json.loads(state.read_text())["windows"]) == 2:
                    break
                if fixture.poll() is not None:
                    raise RuntimeError("disposable fixture exited before becoming ready")
                time.sleep(0.1)
            else:
                raise RuntimeError("disposable fixture did not become ready")
            # Reap termination during the Go test so kill(pid, 0) cannot see a zombie.
            reaper = threading.Thread(target=fixture.wait, daemon=True)
            reaper.start()
            environment["OPTION_TAB_ACTION_FIXTURE_PID"] = str(fixture.pid)
            print(f"Disposable fixture PID: {fixture.pid}", flush=True)
            result = subprocess.run([str(binary), "-test.run", "^TestDisposableNativeActionSmoke$", "-test.v", "-test.count=1"], env=environment, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60)
            output = f"Disposable fixture PID: {fixture.pid}\n{result.stdout}"
            print(result.stdout, end="")
            if args.log:
                args.log.write_text(output)
        finally:
            if fixture.poll() is None:
                fixture.terminate()
            try:
                fixture.wait(timeout=5)
            except subprocess.TimeoutExpired:
                fixture.kill()
                fixture.wait()
    sys.exit(result.returncode)

#!/usr/bin/env python3
"""Explicit opt-in, own disposable LSUIElement fixture; never prompts or activates."""
import os
from pathlib import Path
import plistlib
import signal
import subprocess
import tempfile
import time
import uuid
import xml.etree.ElementTree as ET

HERE = Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix="option-tab-automation-") as directory:
    root = Path(directory)
    app = root / "AutomationTransportFixture.app"
    macos = app / "Contents" / "MacOS"
    resources = app / "Contents" / "Resources"
    macos.mkdir(parents=True)
    resources.mkdir()
    binary = macos / "automation-fixture"
    bundle = "com.optiontab.fixture.automation." + uuid.uuid4().hex
    info = {"CFBundleExecutable": binary.name, "CFBundleIdentifier": bundle,
            "CFBundleName": "Option Tab Disposable Automation Fixture",
            "CFBundlePackageType": "APPL", "LSUIElement": True,
            "NSAppleScriptEnabled": True, "OSAScriptingDefinition": "OptionTab.sdef",
            "NSAppleEventsUsageDescription": "This disposable fixture tests only its own transport."}
    (app / "Contents" / "Info.plist").write_bytes(plistlib.dumps(info))
    dictionary = HERE.parents[3] / "build" / "darwin" / "OptionTab.sdef"
    (resources / "OptionTab.sdef").write_bytes(dictionary.read_bytes())
    subprocess.run(["clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa",
                    "-framework", "Carbon", str(HERE / "crossprocess.m"), "-o", str(binary)], check=True)
    subprocess.run(["codesign", "--force", "--sign", "-", str(app)], check=True)
    subprocess.run(["codesign", "--verify", "--strict", str(app)], check=True)
    discovered = subprocess.check_output(["sdef", str(app)], timeout=5)
    codes = {command.attrib["code"] for command in ET.fromstring(discovered).iter("command")}
    expected = {"OpTbswop", "OpTbpvsh", "OpTbpvhi", "OpTbwact", "OpTbqapp", "OpTbqwin", "OpTbqact"}
    if codes != expected:
        raise RuntimeError("Packaged sdef command discovery mismatch")
    print("PASS packaged sdef discovery: seven exact private commands", flush=True)
    ready = root / "receiver.pid"
    pid = None
    try:
        subprocess.run(["open", "-g", "-j", "-n", str(app), "--args", "--receiver", str(ready)], check=True, timeout=5)
        end = time.monotonic() + 5
        while not ready.exists() and time.monotonic() < end:
            time.sleep(.01)
        if not ready.exists():
            raise RuntimeError("Disposable receiver did not announce PID")
        pid = int(ready.read_text())
        path = subprocess.check_output(["ps", "-p", str(pid), "-o", "comm="], text=True).strip()
        if Path(path).resolve() != binary.resolve():
            raise RuntimeError("Announced PID has different executable; refusing send/cleanup")
        print(f"PASS packaged native registration status=1 receiverPID={pid}", flush=True)
        subprocess.run([str(binary), "--send", str(pid), str(app)], check=True, timeout=5)
    finally:
        if pid:
            result = subprocess.run(["ps", "-p", str(pid), "-o", "comm="], text=True, capture_output=True)
            if result.returncode == 0 and Path(result.stdout.strip()).resolve() == binary.resolve():
                os.kill(pid, signal.SIGTERM)
                for _ in range(100):
                    if subprocess.run(["kill", "-0", str(pid)], capture_output=True).returncode != 0:
                        break
                    time.sleep(.01)
                else:
                    raise RuntimeError("Exact fixture PID did not terminate; retaining temp resources is required")

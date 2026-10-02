#!/usr/bin/env python3
"""Explicit, isolated native capture stress against committed product sources."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import selectors
import shutil
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-native", action="store_true", help="create owned test windows")
    args = parser.parse_args()
    if not args.run_native:
        parser.error("--run-native is required; this test creates four disposable windows")

    source = Path(__file__).resolve().parent
    repo = source.parents[5]
    work = Path(tempfile.mkdtemp(prefix="option-tab-capture-stress-"))
    print("EVIDENCE", work, flush=True)
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()
    with (work / "build.log").open("w") as log:
        subprocess.run(
            ["git", "clone", "--shared", "--no-checkout", str(repo), str(work / "source")],
            check=True, stdout=log, stderr=subprocess.STDOUT,
        )
        subprocess.run(
            ["git", "checkout", "--detach", head], cwd=work / "source",
            check=True, stdout=log, stderr=subprocess.STDOUT,
        )
        desktop = work / "source/apps/desktop"
        probe_source = desktop / "testdata/capture_stress"
        probe_source.mkdir()
        shutil.copy2(source / "main.go", probe_source / "main.go")
        subprocess.run(
            ["go", "build", "-o", str(work / "capture-probe"), "./testdata/capture_stress"],
            cwd=desktop, check=True, stdout=log, stderr=subprocess.STDOUT,
        )
        subprocess.run(
            ["clang", "-fobjc-arc", "-framework", "Cocoa", str(source / "fixture.m"),
             "-o", str(work / "fixture")],
            check=True, stdout=log, stderr=subprocess.STDOUT,
        )

    processes = []
    samples = []
    result = {
        "sourceCommit": head,
        "binarySha256": hashlib.sha256((work / "capture-probe").read_bytes()).hexdigest(),
        "probeSourceSha256": hashlib.sha256((source / "main.go").read_bytes()).hexdigest(),
        "os": subprocess.check_output(["sw_vers"], text=True).strip(),
        "arch": subprocess.check_output(["uname", "-m"], text=True).strip(),
    }
    try:
        with (work / "fixture.log").open("w") as fixture_log:
            fixture = subprocess.Popen(
                [str(work / "fixture")], stdout=subprocess.PIPE,
                stderr=fixture_log, text=True,
            )
            processes.append(fixture)
            with selectors.DefaultSelector() as ready:
                ready.register(fixture.stdout, selectors.EVENT_READ)
                if not ready.select(5):
                    raise RuntimeError("fixture startup timeout")
            ids = fixture.stdout.readline().strip()
            if len(ids.split(",")) != 4 or not all(x.isdigit() for x in ids.split(",")):
                raise RuntimeError("invalid owned window IDs")
            env = os.environ.copy()
            env.update(CAPTURE_WINDOW_IDS=ids, CAPTURE_FIXTURE_PID=str(fixture.pid))
            with (work / "probe.jsonl").open("w") as output, \
                    (work / "probe.stderr.log").open("w") as stderr:
                probe = subprocess.Popen(
                    [str(work / "capture-probe")], env=env, stdout=output, stderr=stderr,
                )
                processes.append(probe)
                start = time.monotonic()
                while probe.poll() is None:
                    if time.monotonic() - start > 180:
                        raise RuntimeError("native capture probe timeout")
                    sample = subprocess.run(
                        ["ps", "-o", "rss=", "-p", str(probe.pid)],
                        capture_output=True, text=True,
                    )
                    if sample.returncode == 0 and sample.stdout.strip():
                        samples.append({
                            "seconds": time.monotonic() - start,
                            "unixTime": time.time(),
                            "rssBytes": int(sample.stdout.strip()) * 1024,
                        })
                    time.sleep(0.25)
                result["exitCode"] = probe.returncode
                if probe.returncode:
                    raise RuntimeError("native capture probe failed; inspect probe.jsonl")
    except Exception as error:
        result["error"] = str(error)
        raise
    finally:
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
        for process in processes:
            try:
                process.wait(timeout=4)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        result["samples"] = samples
        result["ownedProcessesStopped"] = all(p.poll() is not None for p in processes)
        (work / "run.json").write_text(json.dumps(result, indent=2) + "\n")
        print("CLEANUP owned processes stopped; no user window actions sent", flush=True)
    print("PASS capture lifetime assertions; RSS/CPU are measurements", flush=True)


if __name__ == "__main__":
    main()

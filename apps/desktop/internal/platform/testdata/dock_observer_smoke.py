#!/usr/bin/env python3
"""Hover only a disposable app's native Dock icon; never change Dock preferences."""
import argparse
import json
import os
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import time

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument("--log",type=Path)
args=parser.parse_args()
if sys.platform!="darwin":parser.error("requires macOS and Accessibility access")
sources=Path(__file__).resolve().parent
desktop=sources.parents[2]
with tempfile.TemporaryDirectory(prefix="option-tab-dock-observer-") as directory:
 root=Path(directory);bundle=root/"DockObserverFixture.app";macos=bundle/"Contents/MacOS";macos.mkdir(parents=True)
 with (bundle/"Contents/Info.plist").open("wb") as out:plistlib.dump({"CFBundleIdentifier":"org.optiontab.DockObserverFixture","CFBundleExecutable":"Fixture","CFBundleName":"DockObserverFixture","CFBundlePackageType":"APPL"},out)
 for source,target in [("dock_observer_fixture.m",macos/"Fixture"),("dock_icon_probe.m",root/"probe")]:subprocess.run(["clang","-fobjc-arc","-framework","Cocoa","-framework","ApplicationServices",str(sources/source),"-o",str(target)],check=True)
 binary=root/"platform.test";subprocess.run(["go","test","-c","./internal/platform","-o",str(binary)],cwd=desktop,check=True)
 fixture=subprocess.Popen([str(macos/"Fixture")],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 original=None;result=None
 try:
  for _ in range(25):
   probe=subprocess.run([str(root/"probe"),str(bundle)],text=True,capture_output=True)
   if probe.returncode==0:break
   if fixture.poll() is not None:raise RuntimeError("fixture exited")
   time.sleep(.2)
  else:raise RuntimeError(probe.stderr)
  icon=json.loads(probe.stdout);original=(icon["pointerX"],icon["pointerY"])
  # Reveal an auto-hidden Dock through ordinary pointer movement, never prefs.
  center=(icon["x"]+icon["w"]/2,icon["y"]+icon["h"]/2)
  reveal=(max(icon["screenX"]+1,min(center[0],icon["screenX"]+icon["screenW"]-1)),max(icon["screenY"]+1,min(center[1],icon["screenY"]+icon["screenH"]-1)))
  subprocess.run([str(root/"probe"),"--move",str(reveal[0]),str(reveal[1])],check=True)
  time.sleep(.8)
  # Launch/autohide/magnification animate icon bounds. Requery the exact URL.
  for _ in range(3):
   icon=json.loads(subprocess.check_output([str(root/"probe"),str(bundle)],text=True))
   center=(icon["x"]+icon["w"]/2,icon["y"]+icon["h"]/2)
   subprocess.run([str(root/"probe"),"--move",str(center[0]),str(center[1])],check=True)
   time.sleep(.3)
  env=os.environ.copy();env["OPTION_TAB_DOCK_OBSERVER_DIAGNOSTICS"]="1";env["OPTION_TAB_DOCK_FIXTURE_PID"]=str(fixture.pid);env["OPTION_TAB_DOCK_FIXTURE_PATH"]=str(bundle)
  result=subprocess.run([str(binary),"-test.run","^TestDockObserverDisposableIconSmoke$","-test.v","-test.count=1"],env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=35)
  output=f"Disposable app PID={fixture.pid}; native icon={icon}\n{result.stdout}";print(output)
  if args.log:args.log.write_text(output)
 finally:
  if original:subprocess.run([str(root/"probe"),"--move",str(original[0]),str(original[1])],check=False)
  if fixture.poll() is None:fixture.terminate()
  try:fixture.wait(timeout=5)
  except subprocess.TimeoutExpired:fixture.kill();fixture.wait()
 if result:sys.exit(result.returncode)

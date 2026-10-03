#!/usr/bin/env python3
"""Isolated real App/Wails/Dock smoke. Only disposable fixture PIDs are admitted."""
import os,pathlib,plistlib,subprocess,tempfile,json,time,sys
source=pathlib.Path(__file__).resolve().parent
desktop=source.parents[1]
root=pathlib.Path(tempfile.mkdtemp(prefix='option-tab-combined-smoke-'))
print('EVIDENCE',root,flush=True)
def bundle(name,bundle_id,exe,accessory=False):
 p=root/(name+'.app');(p/'Contents/MacOS').mkdir(parents=True)
 with (p/'Contents/Info.plist').open('wb') as f:plistlib.dump({'CFBundleExecutable':exe,'CFBundleIdentifier':bundle_id,'CFBundleName':name,'CFBundlePackageType':'APPL','NSHighResolutionCapable':True,'LSUIElement':accessory},f)
 return p
fixture=bundle('CombinedFixture','com.optiontab.disposable-combined-fixture','fixture')
empty=bundle('CombinedEmpty','com.optiontab.disposable-combined-empty','fixture')
app=bundle('CombinedSmoke','com.optiontab.disposable-combined-smoke','smoke',True)
subprocess.run(['clang','-fobjc-arc','-framework','Cocoa',str(source/'fixture.m'),'-o',str(fixture/'Contents/MacOS/fixture')],check=True)
import shutil
shutil.copy2(fixture/'Contents/MacOS/fixture',empty/'Contents/MacOS/fixture')
probe=root/'probe'
subprocess.run(['clang','-fobjc-arc','-framework','Cocoa','-framework','ApplicationServices',str(desktop/'internal/platform/testdata/dock_icon_probe.m'),'-o',str(probe)],check=True)
subprocess.run(['bun','run','build'],cwd=desktop/'frontend',check=True,stdout=(root/'frontend-build.log').open('w'),stderr=subprocess.STDOUT)
main=root/'main.go';main.write_text((source/'main.go.in').read_text().replace('__NATIVE__',str(source/'native.m')))
overlay=root/'overlay.json';overlay.write_text(json.dumps({'Replace':{str(desktop/'main.go'):str(main)}}))
built=subprocess.run(['go','build','-overlay',str(overlay),'-o',str(app/'Contents/MacOS/smoke'),'.'],cwd=desktop,stdout=(root/'build.log').open('w'),stderr=subprocess.STDOUT)
if built.returncode:print((root/'build.log').read_text());sys.exit(built.returncode)
original=json.loads(subprocess.check_output([str(probe),'--pointer']))
processes=[]
try:
 for path,args in [(fixture,[]),(empty,['--empty'])]:
  p=subprocess.Popen([str(path/'Contents/MacOS/fixture'),*args],stdout=(root/(path.stem+'.log')).open('w'),stderr=subprocess.STDOUT);processes.append(p)
 time.sleep(1)
 ids=[line.split()[2] for line in (root/'CombinedFixture.log').read_text().splitlines() if line.startswith('WINDOW ')]
 env=os.environ.copy();env.update(SMOKE_WINDOW_IDS=','.join(ids),SMOKE_DIR=str(root),SMOKE_FIXTURE_PID=str(processes[0].pid),SMOKE_EMPTY_PID=str(processes[1].pid),SMOKE_FIXTURE_APP=str(fixture),SMOKE_DOCK_PROBE=str(probe))
 result=subprocess.run([str(app/'Contents/MacOS/smoke')],env=env,cwd=desktop,stdout=(root/'smoke.log').open('w'),stderr=subprocess.STDOUT,timeout=90)
 print((root/'smoke.log').read_text())
 passed=(root/'passed').exists() and result.returncode==0
finally:
 for p in processes:
  if p.poll() is None:p.terminate()
 for p in processes:
  try:p.wait(timeout=4)
  except subprocess.TimeoutExpired:p.kill();p.wait()
 subprocess.run([str(probe),'--move',str(original['x']),str(original['y'])],check=False)
 print('CLEANUP fixtures terminated, pointer restored; evidence retained at',root,flush=True)
sys.exit(0 if passed else 1)

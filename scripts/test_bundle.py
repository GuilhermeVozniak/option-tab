"""Build orchestration tests only; fake tools do not prove signing or notarization."""
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
TOOL = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
with open(os.environ['CALL_LOG'], 'a') as f:
    f.write(json.dumps([name, args, {k: os.getenv(k) for k in ['MACOSX_DEPLOYMENT_TARGET','CGO_CFLAGS','CGO_LDFLAGS','GOARCH']}])+'\n')
if name == 'go':
    if args[0] == 'list': print('v3.0.0-alpha.102')
    elif args[0] == 'env': print('arm64')
    elif args[0] == 'run':
        if os.getenv('FAIL_BINDINGS'): sys.exit(21)
        pathlib.Path(os.environ['BINDINGS_MARKER']).touch()
    elif args[0] == 'build':
        p=pathlib.Path(args[args.index('-o')+1]);p.parent.mkdir(parents=True,exist_ok=True);p.touch()
elif name == 'lipo':
    if args[0] == '-archs': print(os.environ.get('FAKE_ARCHS','arm64 x86_64'))
    elif args[0] == '-create': pathlib.Path(args[args.index('-output')+1]).touch()
elif name == 'xcrun' and args[0] == 'notarytool' and os.getenv('FAIL_NOTARY'): sys.exit(23)
elif name == 'xcrun' and args[0] == 'vtool': print('    minos '+os.environ.get('FAKE_MINOS','14.0'))
elif name == 'iconutil': pathlib.Path(args[args.index('-o')+1]).touch()
elif name == 'npx': pathlib.Path(args[-1]).touch()
'''

class BundleContract(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for file in ('scripts/bundle.sh', 'scripts/generate-bindings.sh'):
            target = self.root / file
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy(ROOT / file, target)
        if os.getenv('BUNDLE_SCRIPT_UNDER_TEST'):
            shutil.copy(os.environ['BUNDLE_SCRIPT_UNDER_TEST'], self.root / 'scripts/bundle.sh')
        build = self.root / 'apps/desktop/build/darwin'
        build.mkdir(parents=True)
        (self.root / 'apps/desktop/frontend').mkdir()
        (build / 'Info.plist').write_bytes(plistlib.dumps({'CFBundleShortVersionString':'0.4.1', 'CFBundleVersion':'0.4.1', 'LSMinimumSystemVersion':'14.0', 'CFBundleExecutable':'option-tab', 'OSAScriptingDefinition':'OptionTab.sdef'}))
        (build / 'OptionTab.sdef').write_text('<dictionary/>')
        tools = self.root / 'tools'
        tools.mkdir()
        for name in ('go','bun','lipo','xcrun','sips','iconutil','npx','codesign','wails3'):
            p = tools / name
            p.write_text(TOOL)
            p.chmod(0o755)
        self.log = self.root / 'calls.jsonl'
        self.env = {**os.environ, 'PATH':f'{tools}:{os.environ["PATH"]}', 'CALL_LOG':str(self.log),
                    'BINDINGS_MARKER':str(self.root / 'bindings-ready'), 'VERSION':'0.5.0',
                    'UNIVERSAL':'1', 'BUNDLE_MODE':'release'}
        for key in ('CODESIGN_IDENTITY','APPLE_ID','APPLE_TEAM_ID','APPLE_APP_PASSWORD'):
            self.env[key] = 'fake-test-only'
    def run_bundle(self, **env):
        return subprocess.run(['bash','scripts/bundle.sh'], cwd=self.root, env={**self.env,**env}, capture_output=True,text=True)
    def calls(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []
    def test_universal_release_order_and_architecture(self):
        result = self.run_bundle()
        self.assertEqual(result.returncode,0,result.stderr)
        calls=self.calls()
        generated=next(i for i,c in enumerate(calls) if c[0]=='go' and c[1][0]=='run')
        front=next(i for i,c in enumerate(calls) if c[0]=='bun')
        self.assertLess(generated,front)
        self.assertEqual(calls[generated][1],['run','github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.102','generate','bindings'])
        builds=[c for c in calls if c[0]=='go' and c[1][0]=='build']
        self.assertEqual({c[2]['GOARCH'] for c in builds},{'arm64','amd64'})
        for c in builds:
            self.assertEqual(c[2]['MACOSX_DEPLOYMENT_TARGET'],'14.0')
            self.assertIn('-mmacosx-version-min=14.0',c[2]['CGO_LDFLAGS'])
        self.assertTrue((self.root/'apps/desktop/build/bin/option-tab_0.5.0_darwin_universal.dmg').exists())
        self.assertTrue(any(c[0]=='xcrun' and c[1][:2]==['notarytool','submit'] for c in calls))
        self.assertTrue(any(c[0]=='xcrun' and c[1][:2]==['stapler','validate'] for c in calls))
    def test_missing_signing_fails_before_build(self):
        result=self.run_bundle(APPLE_APP_PASSWORD='')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('APPLE_APP_PASSWORD',result.stderr)
        self.assertEqual(self.calls(),[])
    def test_failed_bindings_never_build_frontend(self):
        self.assertNotEqual(self.run_bundle(FAIL_BINDINGS='1').returncode,0)
        self.assertFalse(any(c[0]=='bun' for c in self.calls()))
    def test_thin_binary_cannot_be_published_as_universal(self):
        result=self.run_bundle(FAKE_ARCHS='arm64')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Architecture mismatch',result.stderr)
        self.assertFalse(any(c[0]=='npx' for c in self.calls()))
    def test_wrong_deployment_floor_stops_packaging(self):
        result=self.run_bundle(FAKE_MINOS='13.0')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Deployment floor mismatch',result.stderr)
        self.assertFalse(any(c[0]=='npx' for c in self.calls()))
    def test_incorrect_bundle_floor_refuses_release(self):
        plist=self.root/'apps/desktop/build/darwin/Info.plist'
        info=plistlib.loads(plist.read_bytes())
        info['LSMinimumSystemVersion']='13.0'
        plist.write_bytes(plistlib.dumps(info))
        result=self.run_bundle()
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Bundle minimum OS must be 14.0',result.stderr)
        self.assertFalse(any(c[0]=='codesign' for c in self.calls()))
    def test_notarization_failure_is_not_success(self):
        result=self.run_bundle(FAIL_NOTARY='1')
        self.assertEqual(result.returncode,23)
        self.assertNotIn('==> done:',result.stdout)
        self.assertFalse(any(c[0]=='xcrun' and c[1][:2]==['stapler','validate'] for c in self.calls()))
    def test_unsigned_is_explicitly_unverified(self):
        result=self.run_bundle(BUNDLE_MODE='unsigned',UNIVERSAL='0',FAKE_ARCHS='arm64')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertIn('UNVERIFIED',result.stdout)
        self.assertTrue((self.root/'apps/desktop/build/bin/option-tab_0.5.0_darwin_arm64_UNVERIFIED.dmg').exists())
        self.assertFalse(any(c[0]=='codesign' or (c[0]=='xcrun' and c[1][0]=='notarytool') for c in self.calls()))

if __name__ == '__main__':
    unittest.main()

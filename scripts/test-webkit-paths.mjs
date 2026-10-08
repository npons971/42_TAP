import assert from 'node:assert/strict';
import { chmodSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { webkitRuntimeEnv, webkitRuntimePaths } from './webkit-runtime.mjs';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
mkdirSync(path.join(repo, '.build'), { recursive: true });
const temporary = mkdtempSync(path.join(repo, '.build/webkit paths '));
function fixture(name, executablePrefix, bundlePrefix) {
  const sdk = path.join(temporary, name);
  const directory = path.join(sdk, executablePrefix, 'webkit2gtk-4.1');
  const bundle = path.join(sdk, bundlePrefix, 'webkit2gtk-4.1/injected-bundle');
  mkdirSync(directory, { recursive: true });
  mkdirSync(bundle, { recursive: true });
  for (const process of ['WebKitNetworkProcess', 'WebKitWebProcess']) {
    writeFileSync(path.join(directory, process), '', { mode: 0o755 });
  }
  writeFileSync(path.join(bundle, 'libwebkit2gtkinjectedbundle.so'), '');
  assert.deepEqual(webkitRuntimePaths(sdk), { directory, bundle });
  return { sdk, directory, bundle };
}
try {
  fixture('Fedora', 'usr/libexec', 'usr/lib64');
  fixture('Debian', 'usr/lib/x86_64-linux-gnu', 'usr/lib/x86_64-linux-gnu');
  const runtime = fixture('runtime/.webkit-sdk', 'usr/libexec', 'usr/lib64');
  const runtimeRepo = path.dirname(runtime.sdk);
  mkdirSync(path.join(runtimeRepo, '.build'));
  writeFileSync(path.join(runtimeRepo, '.build/libtap-webkit-launch.so'), '');
  const env = webkitRuntimeEnv(runtimeRepo, { LD_LIBRARY_PATH: '/host/libraries', CUSTOM: 'preserved' });
  assert.equal(env.TAP_WEBKIT_EXEC_PATH, runtime.directory);
  assert.equal(env.WEBKIT_INJECTED_BUNDLE_PATH, runtime.bundle);
  assert.ok(!env.LD_LIBRARY_PATH.includes('/host/libraries'));
  assert.equal(env.CUSTOM, 'preserved');
  const { sdk, directory, bundle } = fixture('incomplete', 'usr/lib', 'usr/lib');
  rmSync(path.join(bundle, 'libwebkit2gtkinjectedbundle.so'));
  assert.throws(() => webkitRuntimePaths(sdk), /bundle injecté/);
  writeFileSync(path.join(bundle, 'libwebkit2gtkinjectedbundle.so'), '');
  chmodSync(path.join(directory, 'WebKitWebProcess'), 0o644);
  assert.throws(() => webkitRuntimePaths(sdk), /Processus WebKit/);
  console.log('Chemins WebKit vérifiés : Fedora séparé, Debian, bundle absent, processus non exécutable.');
} finally {
  rmSync(temporary, { recursive: true, force: true });
}

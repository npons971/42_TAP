import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { webkitRuntimeEnv } from './webkit-runtime.mjs';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const env = webkitRuntimeEnv(repo);
env.LD_LIBRARY_PATH = ['usr/lib64', 'usr/lib/x86_64-linux-gnu', 'usr/lib']
  .map(dir => path.join(repo, '.webkit-sdk', dir)).concat(process.env.LD_LIBRARY_PATH || []).join(':');
env.PKG_CONFIG_PATH = ['usr/lib64/pkgconfig', 'usr/lib/x86_64-linux-gnu/pkgconfig', 'usr/lib/pkgconfig', 'usr/share/pkgconfig']
  .map(dir => path.join(repo, '.webkit-sdk', dir)).join(':');
function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: repo, env, encoding: 'utf8', ...options });
  assert.equal(result.error, undefined, result.error?.message);
  assert.equal(result.status, 0, result.stderr);
  return result.stdout.trim();
}
mkdirSync(path.join(repo, '.build'), { recursive: true });
const temporary = mkdtempSync(path.join(repo, '.build/webkit launch '));
try {
  const source = path.join(temporary, 'probe.c');
  const binary = path.join(temporary, 'probe');
  writeFileSync(source, `#include <gio/gio.h>
int main(int argc, char **argv) {
    if (argc < 2) return 2;
    GError *error = NULL;
    GSubprocessLauncher *launcher = g_subprocess_launcher_new(G_SUBPROCESS_FLAGS_STDOUT_PIPE);
    GSubprocess *child = g_subprocess_launcher_spawnv(launcher, (const gchar *const *)(argv + 1), &error);
    if (!child) { g_printerr("%s\\n", error->message); return 1; }
    gchar *output = NULL;
    if (!g_subprocess_communicate_utf8(child, NULL, NULL, &output, NULL, &error)) return 1;
    g_print("%s", output);
    int result = g_subprocess_get_successful(child) ? 0 : 1;
    g_free(output); g_object_unref(child); g_object_unref(launcher);
    return result;
}
`);
  const flags = run(path.join(repo, '.webkit-sdk/usr/bin/pkg-config'), ['--cflags', '--libs', 'gio-2.0']);
  run(path.join(repo, 'cc'), ['-Wall', '-Wextra', '-Werror', source, '-o', binary, ...flags.split(/\s+/)]);
  const names = ['WebKitNetworkProcess', 'WebKitWebProcess', 'WebKitGPUProcess'];
  for (const name of names) symlinkSync(path.join(repo, 'node'), path.join(temporary, name));
  const local = { ...env, TAP_WEBKIT_EXEC_PATH: temporary };
  for (const prefix of ['/usr/lib/x86_64-linux-gnu', '/usr/lib64', '/usr/libexec', '/usr/lib']) {
    for (const name of names) {
      assert.equal(run(binary, [`${prefix}/webkit2gtk-4.1/${name}`, '-e', 'console.log("local helper launched")'], { env: local }), 'local helper launched');
    }
  }
  assert.equal(run(binary, [path.join(repo, 'node'), '-e', 'console.log("ordinary command")']), 'ordinary command');
  // No redirect unless the launcher explicitly supplies the local SDK path.
  const inherited = { ...env };
  delete inherited.TAP_WEBKIT_EXEC_PATH;
  assert.equal(run(binary, [path.join(repo, 'node'), '-e', 'console.log("no redirect")'], { env: inherited }), 'no redirect');
  console.log('WebKit local launch checks passed (GIO, Debian/Fedora paths, spaces, ordinary commands).');
} finally {
  rmSync(temporary, { recursive: true, force: true });
}

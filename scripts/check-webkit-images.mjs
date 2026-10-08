// Exercise the same image decoder as GTK without needing a display server.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { webkitRuntimeEnv } from './webkit-runtime.mjs';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sdk = path.join(repo, '.webkit-sdk');
for (const tool of ['node', 'cc']) {
  fs.accessSync(path.join(repo, tool), fs.constants.X_OK);
}
const env = webkitRuntimeEnv(repo);
env.PATH = [repo, path.join(sdk, 'usr/bin'), env.PATH].filter(Boolean).join(':');
env.PKG_CONFIG_LIBDIR = ['usr/lib64/pkgconfig', 'usr/lib/x86_64-linux-gnu/pkgconfig',
  'usr/lib/pkgconfig', 'usr/share/pkgconfig'].map(dir => path.join(sdk, dir)).join(':');
env.PKG_CONFIG_PATH = env.PKG_CONFIG_LIBDIR;
const flags = execFileSync(path.join(sdk, 'usr/bin/pkg-config'),
  ['--cflags', '--libs', 'gdk-pixbuf-2.0'], { env, encoding: 'utf8' }).trim().split(/\s+/);
fs.mkdirSync(path.join(repo, '.build'), { recursive: true });
const binary = path.join(repo, '.build/check-webkit-images');
execFileSync(path.join(repo, 'cc'), ['-Wall', '-Wextra', '-Werror',
  path.join(repo, 'scripts/check-webkit-images.c'), '-o', binary, ...flags], { env, stdio: 'inherit' });
execFileSync(binary, [path.join(sdk, 'usr/share/icons/Adwaita/scalable/status/image-missing.svg')],
  { env, stdio: 'inherit', timeout: 20000 });

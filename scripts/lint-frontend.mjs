import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const root = new URL('../', import.meta.url);
const require = createRequire(new URL('Project/client-gui/frontend/package.json', root));
const { compile } = require('svelte/compiler');
const listing = spawnSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '--', '*.js', '*.mjs', '*.svelte'], {
  cwd: root, encoding: 'utf8',
});
if (listing.status !== 0) {
  process.stderr.write(listing.stderr);
  process.exit(1);
}
let failed = false;
for (const path of listing.stdout.trim().split('\n').filter(Boolean)) {
  // Wails runtime bindings are generated third-party code.
  if (path.includes('/wailsjs/')) continue;
  const file = new URL(path, root);
  if (!path.endsWith('.svelte')) {
    const result = spawnSync(process.execPath, ['--check', fileURLToPath(file)], { encoding: 'utf8' });
    if (result.status !== 0) {
      process.stderr.write(result.stderr || result.error?.message || `Cannot check ${path}\n`);
      failed = true;
    }
    continue;
  }
  try {
    const { warnings } = compile(readFileSync(file, 'utf8'), { filename: path, generate: false });
    for (const warning of warnings) {
      console.error(`${path}:${warning.start?.line ?? 1}:${warning.start?.column ?? 0}: ${warning.code}: ${warning.message}`);
      failed = true;
    }
  } catch (error) {
    console.error(`${path}: ${error.message}`);
    failed = true;
  }
}
if (failed) process.exit(1);
console.log('JavaScript syntax and Svelte compiler checks passed.');

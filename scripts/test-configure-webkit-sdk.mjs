import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const configure = path.join(repo, 'scripts/configure-webkit-sdk.mjs');
fs.mkdirSync(path.join(repo, '.build'), { recursive: true });
const temporary = fs.mkdtempSync(path.join(repo, '.build/webkit-linker-'));
function write(root, relative, contents = '') {
  const filename = path.join(root, relative);
  fs.mkdirSync(path.dirname(filename), { recursive: true });
  fs.writeFileSync(filename, contents);
}
function run(sdk, destination) {
  const result = spawnSync(process.execPath, [configure, sdk, destination], { encoding: 'utf8' });
  assert.equal(result.error, undefined, result.error?.message);
  return result;
}
function setup(name, directory) {
  const sdk = path.join(temporary, `${name}.stage`);
  const destination = path.join(temporary, name);
  write(sdk, 'usr/include/stdlib.h');
  write(sdk, `${directory}/crt1.o`);
  return { sdk, destination, directory };
}
function verify({ sdk, destination, directory }, references, resolved, extra = '') {
  const relativeScript = `${directory}/libm.so`;
  const source = `/* GNU ld script */\nGROUP ( ${references[0]} AS_NEEDED ( ${references.slice(1).join(' ')} ) )\n${extra}`;
  write(sdk, relativeScript, source);
  const expected = `/* GNU ld script */\nGROUP ( ${path.join(destination, resolved[0])} AS_NEEDED ( ${resolved.slice(1).map(file => path.join(destination, file)).join(' ')} ) )\n${extra}`;
  for (let repeat = 0; repeat < 2; repeat++) {
    const result = run(sdk, destination);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(fs.readFileSync(path.join(sdk, relativeScript), 'utf8'), expected);
    const libc = fs.readFileSync(path.join(sdk, '.zig-libc'), 'utf8');
    assert.ok(libc.includes(`crt_dir=${path.join(destination, directory)}\n`));
    assert.ok(!libc.includes(sdk));
  }
  // Generated paths must also remain valid after the staging tree is promoted.
  fs.renameSync(sdk, destination);
  const result = run(destination, destination);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(fs.readFileSync(path.join(destination, relativeScript), 'utf8'), expected);
  for (const relative of resolved) assert.ok(fs.statSync(path.join(destination, relative)).isFile());
}
try {
  const multiarch = 'usr/lib/x86_64-linux-gnu';
  const ubuntu = setup('Ubuntu', multiarch);
  const ubuntuLibraries = [`${multiarch}/libm.so.6`, `${multiarch}/libmvec.so.1`, `${multiarch}/libc_nonshared.a`];
  for (const relative of ubuntuLibraries) write(ubuntu.sdk, relative);
  write(ubuntu.sdk, `${multiarch}/ld-linux-x86-64.so.2`);
  fs.mkdirSync(path.join(ubuntu.sdk, 'usr/lib64'), { recursive: true });
  fs.symlinkSync('../lib/x86_64-linux-gnu/ld-linux-x86-64.so.2', path.join(ubuntu.sdk, 'usr/lib64/ld-linux-x86-64.so.2'));
  verify(ubuntu, ['/lib/x86_64-linux-gnu/libm.so.6', '/lib/x86_64-linux-gnu/libmvec.so.1',
    '/usr/lib/x86_64-linux-gnu/libc_nonshared.a', '/lib64/ld-linux-x86-64.so.2'],
  [...ubuntuLibraries, 'usr/lib64/ld-linux-x86-64.so.2']);

  const legacy = setup('Debian', multiarch);
  write(legacy.sdk, 'lib/x86_64-linux-gnu/libc.so.6');
  write(legacy.sdk, `${multiarch}/libc.so.6`, 'alternative must not replace exact path');
  write(legacy.sdk, 'lib64/ld-linux-x86-64.so.2');
  verify(legacy, ['/lib/x86_64-linux-gnu/libc.so.6', '/lib64/ld-linux-x86-64.so.2'],
    ['lib/x86_64-linux-gnu/libc.so.6', 'lib64/ld-linux-x86-64.so.2']);

  const fedora = setup('Fedora', 'usr/lib64');
  write(fedora.sdk, 'usr/lib64/libm.so.6');
  write(fedora.sdk, 'usr/lib64/libc_nonshared.a');
  verify(fedora, ['/lib64/libm.so.6', '/usr/lib64/libc_nonshared.a'],
    ['usr/lib64/libm.so.6', 'usr/lib64/libc_nonshared.a']);

  const nested = setup('nested', multiarch);
  write(nested.sdk, `${multiarch}/private/libexample.so.1`);
  verify(nested, ['/lib/x86_64-linux-gnu/private/libexample.so.1'],
    [`${multiarch}/private/libexample.so.1`]);

  const repaired = setup('already-configured', multiarch);
  write(repaired.sdk, `${multiarch}/libc.so.6`);
  write(repaired.sdk, `${multiarch}/ld-linux-x86-64.so.2`);
  // Repair the previously emitted nonexistent destination paths, and resolve
  // a loader without a lib64 alias. Staging paths must not leak into output.
  verify(repaired, [path.join(repaired.destination, 'lib/x86_64-linux-gnu/libc.so.6'),
    path.join(repaired.sdk, 'lib64/ld-linux-x86-64.so.2')],
  [`${multiarch}/libc.so.6`, `${multiarch}/ld-linux-x86-64.so.2`]);

  const missing = setup('missing', multiarch);
  const missingScript = `${multiarch}/libm.so`;
  const source = '/* GNU ld script */\nGROUP ( /lib/x86_64-linux-gnu/libm.so.6 )\n';
  write(missing.sdk, missingScript, source);
  const missingResult = run(missing.sdk, missing.destination);
  assert.notEqual(missingResult.status, 0);
  assert.match(missingResult.stderr, /Bibliothèque locale introuvable.*\/lib\/x86_64-linux-gnu\/libm\.so\.6/);
  assert.equal(fs.readFileSync(path.join(missing.sdk, missingScript), 'utf8'), source);

  // An existing library outside the SDK must not satisfy a missing dependency.
  const outside = path.join(temporary, 'host-library.so.6');
  fs.writeFileSync(outside, 'external library');
  fs.symlinkSync(outside, path.join(missing.sdk, multiarch, 'libm.so.6'));
  assert.notEqual(run(missing.sdk, missing.destination).status, 0);
  console.log('Scripts ld locaux vérifiés : Ubuntu, Debian, Fedora, reconfiguration, promotion, dépendance absente et lien externe.');
} finally {
  fs.rmSync(temporary, { recursive: true, force: true });
}

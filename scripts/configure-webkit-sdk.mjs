import fs from 'node:fs';
import path from 'node:path';

const sdk = path.resolve(process.argv[2]);
const destination = path.resolve(process.argv[3]);
const xml = value => value.replace(/[&<>"']/g, character => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;',
}[character]));
fs.mkdirSync(path.join(sdk, 'etc/fonts'), { recursive: true });
fs.writeFileSync(path.join(sdk, 'etc/fonts/tap-fonts.conf'), `<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <dir>${xml(path.join(destination, 'usr/share/fonts'))}</dir>
  <cachedir>${xml(path.join(path.dirname(destination), '.build/fontconfig'))}</cachedir>
  <alias><family>sans-serif</family><prefer><family>Noto Sans</family></prefer></alias>
</fontconfig>
`);
const directories = ['usr/lib64', 'usr/lib/x86_64-linux-gnu', 'usr/lib',
  'lib64', 'lib/x86_64-linux-gnu', 'lib'];
const crt = directories.find(dir => fs.existsSync(path.join(sdk, dir, 'crt1.o')));
if (!crt || !fs.existsSync(path.join(sdk, 'usr/include/stdlib.h'))) {
  throw new Error('Headers et fichiers de démarrage libc locaux manquants.');
}
const systemInclude = fs.existsSync(path.join(sdk, 'usr/include/x86_64-linux-gnu/sys/errno.h'))
  ? 'usr/include/x86_64-linux-gnu' : 'usr/include';
// ZIG_LIBC controls Zig's native include/CRT discovery. --sysroot alone leaves
// its native headers on the host and prefixes absolute pkg-config -L twice.
fs.writeFileSync(path.join(sdk, '.zig-libc'), [
  `include_dir=${path.join(destination, 'usr/include')}`,
  `sys_include_dir=${path.join(destination, systemInclude)}`,
  `crt_dir=${path.join(destination, crt)}`,
  'msvc_lib_dir=', 'kernel32_lib_dir=', 'gcc_dir=', '',
].join('\n'));
// libc.so and libm.so may be GNU linker scripts with absolute system paths.
const visited = new Set();
for (const dir of directories) {
  const directory = path.join(sdk, dir);
  if (!fs.existsSync(directory)) continue;
  const canonical = fs.realpathSync(directory);
  if (visited.has(canonical)) continue;
  visited.add(canonical);
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.so')) continue;
    const filename = path.join(directory, entry.name);
    const source = fs.readFileSync(filename);
    if (!source.subarray(0, 128).toString().includes('GNU ld script')) continue;
    const text = source.toString();
    fs.writeFileSync(filename, text.replace(/\/(?:usr\/)?lib(?:64|\/x86_64-linux-gnu)?\/[\w.+-]+/g,
      (value, offset) => text.slice(0, offset).endsWith(destination) ? value : path.join(destination, value)));
  }
}

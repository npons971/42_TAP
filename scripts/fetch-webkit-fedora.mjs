// Read Fedora repository metadata and extract RPMs locally; never install them
// into the system database or invoke a system package manager.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const [, release, ...flags] = process.argv.slice(2);
if (!/^\d+$/.test(release || '') || process.platform !== 'linux' || process.arch !== 'x64') {
  throw new Error('Expected a Fedora release number on Linux x86_64.');
}
const cache = path.join(repo, 'install_files/webkit');
const indexes = path.join(cache, 'indexes');
fs.mkdirSync(indexes, { recursive: true });
const decode = value => value.replace(/&quot;/g, '"').replace(/&apos;/g, "'")
  .replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
const attributes = text => Object.fromEntries([...text.matchAll(/([\w:-]+)="([^"]*)"/g)]
  .map(([, key, value]) => [key, decode(value)]));
const tag = (text, name) => decode(new RegExp(`<${name}(?:\\s[^>]*)?>([^<]*)</${name}>`).exec(text)?.[1] || '');
const entries = (text, name) => [...(new RegExp(`<rpm:${name}>([\\s\\S]*?)</rpm:${name}>`).exec(text)?.[1] || '')
  .matchAll(/<rpm:entry\s+([^>]+)\/>/g)].map(([, value]) => attributes(value));
const evr = value => `${value.epoch || '0'}:${value.ver}-${value.rel || '0'}`;
const comparisons = new Map();
function compare(a, b) {
  if (a === b) return 0;
  const key = `${a}\t${b}`;
  if (!comparisons.has(key)) {
    // JSON string syntax also quotes the numeric RPM EVR safely for Lua.
    const expression = `%{lua:print(rpm.vercmp(${JSON.stringify(a)}, ${JSON.stringify(b)}))}`;
    comparisons.set(key, Number(execFileSync('rpm', ['--eval', expression], { encoding: 'utf8' }).trim()));
  }
  return comparisons.get(key);
}
function satisfies(actual, requirement) {
  if (!requirement.flags) return true;
  if (!actual) return false;
  const result = compare(actual, evr(requirement));
  return { EQ: result === 0, GE: result >= 0, LE: result <= 0, GT: result > 0, LT: result < 0 }[requirement.flags] || false;
}
function download(url, destination) {
  execFileSync('curl', ['-fL', '--retry', '3', '--connect-timeout', '15', '--max-time', '180', '-o', destination + '.tmp', url], { stdio: 'inherit' });
  fs.renameSync(destination + '.tmp', destination);
}
function verified(filename, checksum) {
  return fs.existsSync(filename) && createHash('sha256').update(fs.readFileSync(filename)).digest('hex') === checksum;
}
const packages = new Map();
for (const [suite, base] of [
  ['release', `https://dl.fedoraproject.org/pub/fedora/linux/releases/${release}/Everything/x86_64/os`],
  ['updates', `https://dl.fedoraproject.org/pub/fedora/linux/updates/${release}/Everything/x86_64`],
]) {
  const manifest = path.join(indexes, `fedora-${release}-${suite}-repomd.xml`);
  if (!fs.existsSync(manifest) || flags.includes('--refresh')) download(`${base}/repodata/repomd.xml`, manifest);
  const primary = /<data type="primary">([\s\S]*?)<\/data>/.exec(fs.readFileSync(manifest, 'utf8'))?.[1];
  if (!primary || !/<checksum type="sha256">/.test(primary)) throw new Error('Invalid Fedora primary metadata.');
  const location = attributes(/<location\s+([^>]+)\/>/.exec(primary)?.[1] || '').href;
  if (!/^repodata\/[\w.-]+$/.test(location || '')) throw new Error('Invalid Fedora metadata location.');
  const checksum = tag(primary, 'checksum');
  const index = path.join(indexes, path.basename(location));
  if (!verified(index, checksum)) download(`${base}/${location}`, index);
  if (!verified(index, checksum)) throw new Error('Fedora metadata checksum mismatch.');
  console.log(`Index GTK/WebKit : Fedora ${release}/${suite}`);
  const command = location.endsWith('.zst') ? 'zstd' : location.endsWith('.xz') ? 'xz' : 'gzip';
  const source = execFileSync(command, ['-dc', index], { encoding: 'utf8', maxBuffer: 512 * 1024 * 1024 });
  for (const [, block] of source.matchAll(/<package type="rpm">([\s\S]*?)<\/package>/g)) {
    const arch = tag(block, 'arch');
    if (!['x86_64', 'noarch'].includes(arch)) continue;
    const name = tag(block, 'name');
    const version = attributes(/<version\s+([^>]+)\/>/.exec(block)?.[1] || '');
    const location = attributes(/<location\s+([^>]+)\/>/.exec(block)?.[1] || '').href;
    if (!/^Packages\/[\w./+~^-]+\.rpm$/.test(location || '') || location.includes('..')) throw new Error('Invalid RPM location.');
    if (!packages.has(name)) packages.set(name, []);
    packages.get(name).push({ name, version: evr(version), location, base, checksum: tag(block, 'checksum'),
      provides: entries(block, 'provides'), requires: entries(block, 'requires'),
      files: [...block.matchAll(/<file(?:\s[^>]*)?>([^<]+)<\/file>/g)].map(([, value]) => decode(value)) });
  }
}
const providers = new Map();
function provide(name, pkg, version) {
  if (!providers.has(name)) providers.set(name, []);
  providers.get(name).push({ pkg, version });
}
for (const pkg of [...packages.values()].flat()) {
  for (const value of pkg.provides) provide(value.name, pkg, value.ver ? evr(value) : '');
  for (const file of pkg.files) provide(file, pkg, '');
}
const installed = new Map();
const inventory = execFileSync('rpm', ['-qa', '--qf', '[%{PROVIDENAME}\t%{PROVIDEVERSION}\n]'], { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
for (const line of inventory.trim().split('\n')) {
  const [name, version] = line.split('\t');
  if (!installed.has(name)) installed.set(name, []);
  installed.get(name).push(version === '(none)' ? '' : version);
}
const selected = new Map();
function conditional(requirement) {
  if (!requirement.name.startsWith('(')) return requirement;
  // Fedora development packages conditionally require RPM build macros.
  const match = /^\((.+?) if ([\w.+-]+)\)$/.exec(requirement.name);
  if (!match) throw new Error('Unsupported RPM dependency: ' + requirement.name);
  if (!installed.has(match[2]) && !selected.has(match[2])) return null;
  const atom = /^([^\s]+)(?:\s+(=|>=|<=|>|<)\s+(\S+))?$/.exec(match[1]);
  if (!atom) throw new Error('Unsupported RPM dependency: ' + requirement.name);
  const [, name, operator, version] = atom;
  if (!operator) return { name };
  const value = /^(?:(\d+):)?(.+)-([^-]+)$/.exec(version);
  if (!value) throw new Error('Invalid RPM dependency version: ' + version);
  return { name, flags: { '=': 'EQ', '>=': 'GE', '<=': 'LE', '>': 'GT', '<': 'LT' }[operator],
    epoch: value[1] || '0', ver: value[2], rel: value[3] };
}
function include(pkg) {
  if (selected.has(pkg.name)) return;
  selected.set(pkg.name, pkg);
  for (const dependency of pkg.requires) {
    const requirement = conditional(dependency);
    if (!requirement) continue;
    if (requirement.name.startsWith('rpmlib(')) continue;
    // Development headers must be extracted even if installed on this host.
    const development = /-devel(?:\(|$)/.test(requirement.name) || requirement.name.startsWith('pkgconfig(');
    if (!development && (installed.get(requirement.name) || []).some(value => satisfies(value, requirement))) continue;
    if (requirement.name.startsWith('/') && fs.existsSync(requirement.name)) continue;
    const candidates = providers.get(requirement.name) || [];
    const provider = candidates.filter(value => satisfies(value.version, requirement))
      .sort((a, b) => compare(b.pkg.version, a.pkg.version))[0];
    if (!provider) throw new Error(`Cannot resolve ${pkg.name}: ${requirement.name}`);
    include(provider.pkg);
  }
}
for (const name of ['gtk3-devel', 'webkit2gtk4.1-devel', 'pkgconf', 'pkgconf-pkg-config', 'libpkgconf']) {
  const pkg = packages.get(name)?.sort((a, b) => compare(b.version, a.version))[0];
  if (!pkg) throw new Error('Package unavailable in this Fedora release: ' + name);
  include(pkg);
}
console.log(`Archives GTK/WebKit locales : ${selected.size} paquets.`);
if (flags.includes('--plan')) console.log([...selected.keys()].sort().join('\n'));
else {
  const files = [];
  for (const pkg of selected.values()) {
    const filename = path.basename(pkg.location);
    const destination = path.join(cache, filename);
    if (!verified(destination, pkg.checksum)) {
      console.log('Archive GTK/WebKit : ' + filename);
      download(`${pkg.base}/${pkg.location}`, destination);
    }
    if (!verified(destination, pkg.checksum)) throw new Error('RPM checksum mismatch: ' + filename);
    files.push(filename);
  }
  fs.writeFileSync(path.join(cache, '.selected-archives'), files.join('\n') + '\n');
}

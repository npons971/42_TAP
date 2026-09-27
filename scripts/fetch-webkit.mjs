// Download distribution archives into the repository without a package manager.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const cache = path.join(repo, 'install_files/webkit');
const [distribution, codename, ...flags] = process.argv.slice(2);
if (!['ubuntu', 'debian'].includes(distribution) || !/^[a-z]+$/.test(codename || '')) {
  throw new Error('Expected an Ubuntu/Debian distribution and release codename.');
}
if (process.platform !== 'linux' || process.arch !== 'x64') {
  throw new Error('The local SDK currently supports Linux x86_64 only.');
}
fs.mkdirSync(cache, { recursive: true });
const indexes = path.join(cache, 'indexes');
fs.mkdirSync(indexes, { recursive: true });
const packages = new Map(), providers = new Map();
// Keep the host's existing runtime ABI; headers and missing libraries go local.
const installed = new Map();
const inventory = execFileSync('dpkg-query', ['-W', '-f=${binary:Package}\t${Version}\t${db:Status-Status}\t${Architecture}\n'], { encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
for (const line of inventory.split('\n')) {
  const [name, version, status, architecture] = line.split('\t');
  if (status === 'installed' && ['amd64', 'all'].includes(architecture)) installed.set(name.replace(/:.*$/, ''), version);
}
const versionResults = new Map();
const compare = (a, operator, b) => {
  if (a === b) return ['=', '>=', '<=', 'eq', 'ge', 'le'].includes(operator);
  const key = [a, operator, b].join(' ');
  if (versionResults.has(key)) return versionResults.get(key);
  let result;
  try { execFileSync('dpkg', ['--compare-versions', a, operator, b], { stdio: 'ignore' }); result = true; }
  catch { result = false; }
  versionResults.set(key, result);
  return result;
};
function download(url, destination) {
  execFileSync('curl', ['-fL', '--retry', '3', '--connect-timeout', '15', '--max-time', '180', '-o', destination + '.tmp', url], { stdio: 'inherit' });
  fs.renameSync(destination + '.tmp', destination);
}
const sources = distribution === 'ubuntu'
  ? [['https://archive.ubuntu.com/ubuntu', codename], ['https://archive.ubuntu.com/ubuntu', codename + '-updates'], ['https://security.ubuntu.com/ubuntu', codename + '-security']]
  : [['https://deb.debian.org/debian', codename], ['https://deb.debian.org/debian', codename + '-updates'], ['https://security.debian.org/debian-security', codename + '-security']];
for (const [base, suite] of sources) {
  for (const component of distribution === 'ubuntu' ? ['main', 'universe'] : ['main']) {
    const index = path.join(indexes, suite + '-' + component + '.xz');
    if (!fs.existsSync(index) || flags.includes('--refresh')) {
      console.log('Index GTK/WebKit : ' + suite + '/' + component);
      download(base + '/dists/' + suite + '/' + component + '/binary-amd64/Packages.xz', index);
    }
    const source = execFileSync('xz', ['-dc', index], { encoding: 'utf8', maxBuffer: 300 * 1024 * 1024 });
    for (const block of source.split('\n\n')) {
      const fields = {};
      for (const line of block.split('\n')) {
        const match = /^([\w-]+): (.*)$/.exec(line);
        if (match && ['Package', 'Version', 'Filename', 'SHA256', 'Provides', 'Depends', 'Pre-Depends'].includes(match[1])) fields[match[1]] = match[2];
      }
      if (!fields.Package || !fields.Filename || !fields.SHA256) continue;
      const current = packages.get(fields.Package);
      if (!current || compare(fields.Version, 'gt', current.Version)) packages.set(fields.Package, { ...fields, base });
    }
  }
}
for (const pkg of packages.values()) {
  for (const provided of (pkg.Provides || '').split(', ')) {
    const name = provided.split(' ')[0];
    if (!providers.has(name)) providers.set(name, pkg);
  }
}
const selected = new Map();
function resolve(dependency) {
  const match = /^([\w.+-]+)(?::[\w-]+)?(?:\s+\((>=|<=|=|<<|>>)\s+([^\)]+)\))?/.exec(dependency.trim());
  if (!match) throw new Error('Unsupported dependency: ' + dependency);
  const [, name, operator, version] = match;
  const localVersion = installed.get(name);
  if (!/-dev(?:-bin)?$/.test(name) && localVersion && (!operator || compare(localVersion, operator, version))) {
    return { installed: true };
  }
  const pkg = packages.get(name) || providers.get(name);
  if (!pkg || (operator && !compare(pkg.Version, operator, version))) return null;
  return pkg;
}
function include(pkg) {
  if (selected.has(pkg.Package)) return;
  selected.set(pkg.Package, pkg);
  for (const group of [pkg['Pre-Depends'], pkg.Depends].filter(Boolean).join(', ').split(', ').filter(Boolean)) {
    const alternative = group.split(/\s*\|\s*/).map(resolve).find(Boolean);
    if (!alternative) throw new Error('Cannot resolve ' + pkg.Package + ': ' + group);
    if (!alternative.installed) include(alternative);
  }
}
for (const name of ['libgtk-3-dev', 'libwebkit2gtk-4.1-dev', 'pkgconf', 'pkgconf-bin', 'libpkgconf3']) {
  const pkg = packages.get(name);
  if (!pkg) throw new Error('Package unavailable in this release: ' + name);
  include(pkg);
}
console.log('Archives GTK/WebKit locales : ' + selected.size + ' paquets.');
if (flags.includes('--plan')) {
  console.log([...selected.keys()].sort().join('\n'));
  process.exit(0);
}
const files = [];
for (const pkg of selected.values()) {
  const filename = path.basename(pkg.Filename);
  const destination = path.join(cache, filename);
  const valid = () => fs.existsSync(destination) && createHash('sha256').update(fs.readFileSync(destination)).digest('hex') === pkg.SHA256;
  if (!valid()) download(pkg.base + '/' + pkg.Filename, destination);
  if (!valid()) throw new Error('Archive checksum mismatch: ' + filename);
  files.push(filename);
}
fs.writeFileSync(path.join(cache, '.selected-archives'), files.join('\n') + '\n');

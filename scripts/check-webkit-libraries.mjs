// Ask the SDK's own ELF loader to resolve GTK/WebKit and their dependencies.
// Fail if a library is missing or resolved outside the repository-local SDK.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { webkitLibraryPaths, webkitRuntimePaths } from './webkit-runtime.mjs';

const sdk = path.resolve(process.argv[2]);
const libraries = webkitLibraryPaths(sdk);
const locate = name => libraries.map(dir => path.join(dir, name)).find(file => fs.existsSync(file));
const loader = locate('ld-linux-x86-64.so.2');
if (!loader) throw new Error('Chargeur ELF local manquant dans le SDK.');
const { directory, bundle } = webkitRuntimePaths(sdk);
const targets = ['libgtk-3.so.0', 'libwebkit2gtk-4.1.so.0'].map(name => {
  const file = locate(name);
  if (!file) throw new Error('Bibliothèque locale manquante : ' + name);
  return file;
}).concat(['WebKitNetworkProcess', 'WebKitWebProcess'].map(name => path.join(directory, name)),
  path.join(bundle, 'libwebkit2gtkinjectedbundle.so'));
if (fs.existsSync(path.join(directory, 'WebKitGPUProcess'))) targets.push(path.join(directory, 'WebKitGPUProcess'));
for (const target of targets) {
  const env = { ...process.env, LD_LIBRARY_PATH: libraries.join(':') };
  delete env.LD_PRELOAD;
  const output = execFileSync(loader, ['--library-path', libraries.join(':'), '--list', target], { env, encoding: 'utf8' });
  for (const line of output.split('\n')) {
    const resolved = /=> (\/\S+)\s+\(/.exec(line)?.[1];
    if (resolved && !fs.realpathSync(resolved).startsWith(fs.realpathSync(sdk) + path.sep)) {
      throw new Error(`${path.basename(target)} utilise une bibliothèque de l'hôte : ${resolved}`);
    }
    if (line.includes('not found')) throw new Error(`${path.basename(target)} : ${line.trim()}`);
  }
}
console.log('GTK, WebKit, bundle et processus : toutes les bibliothèques liées sont dans le SDK local.');

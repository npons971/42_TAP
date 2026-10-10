// Archive extraction skips the package hooks that register image formats.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { webkitLibraryPaths } from './webkit-runtime.mjs';

const sdk = path.resolve(process.argv[2]);
const destination = path.resolve(process.argv[3]);
const libraries = webkitLibraryPaths(sdk);
const env = { ...process.env, LD_LIBRARY_PATH: libraries.join(':'),
  XDG_DATA_DIRS: path.join(sdk, 'usr/share') };
delete env.LD_PRELOAD;
const mime = path.join(sdk, 'usr/share/mime');
execFileSync(path.join(sdk, 'usr/bin/update-mime-database'), [mime], { env, stdio: 'inherit' });

// Fedora installs the query tool in bin with a -64 suffix; Debian/Ubuntu may
// put it in the pixbuf root or the version directory beside the modules.
// Discover the module directory independently of the tool.
const queries = [path.join(sdk, 'usr/bin/gdk-pixbuf-query-loaders-64'),
  path.join(sdk, 'usr/bin/gdk-pixbuf-query-loaders')];
for (const library of libraries) {
  const pixbuf = path.join(library, 'gdk-pixbuf-2.0');
  if (!fs.existsSync(pixbuf)) continue;
  for (const version of fs.readdirSync(pixbuf)) {
    const directory = path.join(pixbuf, version);
    const loaders = path.join(directory, 'loaders');
    if (!fs.existsSync(loaders)) continue;
    const query = [path.join(directory, 'gdk-pixbuf-query-loaders'),
      path.join(pixbuf, 'gdk-pixbuf-query-loaders'), ...queries]
      .find(file => fs.existsSync(file));
    if (!query) throw new Error('Outil local gdk-pixbuf-query-loaders manquant.');
    const modules = fs.readdirSync(loaders).filter(name => name.endsWith('.so'))
      .map(name => path.join(loaders, name));
    if (!modules.length) continue;
    const cache = execFileSync(query, modules, { env, encoding: 'utf8' });
    fs.writeFileSync(path.join(directory, 'loaders.cache'), cache.replaceAll(sdk, destination));
  }
}

// New GdkPixbuf uses Glycin instead of legacy modules. Its configuration must
// launch the extracted helpers, including after the staging directory moves.
const glycin = path.join(sdk, 'usr/share/glycin-loaders');
if (fs.existsSync(glycin)) {
  for (const version of fs.readdirSync(glycin)) {
    const directory = path.join(glycin, version, 'conf.d');
    if (!fs.existsSync(directory)) continue;
    for (const name of fs.readdirSync(directory).filter(name => name.endsWith('.conf'))) {
      const file = path.join(directory, name);
      const source = fs.readFileSync(file, 'utf8');
      fs.writeFileSync(file, source.replace(/^Exec=(\/usr\/[^\r\n]+)$/gm,
        (_, executable) => {
          if (!fs.existsSync(path.join(sdk, executable))) {
            throw new Error('Chargeur Glycin local manquant : ' + executable);
          }
          return 'Exec=' + path.join(destination, executable);
        }));
    }
  }
}

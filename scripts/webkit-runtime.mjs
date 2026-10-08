import { accessSync, constants, existsSync, readdirSync, realpathSync } from 'node:fs';
import path from 'node:path';

export function webkitLibraryPaths(sdk) {
  const libraries = ['usr/lib64', 'usr/lib/x86_64-linux-gnu', 'usr/lib',
    'lib64', 'lib/x86_64-linux-gnu', 'lib'].map(dir => path.join(sdk, dir));
  const seen = new Set();
  function visit(directory) {
    if (!existsSync(directory)) return;
    const canonical = realpathSync(directory);
    if (seen.has(canonical) || !canonical.startsWith(realpathSync(sdk) + path.sep)) return;
    seen.add(canonical);
    const entries = readdirSync(directory, { withFileTypes: true });
    // Private libraries (notably PulseAudio) have an absolute distribution
    // RUNPATH. Make their local directories visible before that host path.
    if (!libraries.includes(directory) && entries.some(entry => /\.so(?:\.|$)/.test(entry.name))) {
      libraries.push(directory);
    }
    for (const entry of entries) if (entry.isDirectory()) visit(path.join(directory, entry.name));
  }
  for (const directory of [...libraries]) visit(directory);
  return libraries;
}

export function webkitRuntimePaths(sdk) {
  const candidates = ['usr/lib/x86_64-linux-gnu', 'usr/lib64', 'usr/libexec', 'usr/lib']
    .map(dir => path.join(sdk, dir, 'webkit2gtk-4.1'));
  const directory = candidates.find(dir => {
    try {
      for (const name of ['WebKitNetworkProcess', 'WebKitWebProcess']) {
        accessSync(path.join(dir, name), constants.X_OK);
      }
      return true;
    } catch { return false; }
  });
  const bundle = candidates.map(dir => path.join(dir, 'injected-bundle')).find(dir => {
    try {
      accessSync(path.join(dir, 'libwebkit2gtkinjectedbundle.so'), constants.R_OK);
      return true;
    } catch { return false; }
  });
  if (!directory || !bundle) throw new Error('Processus WebKit ou bundle injecté locaux manquants. Lancez make install.');
  return { directory, bundle };
}

export function webkitRuntimeEnv(repo, base = process.env) {
  const sdk = path.join(repo, '.webkit-sdk');
  const { directory, bundle } = webkitRuntimePaths(sdk);
  const libraries = webkitLibraryPaths(sdk);
  const library = libraries.find(dir => existsSync(path.join(dir, 'libgtk-3.so.0')));
  const pixbuf = library && path.join(library, 'gdk-pixbuf-2.0');
  const pixbufCache = pixbuf && existsSync(pixbuf) && readdirSync(pixbuf)
    .map(version => path.join(pixbuf, version, 'loaders.cache')).find(existsSync);
  const scanner = [path.join(sdk, 'usr/libexec'), ...libraries]
    .map(dir => path.join(dir, 'gstreamer-1.0/gst-plugin-scanner')).find(existsSync);
  // Keep the adapter outside the SDK, which installation replaces atomically.
  const shim = path.join(repo, '.build/libtap-webkit-launch.so');
  try { accessSync(shim, constants.R_OK); }
  catch { throw new Error('Lanceur WebKit local manquant. Lancez make install.'); }
  return {
    ...base,
    TAP_WEBKIT_EXEC_PATH: directory,
    WEBKIT_EXEC_PATH: directory,
    WEBKIT_INJECTED_BUNDLE_PATH: bundle,
    LD_LIBRARY_PATH: libraries.join(':'),
    GSETTINGS_SCHEMA_DIR: path.join(sdk, 'usr/share/glib-2.0/schemas'),
    XDG_DATA_DIRS: path.join(sdk, 'usr/share'),
    FONTCONFIG_PATH: path.join(sdk, 'etc/fonts'),
    FONTCONFIG_FILE: path.join(sdk, 'etc/fonts/tap-fonts.conf'),
    GI_TYPELIB_PATH: libraries.map(dir => path.join(dir, 'girepository-1.0')).join(':'),
    ...(library ? {
      GIO_MODULE_DIR: path.join(library, 'gio/modules'),
      GTK_PATH: path.join(library, 'gtk-3.0'),
      LIBGL_DRIVERS_PATH: path.join(library, 'dri'),
      GST_PLUGIN_SYSTEM_PATH_1_0: path.join(library, 'gstreamer-1.0'),
      GST_PLUGIN_PATH_1_0: path.join(library, 'gstreamer-1.0'),
    } : {}),
    ...(pixbufCache ? { GDK_PIXBUF_MODULE_FILE: pixbufCache } : {}),
    ...(scanner ? { GST_PLUGIN_SCANNER_1_0: scanner } : {}),
    LD_PRELOAD: [shim, base.LD_PRELOAD].filter(Boolean).join(':'),
  };
}

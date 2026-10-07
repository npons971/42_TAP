import { accessSync, constants } from 'node:fs';
import path from 'node:path';

export function webkitRuntimeEnv(repo, base = process.env) {
  const sdk = path.join(repo, '.webkit-sdk');
  const candidates = ['usr/lib/x86_64-linux-gnu', 'usr/lib64', 'usr/libexec', 'usr/lib']
    .map(dir => path.join(sdk, dir, 'webkit2gtk-4.1'));
  const directory = candidates.find(dir => {
    try {
      for (const name of ['WebKitNetworkProcess', 'WebKitWebProcess']) {
        accessSync(path.join(dir, name), constants.X_OK);
      }
      accessSync(path.join(dir, 'injected-bundle/libwebkit2gtkinjectedbundle.so'), constants.R_OK);
      return true;
    } catch { return false; }
  });
  // Keep the adapter outside the SDK, which installation replaces atomically.
  const shim = path.join(repo, '.build/libtap-webkit-launch.so');
  try { accessSync(shim, constants.R_OK); }
  catch { throw new Error('Lanceur WebKit local manquant. Lancez make install.'); }
  if (!directory) throw new Error('Processus WebKit locaux manquants. Lancez make install.');
  return {
    ...base,
    TAP_WEBKIT_EXEC_PATH: directory,
    WEBKIT_EXEC_PATH: directory,
    WEBKIT_INJECTED_BUNDLE_PATH: path.join(directory, 'injected-bundle'),
    LD_PRELOAD: [shim, base.LD_PRELOAD].filter(Boolean).join(':'),
  };
}

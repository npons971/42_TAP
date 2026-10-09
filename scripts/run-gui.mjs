import { spawn } from 'node:child_process';
import { openSync, closeSync } from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const options = { host: '127.0.0.1', port: 4242, server: 'auto', world: 'data/world.json', dev: false };
for (let i = 2; i < process.argv.length; i++) {
  const key = process.argv[i].replace(/^--/, '');
  if (key === 'dev') options.dev = true;
  else if (['host', 'port', 'server', 'world'].includes(key) && process.argv[i + 1]) options[key] = process.argv[++i];
  else throw new Error('Unknown or incomplete GUI option: ' + process.argv[i]);
}
options.port = Number(options.port);
if (!Number.isInteger(options.port) || options.port < 1 || options.port > 65535 || !['auto', 'off'].includes(options.server)) {
  throw new Error('GUI_PORT must be between 1 and 65535; GUI_SERVER must be auto or off.');
}
const sdk = path.join(repo, '.webkit-sdk/usr');
const libraries = ['lib64', 'lib/x86_64-linux-gnu', 'lib'].map(dir => path.join(sdk, dir));
const env = { ...process.env,
  PATH: [repo, path.join(sdk, 'bin'), process.env.PATH].filter(Boolean).join(path.delimiter),
  LD_LIBRARY_PATH: [...libraries, process.env.LD_LIBRARY_PATH].filter(Boolean).join(':'),
  GOPATH: path.join(repo, '.go-work'), GOCACHE: path.join(repo, '.go-cache'),
  npm_config_cache: path.join(repo, '.npm-cache'),
  TAP_GUI_HOST: options.host, TAP_GUI_PORT: String(options.port),
};
// Validate the greeting rather than accepting an unrelated service on this port.
function probe() {
  return new Promise(resolve => {
    const socket = net.createConnection({ host: options.host, port: options.port });
    let data = '', settled = false;
    const finish = status => {
      if (settled) return;
      settled = true; socket.destroy(); resolve(status);
    };
    socket.setTimeout(1000, () => finish('occupied'));
    socket.on('error', err => finish(err.code === 'ECONNREFUSED' ? 'absent' : 'occupied'));
    socket.on('end', () => finish('occupied'));
    socket.on('data', chunk => {
      data += chunk.toString('utf8');
      if (data.includes('\n')) finish(data.split('\n')[0] === 'OK hello proto=1' ? 'tap' : 'occupied');
      else if (data.length > 4096) finish('occupied');
    });
  });
}
let server, gui;
let stopping = false;
async function stopChild(child) {
  if (!child || !child.pid) return;
  const signal = name => {
    try {
      if (child === gui) process.kill(-child.pid, name);
      else child.kill(name);
    } catch (err) { if (err.code !== 'ESRCH') throw err; }
  };
  if (child.exitCode !== null || child.signalCode !== null) {
    if (child === gui) signal('SIGKILL');
    return;
  }
  const exited = new Promise(resolve => child.once('exit', resolve));
  signal('SIGTERM');
  const result = await Promise.race([exited.then(() => true), delay(3000).then(() => false)]);
  if (!result) { signal('SIGKILL'); await exited; }
  // Wails/Vite compiler children can outlive the CLI during early startup.
  if (child === gui) signal('SIGKILL');
}
async function cleanup(code) {
  if (stopping) return;
  stopping = true;
  await stopChild(gui);
  await stopChild(server);
  process.exit(code);
}
process.on('SIGINT', () => cleanup(130));
process.on('SIGTERM', () => cleanup(143));
try {
  if (!process.env.DISPLAY && !process.env.WAYLAND_DISPLAY) {
    throw new Error('A graphical session is required. Use make build-gui to compile without opening a window.');
  }
  const local = ['127.0.0.1', 'localhost', '::1'].includes(options.host);
  if (options.server === 'auto' && local) {
    const status = await probe();
    if (status === 'occupied') throw new Error('The selected port is occupied by a service without the TAP greeting. Choose GUI_PORT or use GUI_SERVER=off.');
    if (status === 'tap') console.log('Serveur TAP existant : ' + options.host + ':' + options.port);
    else {
      const log = openSync(path.join(repo, '.build/gui-server.log'), 'a', 0o600);
      try {
        const address = (options.host.includes(':') ? '[' + options.host + ']' : options.host) + ':' + options.port;
        server = spawn(path.join(repo, '.build/tap-server'), ['-addr', address, '-world', options.world], { cwd: repo, env, stdio: ['ignore', log, log] });
      } finally { closeSync(log); }
      let serverError;
      server.on('error', err => { serverError = err; });
      let ready = false;
      for (let attempt = 0; attempt < 40; attempt++) {
        if (serverError || server.exitCode !== null || server.signalCode !== null) break;
        if (await probe() === 'tap') { ready = true; break; }
        await delay(100);
      }
      if (!ready) throw new Error('Local server failed to start. See .build/gui-server.log. ' + (serverError || ''));
      console.log('Serveur local démarré sur ' + options.host + ':' + options.port + ' (journal : .build/gui-server.log).');
    }
  }
  console.log('Ouverture du GUI — choisissez un pseudo puis CONNECT & PLAY.');
  gui = options.dev
    ? spawn(path.join(repo, 'wails'), ['dev', '-tags', 'webkit2_41'], { cwd: repo, env, stdio: 'inherit', detached: true })
    : spawn(path.join(repo, 'Project/client-gui/build/bin/tap-gui'), [], { cwd: repo, env, stdio: 'inherit', detached: true });
  const code = await new Promise((resolve, reject) => {
    gui.once('error', reject);
    gui.once('exit', code => resolve(code ?? 1));
  });
  await cleanup(code);
} catch (err) {
  console.error(String(err.message || err));
  await cleanup(1);
}

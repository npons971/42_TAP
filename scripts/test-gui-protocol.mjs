import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

// Exercise shipped handlers with Wails promises and deterministic timers.
const source = fs.readFileSync(new URL('../Project/client-gui/frontend/src/App.svelte', import.meta.url), 'utf8');
const script = source.match(/<script[^>]*>([\s\S]*?)<\/script>/)[1]
  .replace(/import\s*[\s\S]*?\sfrom\s*['"]svelte['"];?/g, '')
  .replace(/import\s+(?:RoomMap|Icon)\s+from\s*['"][^'"]+['"];?/g, '');
const sent = [];
const timers = new Map();
let nextTimer = 0;
let failMethod = '';
const backend = {};
for (const method of ['Connect', 'Disconnect', 'Move', 'Look', 'Take', 'Drop', 'Talk', 'Attack', 'Inventory', 'Status', 'Quest', 'Quests', 'Who', 'Group', 'Chat', 'SendCommand']) {
  backend[method] = async (...args) => {
    if (failMethod === method) throw new Error(`${method}: simulated network failure`);
    sent.push({method, args});
  };
}
const context = vm.createContext({console,
  setTimeout: callback => { const id = ++nextTimer; timers.set(id, callback); return id; },
  clearTimeout: id => timers.delete(id), onMount: () => {}, onDestroy: () => {}, tick: async () => {},
  window: {go: {main: {App: backend}}},
});
vm.runInContext(script, context);
const run = code => vm.runInContext(code, context);
const settle = async () => { await Promise.resolve(); await Promise.resolve(); };
const methods = () => sent.map(({method}) => method);
const resetCalls = () => { sent.length = 0; };
const flushTimers = async () => {
  const callbacks = [...timers.values()];
  timers.clear();
  for (const callback of callbacks) await callback();
  await settle();
};
const expectRefresh = async (expected, label) => {
  await flushTimers();
  await settle();
  for (const method of expected) assert.ok(methods().includes(method), `${label} should refresh ${method}`);
};

run('connected = true');
const initialRoom = run('currentRoom.id');
run(`handleServerOK('players=3', 'WHO')`);
assert.equal(run('serverPlayersCount'), 3);
assert.equal(run('currentRoom.id'), initialRoom);
run(`handleServerOK('{"room":["alice"],"server":5}', 'WHO')`);
assert.equal(run('serverPlayersCount'), 5);
assert.equal(run('roomPlayersCount'), 1);
assert.equal(run('currentRoom.id'), initialRoom);
run(`handleServerOK('["item.apple"]', 'INVENTORY'); handleServerOK('[]', 'QUESTS')`);
assert.equal(run('inventory[0].id'), 'item.apple');
assert.equal(run('quests.length'), 0);
run(`handleServerOK('[{"quest_id":"quest.herbal_cure","status":"active","progress":"1/1"}]', 'QUESTS'); handleServerOK('[]', 'INVENTORY')`);
assert.equal(run('inventory.length'), 0);
assert.equal(run('quests[0].id'), 'quest.herbal_cure');
assert.equal(run('quests[0].progress'), '1/1');

// STATUS carries an object, but ATTACK must receive its canonical target ID.
run(`handleServerOK('{"hp":80,"max_hp":100,"status":"combat","combat":{"target_id":"npc.bandit_leader","target_name":"Bandit Leader"}}', 'STATUS')`);
assert.equal(run('playerHP'), 80);
assert.equal(run('combatState'), 'EN_COMBAT');
assert.equal(run('currentTarget'), 'npc.bandit_leader');
resetCalls();
await run('callAttack(currentTarget)');
assert.deepEqual(sent.find(({method}) => method === 'Attack').args, ['npc.bandit_leader']);
resetCalls();
run(`handleServerOK('{"attacker_hp":100,"player_hp":100,"status":"respawn","target":"npc.bandit_leader","target_hp":30,"damage":0,"outcome":"respawn","room":"loc.town_square"}', 'ATTACK')`);
await expectRefresh(['Look', 'Status', 'Quests'], 'combat reply');
resetCalls();
run(`handleServerEVT('ROOM COMBAT {"status":"victory","outcome":"victory","target":"npc.bandit_leader","player_hp":80,"target_hp":0}')`);
await expectRefresh(['Look', 'Status', 'Quests'], 'combat event');
resetCalls();
run(`handleServerOK('{"outcome":"victory"}', 'ATTACK'); handleServerEVT('ROOM COMBAT {"outcome":"victory"}')`);
await flushTimers();
for (const method of ['Look', 'Status', 'Quests']) {
  assert.equal(methods().filter(name => name === method).length, 1, 'duplicate refreshes must be coalesced');
}
run(`handleServerEVT('STATS players=2'); handleServerEVT('GROUP INVITE alice')`);
assert.equal(run('serverPlayersCount'), 2);
assert.match(run('messages.group.at(-1).text'), /INVITE.*alice/);
resetCalls();
await run(`callGroup('CREATE', '')`);
await run(`callGroup('JOIN', 'alice')`);
assert.deepEqual(sent.filter(({method}) => method === 'Group').map(({args}) => args), [['CREATE', ''], ['JOIN', 'alice']]);
assert.match(source, /callGroup\(["']CREATE["']/);
assert.match(source, /callGroup\(["']JOIN["']/);
assert.doesNotMatch(source, /callGroup\(["']ACCEPT["']/);
resetCalls();
run(`handleServerOK('Hello traveler!', 'TALK')`);
assert.equal(run('activeDialogue.text'), 'Hello traveler!');
await expectRefresh(['Quests'], 'dialogue');
resetCalls();
await run(`callUse('item.apple')`);
assert.ok(sent.some(({method, args}) => method === 'SendCommand' && args[0] === 'USE item.apple'));
resetCalls();
run(`username = 'alice'; handleServerEVT('ROOM ITEM REWARD alice item.vigor_potion')`);
await expectRefresh(['Inventory', 'Status', 'Quests'], 'own reward');
resetCalls();
run(`handleServerERR('400 INVALID_ARGUMENTS LOOK takes no arguments', 'LOOK', 'LOOK DETAILS'); handleServerERR('400 UNKNOWN_COMMAND Unknown command', 'TALKJSON', 'TALKJSON Village Guard')`);
await settle();
assert.deepEqual(sent.filter(({method}) => method === 'SendCommand').map(({args}) => args[0]), ['LOOK', 'TALK Village Guard']);
run(`connecting = true; handleServerERR('201 NAME_IN_USE Already connected', 'CONNECT')`);
await settle();
assert.equal(run('connecting'), false);
assert.ok(methods().includes('Disconnect'));

// RFC usernames use Unicode lowercase letters/digits, 3–20 codepoints.
for (const username of ['élise', 'abcdefghijklmnopqrst', 'élise_１２', '\u{10428}'.repeat(20)]) {
  resetCalls();
  run(`connected = false; connecting = false; host = ' 127.0.0.1 '; port = 4242; username = ${JSON.stringify(username)}`);
  await run('handleConnect()');
  const call = sent.find(({method}) => method === 'Connect');
  assert.ok(call, `valid RFC username ${username} should connect`);
  assert.equal(call.args[0], '127.0.0.1');
  assert.equal(call.args[2], username);
}
for (const username of ['ab', 'abcdefghijklmnopqrstu', '1alice', 'alice bob', 'ali😀']) {
  resetCalls();
  run(`connected = false; connecting = false; host = '127.0.0.1'; port = 4242; username = ${JSON.stringify(username)}`);
  await run('handleConnect()');
  assert.ok(!methods().includes('Connect'), `invalid username ${username} should be rejected`);
  assert.equal(run('connecting'), false);
  assert.ok(run('connectionError.length') > 0);
}
for (const port of [0, 65536, 4242.5, '4242oops']) {
  resetCalls();
  run(`connected = false; connecting = false; host = '127.0.0.1'; username = 'alice'; port = ${JSON.stringify(port)}`);
  await run('handleConnect()');
  assert.ok(!methods().includes('Connect'), `invalid port ${port} should be rejected`);
}
resetCalls();
run(`connected = false; connecting = false; host = '   '; port = 4242; username = 'alice'`);
await run('handleConnect()');
assert.ok(!methods().includes('Connect'), 'blank host should be rejected');
resetCalls();
run(`handleServerOK('connected', 'CONNECT')`);
const callbacks = [...timers.values()];
timers.clear();
for (const callback of callbacks) await callback();
await expectRefresh(['Look', 'Inventory', 'Status', 'Who', 'Quests'], 'connection');

// Promise failures are visible and preserve chat drafts without broadcasting logs.
failMethod = 'Move';
await run(`callMove('north')`);
assert.match(run('messages.logs.at(-1).text'), /simulated network failure/);
failMethod = 'Chat';
run(`activeTab = 'global'; chatInput = 'Keep this draft'`);
await run('handleSendMessage()');
assert.equal(run('chatInput'), 'Keep this draft');
failMethod = '';
resetCalls();
run(`activeTab = 'logs'; chatInput = 'Never broadcast from logs'`);
await run('handleSendMessage()');
assert.ok(!methods().includes('Chat'));
run(`for (let i = 0; i < 650; i++) addMessage('global', 'alice', String(i))`);
assert.ok(run('messages.global.length') <= 500);
assert.ok(run('messages.logs.length') <= 500);
run(`showGroupModal = true; inventory = [{id:'item.apple'}]; quests = [{id:'quest.herbal_cure'}]; playerHP = 17; currentTarget = 'npc.bandit_leader'; resetSession()`);
assert.equal(run('inventory.length'), 0);
assert.equal(run('quests.length'), 0);
assert.equal(run('currentRoom.id'), '');
assert.equal(run('currentTarget'), null);
assert.equal(run('activeDialogue'), null);
assert.equal(run('showGroupModal'), false);
assert.notEqual(run('playerHP'), 17);
resetCalls();
run('connected = false');
await run(`callMove('north')`);
assert.ok(!methods().includes('Move'), 'disconnected actions must not reach backend');

// Hotkeys must stay out of chat, dialogs, form controls and expanded maps.
run('connected = true');
for (const [key, method] of [['1', 'Look'], ['2', 'Status'], ['3', 'Quests'], ['4', 'Who']]) {
  resetCalls();
  run(`shortcutPrevented = false; handleGameShortcut({key:${JSON.stringify(key)}, target:{closest:()=>null}, preventDefault:()=>shortcutPrevented=true})`);
  await settle();
  assert.deepEqual(methods(), [method]);
  assert.equal(run('shortcutPrevented'), true);
}
for (const guard of ['event.repeat = true', 'event.isComposing = true', 'event.ctrlKey = true', 'event.metaKey = true', 'event.altKey = true', 'event.shiftKey = true', 'event.target.closest = () => ({})', 'activeDialogue = {}', 'showGroupModal = true', 'mapExpanded = true', 'connected = false']) {
  resetCalls();
  run(`activeDialogue = null; showGroupModal = false; mapExpanded = false; connected = true; event = {key:'1',target:{closest:()=>null},preventDefault:()=>{}}; ${guard}; handleGameShortcut(event)`);
  await settle();
  assert.equal(sent.length, 0, `hotkey is ignored when ${guard}`);
}
run(`activeDialogue = null; showGroupModal = false; mapExpanded = false; connected = true; handleGameShortcut({key:'5',target:{closest:()=>null},preventDefault:()=>{}})`);
assert.equal(run('showGroupModal'), true);
run(`selectMapEntity('item', 'item.fountain')`);
assert.equal(run('nearbyTab'), 'item');
assert.equal(run('selectedEntity'), 'item.fountain');
run(`handleServerOK('{"room":{"id":"loc.other","name":"Other","exits":{}},"items":[],"npcs":[],"players":[]}', 'LOOK')`);
assert.equal(run('selectedEntity'), '', 'room changes clear old marker selection');
run(`resetSession()`);
assert.equal(run('mapExpanded'), false);
assert.equal(run('nearbyTab'), 'npc');
assert.equal(run('Object.keys(roomNames).length'), 0, 'discovered map labels do not leak into a new server session');

console.log('GUI protocol: fallback, combat, groups, quests, Unicode validation, connection setup, network errors, logs, hotkeys, map selection and session reset passed.');

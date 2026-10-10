import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

// Exercise shipped handlers with Wails promises and deterministic timers.
const source = fs.readFileSync(new URL('../Project/client-gui/frontend/src/App.svelte', import.meta.url), 'utf8');
const script = source.match(/<script[^>]*>([\s\S]*?)<\/script>/)[1]
  .replace(/import\s*[\s\S]*?\sfrom\s*['"]svelte['"];?/g, '')
  .replace(/import\s+\w+\s+from\s*['"][^'"]+\.svelte['"];?/g, '');
const sent = [];
const timers = new Map();
let nextTimer = 0;
let failMethod = '';
let mountCallback;
const eventHandlers = new Map();
const backend = {};
for (const method of ['Connect', 'Disconnect', 'Move', 'Look', 'Take', 'Drop', 'Talk', 'Attack', 'Inventory', 'Status', 'Quest', 'Quests', 'Who', 'Group', 'Chat', 'SendCommand']) {
  backend[method] = async (...args) => {
    if (failMethod === method) throw new Error(`${method}: simulated network failure`);
    sent.push({method, args});
  };
}
const context = vm.createContext({console,
  setTimeout: callback => { const id = ++nextTimer; timers.set(id, callback); return id; },
  clearTimeout: id => timers.delete(id), onMount: callback => { mountCallback = callback; }, onDestroy: () => {}, tick: async () => {},
  window: {go: {main: {App: backend}}, runtime: {EventsOn: (name, callback) => { eventHandlers.set(name, callback); return () => eventHandlers.delete(name); }}},
});
vm.runInContext(script, context);
mountCallback();
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
assert.equal(run('quests[0].title'), 'Herbal Cure', 'legacy quest identifiers become readable names');
assert.equal(run('questStatusLabel(quests[0])'), 'Return to giver');

// Metadata determines the inventory action; legacy/unknown items never guess USE.
const itemMetadata = [
  {id:'item.healing_salve', name:'Healing Salve', description:'Restores 25 HP.', type:'consumable', heal_value:25, usage:'Use outside combat.'},
  {id:'item.rusty_sword', name:'Rusty Sword', type:'weapon', damage_bonus:10, usage:'Strongest attack bonus applies automatically.'},
  {id:'item.captains_shield', name:"Captain's Shield", type:'armor', defense_bonus:8, usage:'Strongest defense bonus applies automatically.'},
  {id:'item.rare_herbs', name:'Rare Herbs', type:'quest', usage:'Deliver to Village Herbalist.'}
];
run(`handleServerOK(${JSON.stringify(JSON.stringify(itemMetadata))}, 'INVENTORY')`);
assert.equal(run('canUseItem(inventory[0])'), true);
assert.equal(run('inventory[0].heal_value'), 25);
for (let i = 1; i < itemMetadata.length; i++) assert.equal(run(`canUseItem(inventory[${i}])`), false);
assert.equal(run('inventory[1].damage_bonus'), 10);
assert.equal(run('inventory[2].defense_bonus'), 8);
assert.equal(run(`canUseItem(normalizeItems(['item.apple'])[0])`), false, 'unknown legacy metadata must offer INFO');
assert.equal(run(`canUseItem({type:'consumable',heal_value:0})`), false);
assert.match(source, /\{#if canUseItem\(item\)\}/);
assert.match(source, /callInfo\(item.id\)/);
resetCalls();
await run(`callInfo('item.rusty_sword')`);
assert.deepEqual(sent.find(({method}) => method === 'SendCommand').args, ['INFO item.rusty_sword']);
run(`mapExpanded = true; handleServerOK(${JSON.stringify(JSON.stringify(itemMetadata[1]))}, 'INFO')`);
assert.equal(run('activeItemInfo.name'), 'Rusty Sword');
assert.equal(run('activeItemInfo.damage_bonus'), 10);
assert.equal(run('activeItemInfo.usage'), itemMetadata[1].usage);
assert.equal(run('mapExpanded'), false);
assert.equal(run('inventory.length'), 4, 'INFO must not alter inventory');
run(`handleServerOK('Hello traveler!', 'TALK')`);
assert.equal(run('activeItemInfo'), null, 'a delayed dialogue reply must replace item information');
assert.equal(run('activeDialogue.text'), 'Hello traveler!');
run(`handleServerOK(${JSON.stringify(JSON.stringify(itemMetadata[1]))}, 'INFO')`);
assert.equal(run('activeDialogue'), null, 'a delayed item reply must replace the dialogue');
run(`handleServerOK('{"quest_id":"quest.bandit_bounty","description":"Report to the captain."}', 'QUEST')`);
assert.equal(run('activeItemInfo'), null, 'quest acceptance must replace item information too');
run('activeDialogue = null');
run(`activeItemInfo = null; combatState = 'EN_COMBAT'`);
await run(`callInfo('item.captains_shield')`);
assert.deepEqual(sent.at(-1).args, ['INFO item.captains_shield'], 'INFO remains available in combat');
resetCalls();
run(`handleServerERR('400 UNKNOWN_COMMAND Unknown command', 'INFO', 'INFO item.rusty_sword')`);
assert.equal(run('activeItemInfo.damage_bonus'), 10, 'older server INFO rejection may use already supplied metadata');
assert.equal(sent.length, 0, 'INFO fallback never sends USE or retries');
run(`handleServerOK('["item.legacy"]', 'INVENTORY'); handleServerERR('400 UNKNOWN_COMMAND Unknown command', 'INFO', 'INFO item.legacy')`);
assert.equal(run('activeItemInfo.id'), 'item.legacy');
assert.match(run('activeItemInfo.usage'), /does not provide detailed/);
run(`activeItemInfo = null; combatState = 'HORS_COMBAT'`);

// Detailed journals explain both the objective and the return to the giver.
const journal = [
  {id:'quest.bandit_bounty', title:'Bounty on the Cutthroat', description:'Defeat the Bandit Leader, then report back.', status:'NOT_STARTED', giver:'npc.guard_captain', giver_name:'Guard Captain', giver_location_name:'Ruins Gate', reward:"Captain's Shield", steps:[
    {type:'DEFEAT', label:'Defeat Bandit Leader', location_name:"Cutthroat's Den", completed:false},
    {type:'REPORT', label:'Speak to Guard Captain', location_name:'Ruins Gate', completed:false}
  ]}
];
run(`handleServerOK(${JSON.stringify(JSON.stringify(journal))}, 'QUESTS')`);
assert.equal(run('quests[0].state'), 'available');
assert.equal(run('questNextAction(quests[0])'), 'Speak to Guard Captain at Ruins Gate to accept this quest.');
run(`currentRoom.npcs = [{id:'npc.guard_captain'}]`);
assert.equal(run('canTalkToQuestGiver(quests[0])'), true);
run(`quests[0].state = 'active'`);
assert.match(run('questNextAction(quests[0])'), /Defeat Bandit Leader.*Cutthroat/);
assert.equal(run('canTalkToQuestGiver(quests[0])'), false);
run(`quests[0].state = 'ready'; quests[0].steps[0].completed = true`);
assert.equal(run('questNextAction(quests[0])'), 'Speak to Guard Captain at Ruins Gate.');
assert.equal(run('canTalkToQuestGiver(quests[0])'), true);
run(`currentRoom.npcs = []`);
assert.equal(run('canTalkToQuestGiver(quests[0])'), false);
run(`quests[0].state = 'unavailable'`);
assert.match(run('questNextAction(quests[0])'), /Another player/);
assert.equal(run('canTalkToQuestGiver(quests[0])'), false);
run(`quests[0].state = 'completed'`);
assert.match(run('questNextAction(quests[0])'), /Reward received/);
assert.equal(run('canTalkToQuestGiver(quests[0])'), false);
run(`quests[0].state = 'active'; quests[0].steps[0].completed = false; quests[0].steps[0].hint = 'Another player has defeated this enemy.'`);
assert.equal(run('questNextAction(quests[0])'), 'Another player has defeated this enemy.');
run(`quests[0].state = 'available'`);
assert.equal(run('questNextAction(quests[0])'), 'Another player has defeated this enemy.', 'blocked objectives must be explained before acceptance too');
run(`handleServerOK('{"quest_id":"quest.bandit_bounty","description":"Defeat the leader and report back."}', 'QUEST')`);
assert.equal(run('activeDialogue.npc'), 'Bounty on the Cutthroat');
assert.equal(run('activeDialogue.text'), 'Defeat the leader and report back.');
run('activeDialogue = null');

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
run(`handleServerOK('group=group.1', 'GROUP', 'GROUP CREATE')`);
await run(`callGroup('JOIN', ' AlIcE ')`);
run(`handleServerOK('group=group.2', 'GROUP', 'GROUP JOIN alice')`);
await run(`callGroup('INVITE', ' BOB ')`);
run(`handleServerOK('', 'GROUP', 'GROUP INVITE bob')`);
await run(`callGroup('JOIN', ' ÉLISE ')`);
run(`handleServerOK('group=group.3', 'GROUP', 'GROUP JOIN élise')`);
assert.deepEqual(sent.filter(({method}) => method === 'Group').map(({args}) => args), [['CREATE', ''], ['JOIN', 'alice'], ['INVITE', 'bob'], ['JOIN', 'élise']]);
assert.match(source, /callGroup\(["']CREATE["']/);
assert.match(source, /callGroup\(["']JOIN["']/);
assert.doesNotMatch(source, /callGroup\(["']ACCEPT["']/);
resetCalls();
run(`mapExpanded = true; handleServerOK('Hello traveler!', 'TALK')`);
assert.equal(run('activeDialogue.text'), 'Hello traveler!');
assert.equal(run('mapExpanded'), false, 'a delayed TALK reply releases the expanded map focus trap');
run(`mapExpanded = true; handleServerOK('{"npc":"Guard","dialogue":["Stay safe."]}', 'TALKJSON')`);
assert.equal(run('activeDialogue.text'), 'Stay safe.');
assert.equal(run('mapExpanded'), false, 'a delayed TALKJSON reply releases the expanded map focus trap');
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
resetCalls();
run(`handleServerERR('400 INVALID_ARGUMENTS QUESTS takes no arguments', 'QUESTS', 'QUESTS DETAILS')`);
await settle();
assert.deepEqual(sent.filter(({method}) => method === 'SendCommand').map(({args}) => args[0]), ['QUESTS']);
resetCalls();
run(`handleServerERR('400 INVALID_ARGUMENTS QUESTS takes no arguments', 'QUESTS', 'QUESTS')`);
await settle();
assert.equal(sent.length, 0, 'legacy fallback must not retry recursively');
resetCalls();
run(`handleServerEVT('ROOM ITEM REWARD bob item.captains_shield')`);
await expectRefresh(['Look', 'Quests'], 'another player claiming a shared quest reward');

// Errors follow the failed request, including replies received after switching tabs.
for (const scope of ['GLOBAL', 'ROOM', 'GROUP']) {
  const payload = scope === 'GROUP' ? '401 NOT_IN_GROUP CHAT GROUP requires group membership; use GROUP CREATE or GROUP JOIN' : '400 INVALID_ARGUMENTS CHAT message cannot be empty';
  run(`messages = {global: [], room: [], group: [], logs: []}; activeTab = ${JSON.stringify(scope === 'ROOM' ? 'global' : 'room')}; handleServerERR(${JSON.stringify(payload)}, 'CHAT', ${JSON.stringify(`CHAT ${scope} Hello`)})`);
  for (const channel of ['global', 'room', 'group']) {
    assert.equal(run(`messages.${channel}.length`), channel === scope.toLowerCase() ? 1 : 0, `CHAT ${scope} error should only appear in ${scope}`);
  }
  assert.equal(run(`messages.${scope.toLowerCase()}[0].sender`), 'SERVER');
  assert.equal(run('actionError'), payload);
  assert.ok(run('messages.logs.some(msg => msg.text === "S: ERR " + actionError)'));
}
run(`messages = {global: [], room: [], group: [], logs: []}; handleServerERR('401 NOT_IN_GROUP No group to leave', 'GROUP', 'GROUP LEAVE')`);
assert.equal(run('messages.group.length'), 1);
assert.equal(run('messages.room.length'), 0);
for (const [command, request] of [['MOVE', 'MOVE north'], ['LOOK', 'LOOK'], ['CHAT', 'CHAT UNKNOWN Hello'], ['CHAT', ''], ['', '']]) {
  run(`messages = {global: [], room: [], group: [], logs: []}; handleServerERR('400 INVALID_ARGUMENTS Rejected command', ${JSON.stringify(command)}, ${JSON.stringify(request)})`);
  assert.equal(run('messages.global.length + messages.room.length + messages.group.length'), 0, `${request || 'unscoped reply'} should stay out of chat`);
  assert.equal(run('messages.logs.length'), 1);
}
run(`connecting = true; handleServerERR('201 NAME_IN_USE Already connected', 'CONNECT')`);
await settle();
assert.equal(run('connecting'), false);
assert.ok(methods().includes('Disconnect'));

// Project usernames use Unicode lowercase letters/digits, 3–20 codepoints.
// The login form normalizes uppercase input before validation and sending.
for (const username of ['élise', 'abcdefghijklmnopqrst', 'élise_１２', '\u{10428}'.repeat(20), 'ALICE', ' Alice_42 ', 'ÉLISE', '\u{10400}'.repeat(20)]) {
  resetCalls();
  run(`connected = false; connecting = false; host = ' 127.0.0.1 '; port = 4242; username = ${JSON.stringify(username)}`);
  await run('handleConnect()');
  const call = sent.find(({method}) => method === 'Connect');
  assert.ok(call, `valid username ${username} should connect`);
  assert.equal(call.args[0], '127.0.0.1');
  assert.equal(call.args[2], username.trim().toLowerCase());
  assert.equal(run('username'), call.args[2], 'displayed name should match the sent name');
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
run(`showGroupModal = true; activeItemInfo = {id:'item.rusty_sword'}; inventory = [{id:'item.apple'}]; quests = [{id:'quest.herbal_cure'}]; playerHP = 17; currentTarget = 'npc.bandit_leader'; resetSession()`);
assert.equal(run('inventory.length'), 0);
assert.equal(run('quests.length'), 0);
assert.equal(run('currentRoom.id'), '');
assert.equal(run('currentTarget'), null);
assert.equal(run('activeDialogue'), null);
assert.equal(run('activeItemInfo'), null);
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
for (const guard of ['event.repeat = true', 'event.isComposing = true', 'event.ctrlKey = true', 'event.metaKey = true', 'event.altKey = true', 'event.shiftKey = true', 'event.target.closest = () => ({})', 'activeDialogue = {}', 'activeItemInfo = {}', 'showGroupModal = true', 'mapExpanded = true', 'connected = false']) {
  resetCalls();
  run(`activeDialogue = null; activeItemInfo = null; showGroupModal = false; mapExpanded = false; connected = true; event = {key:'1',target:{closest:()=>null},preventDefault:()=>{}}; ${guard}; handleGameShortcut(event)`);
  await settle();
  assert.equal(sent.length, 0, `hotkey is ignored when ${guard}`);
}
run(`activeDialogue = null; activeItemInfo = null; showGroupModal = false; mapExpanded = false; connected = true; handleGameShortcut({key:'5',target:{closest:()=>null},preventDefault:()=>{}})`);
assert.equal(run('showGroupModal'), true);
run(`selectMapEntity('item', 'item.fountain')`);
assert.equal(run('selectedEntity'), 'item.fountain');
run(`handleServerOK('{"room":{"id":"loc.other","name":"Other","exits":{}},"items":[],"npcs":[],"players":[]}', 'LOOK')`);
assert.equal(run('selectedEntity'), '', 'room changes clear old marker selection');
run(`resetSession()`);
assert.equal(run('mapExpanded'), false);
assert.equal(run('Object.keys(roomNames).length'), 0, 'discovered map labels do not leak into a new server session');

const clearMessages = () => run('messages = {global: [], room: [], group: [], logs: []}');
const expectChannel = (expected, label) => {
  for (const channel of ['global', 'room', 'group']) {
    assert.equal(run(`messages.${channel}.length`), channel === expected ? 1 : 0, `${label}: ${channel} routing`);
  }
  assert.equal(run('messages.logs.length'), 1, `${label}: one log entry per server frame`);
};

// Exercise real event subscriptions for every standard event and project extension.
const eventCases = [
  ...['GLOBAL', 'ROOM', 'GROUP'].map(scope => [`${scope} CHAT élise Hello  everyone!`, scope.toLowerCase(), []]),
  ...['INVITE', 'JOIN', 'LEAVE', 'LEADER'].map(action => [`GROUP ${action} alice`, 'group', action === 'INVITE' ? [] : ['SendCommand']]),
  ...['ENTER', 'LEAVE'].map(action => [`ROOM PRESENCE ${action} alice`, 'room', ['Look', 'Who']]),
  ['ROOM COMBAT {"outcome":"victory","target":"npc.bandit_leader"}', 'room', ['Status', 'Look', 'Quests']],
  ...['TAKE', 'DROP', 'USE', 'DELIVER', 'REWARD'].map(action => [`ROOM ITEM ${action} alice item.apple`, 'room', ['Look', 'Inventory', 'Status', 'Quests']]),
  ['ROOM ITEM TAKE bob item.apple', 'room', ['Look', 'Quests']],
  ['STATS players=2', null, []],
];
for (const [payload, channel, refresh] of eventCases) {
  clearMessages(); resetCalls(); timers.clear();
  run(`connected = true; username = 'alice'; activeTab = 'logs'`);
  eventHandlers.get('server_evt')(payload);
  expectChannel(channel, payload);
  assert.equal(run('messages.logs[0].text'), `S: EVT ${payload}`);
  if (payload.includes(' CHAT ')) {
    assert.equal(run(`messages.${channel}[0].sender`), 'élise');
    assert.equal(run(`messages.${channel}[0].text`), 'Hello  everyone!');
  }
  if (payload.startsWith('STATS')) assert.equal(run('serverPlayersCount'), 2);
  await flushTimers();
  assert.deepEqual(methods().sort(), [...refresh].sort(), `${payload}: refresh only affected data`);
}

// Unsupported scopes/events and incomplete data stay in logs, without side effects.
for (const payload of ['GLOBAL PRESENCE ENTER alice', 'GROUP PRESENCE LEAVE alice', 'GROUP COMBAT {}', 'GLOBAL ITEM TAKE alice item.apple', 'UNKNOWN CHAT alice Hello', 'LOGS CHAT alice Hello', 'ROOM INVITE alice', 'ROOM PRESENCE WAVE alice', 'ROOM PRESENCE ENTER', 'ROOM ITEM TAKE alice', 'ROOM ITEM UNKNOWN alice item.apple', 'ROOM COMBAT', 'GROUP JOIN', 'ROOM CHAT alice', 'STATS players=oops', 'STATS players=-1', 'STATS players=2 extra', 'STATS players=9007199254740992', 'UNKNOWN EVENT']) {
  clearMessages(); resetCalls(); timers.clear();
  run('serverPlayersCount = 7');
  eventHandlers.get('server_evt')(payload);
  expectChannel(null, payload);
  assert.equal(run('serverPlayersCount'), 7);
  await flushTimers();
  assert.deepEqual(methods(), [], `${payload}: no unrelated refresh`);
}

// All non-chat command errors belong to the alert and logs; GROUP has its own channel.
const errorCases = [
  ['MOVE north', '301 NO_EXIT No north exit'], ['LOOK', '413 RESPONSE_TOO_LARGE Room reply too large'],
  ['TAKE item.apple', '404 ITEM_NOT_FOUND Missing item'], ['TAKE item.apple', '405 ITEM_NOT_OBTAINABLE Fixed item'],
  ['TAKE item.apple', '409 INVENTORY_FULL Inventory full'], ['DROP item.apple', '404 ITEM_NOT_IN_INVENTORY Missing item'],
  ['INVENTORY', '500 INTERNAL_ERROR Cannot encode inventory'], ['USE item.apple', '405 ITEM_NOT_USABLE Cannot use item'],
  ['USE item.apple', '409 HEALTH_FULL Already healthy'], ['TALK npc.guard', '404 NPC_NOT_FOUND Missing NPC'],
  ['TALKJSON npc.guard', '406 NO_QUEST_AVAILABLE Reward unavailable'], ['ATTACK npc.guard', '405 NPC_NOT_HOSTILE Friendly NPC'],
  ['ATTACK npc.bandit', '409 TARGET_BUSY Busy enemy'], ['DEFEND', '409 NOT_IN_COMBAT No combat'],
  ['FLEE', '409 IN_COMBAT Cannot flee'], ['STATUS', '403 NOT_AUTHENTICATED Login required'],
  ['QUEST npc.guard', '406 NO_QUEST_AVAILABLE No quest'], ['QUESTS', '500 INTERNAL_ERROR Cannot encode quests'],
  ['QUESTINFO quest.unknown', '404 QUEST_NOT_FOUND Missing quest'], ['WHO', '400 INVALID_ARGUMENTS Invalid WHO'],
  ['QUIT extra', '400 INVALID_ARGUMENTS Invalid QUIT'], ['UNKNOWN', '400 UNKNOWN_COMMAND Unknown command'],
  ['WHO', '429 RATE_LIMITED Too many commands'], ['', '503 CONNECTION_LIMIT Too many connections'],
  ['GROUP CREATE', '402 ALREADY_IN_GROUP Already grouped'], ['GROUP INVITE bob', '404 PLAYER_NOT_FOUND Missing player'],
  ['GROUP INVITE bob', '409 TARGET_BUSY Recipient busy'], ['GROUP JOIN bob', '404 GROUP_NOT_FOUND Missing leader'],
  ['GROUP LEAVE', '401 NOT_IN_GROUP No group'],
];
for (const [request, payload] of errorCases) {
  clearMessages();
  const command = request.split(' ')[0];
  eventHandlers.get('server_reply')({kind: 'ERR', command, request, payload});
  expectChannel(command === 'GROUP' ? 'group' : null, request || 'unsolicited error');
  assert.equal(run('actionError'), payload);
}
run(`groupError = ''; groupStatus = 'Previously updated'; showGroupModal = true`);
eventHandlers.get('server_reply')({kind:'ERR', command:'MOVE', request:'MOVE north', payload:'301 NO_EXIT No north exit'});
assert.equal(run('groupError'), '', 'room actions must not populate the group dialog error');
eventHandlers.get('server_reply')({kind:'ERR', command:'GROUP', request:'GROUP JOIN bob', payload:'404 GROUP_NOT_FOUND Missing leader'});
assert.equal(run('groupError'), '404 GROUP_NOT_FOUND Missing leader');
assert.equal(run('groupStatus'), '', 'group errors replace stale success feedback');
for (const [request, payload, channel] of [['CHAT GLOBAL Hello', '', null], ['GROUP LEAVE', '', 'group'], ['GROUP CREATE', 'group=group.1', 'group'], ['CONNECT alice', 'connected', null]]) {
  clearMessages();
  eventHandlers.get('server_reply')({kind: 'OK', command: request.split(' ')[0], request, payload});
  expectChannel(channel, request);
  assert.equal(run('messages.logs[0].text'), `S: OK${payload ? ' ' + payload : ''}`);
  if (request.startsWith('GROUP')) assert.equal(run('groupError'), '', 'successful group actions clear their dialog error');
}
timers.clear();

// JSON replies cannot update a different command's panel based on matching field names.
run(`inventory = [{id:'item.keep'}]; quests = [{id:'quest.keep'}]; playerHP = 77; serverPlayersCount = 7; activeDialogue = null`);
for (const [command, payload] of [['WHO', '["item.apple"]'], ['QUEST', '[{"quest_id":"quest.other"}]'], ['QUESTINFO', '{"npc":"Guard","dialogue":["Hello"],"hp":1,"max_hp":100}'], ['UNKNOWN', '{"room":{"id":"loc.wrong"},"server":0}']]) {
  clearMessages();
  const roomID = run('currentRoom.id');
  eventHandlers.get('server_reply')({kind:'OK', command, payload, request:command});
  expectChannel(null, command);
  assert.equal(run('inventory[0].id'), 'item.keep');
  assert.equal(run('quests[0].id'), 'quest.keep');
  assert.equal(run('playerHP'), 77);
  assert.equal(run('serverPlayersCount'), 7);
  assert.equal(run('activeDialogue'), null);
  assert.equal(run('currentRoom.id'), roomID);
}
timers.clear();

// Background refreshes preserve the alert; local chat/group failures follow their context too.
run(`connected = true; handleServerERR('301 NO_EXIT No north exit', 'MOVE', 'MOVE north'); scheduleRefresh('Look', 'Status')`);
await flushTimers();
assert.equal(run('actionError'), '301 NO_EXIT No north exit');
for (const [method, args, channel] of [['Chat', ['GROUP', 'Hello'], 'group'], ['Group', ['JOIN', 'alice'], 'group'], ['SendCommand', ['CHAT ROOM Hello'], 'room'], ['Move', ['north'], null]]) {
  clearMessages(); failMethod = method;
  await run(`invoke(${JSON.stringify(method)}, ...${JSON.stringify(args)})`);
  expectChannel(channel, `${method} network failure`);
  assert.match(run('actionError'), /simulated network failure/);
}
failMethod = '';
clearMessages();
run(`callGroup('JOIN', '   ')`);
expectChannel('group', 'group input validation');
assert.equal(run('messages.group[0].sender'), 'CLIENT');
assert.equal(run('groupError'), run('actionError'));
resetCalls();
run(`activeTab = 'unknown'; chatInput = 'Hello'`);
await run('handleSendMessage()');
assert.deepEqual(methods(), [], 'only documented chat scopes can be sent');
clearMessages();
eventHandlers.get('server_error')('Invalid server frame');
expectChannel(null, 'network event');
assert.equal(run('actionError'), 'Invalid server frame');
for (const payload of ['201 NAME_IN_USE Already connected', '400 INVALID_ARGUMENTS Invalid name', '900 CONNECTION_FAILED Cannot connect']) {
  clearMessages(); run('connecting = true; connected = false');
  eventHandlers.get('server_reply')({kind:'ERR', command:'CONNECT', request:'CONNECT alice', payload});
  expectChannel(null, 'CONNECT failure');
  assert.equal(run('connectionError'), payload);
  assert.equal(run('connecting'), false);
}
await settle();
eventHandlers.get('disconnected')('Connection closed by server');
assert.equal(run('connected'), false);
assert.equal(run('connectionError'), 'Connection closed by server');
expectChannel(null, 'disconnect');

// Group invitations and membership use confirmed replies, never command dispatch alone.
run(`resetSession(); connected = true; username = 'bob'; handleServerEVT('GROUP INVITE alice'); handleServerEVT('GROUP INVITE alice')`);
assert.equal(run('groupInvitations.length'), 1, 'duplicate invitations are coalesced');
assert.equal(run('showGroupModal'), false, 'invitations do not interrupt play');
resetCalls();
await run(`callGroup('JOIN', groupInvitations[0])`);
assert.equal(run('groupId'), '', 'sending a join does not confirm membership');
assert.equal(run('groupPending'), true);
await run(`callGroup('CREATE')`);
assert.equal(sent.filter(({method}) => method === 'Group').length, 1, 'pending actions block double submission');
eventHandlers.get('server_reply')({kind:'OK', command:'GROUP', request:'GROUP JOIN alice', payload:'group=group.1'});
assert.equal(run('groupId'), 'group.1');
assert.equal(run('groupInvitations.length'), 0);
assert.equal(run('groupPending'), false);
assert.match(run('groupStatus'), /joined alice/);
assert.ok(sent.some(({method, args}) => method === 'SendCommand' && args[0] === 'GROUP STATUS'));
run(`handleServerOK('{"id":"group.1","leader":"alice","members":["alice","bob"]}', 'GROUP', 'GROUP STATUS')`);
assert.equal(run('groupMembers.join(",")'), 'alice,bob');
assert.match(run('groupStatus'), /joined alice/, 'status refresh preserves action feedback');
resetCalls();
run(`handleServerEVT('GROUP LEADER bob')`);
assert.ok(sent.some(({args}) => args[0] === 'GROUP STATUS'), 'leader changes fetch the authoritative roster');
run(`handleServerOK('{"id":"group.1","leader":"bob","members":["bob"]}', 'GROUP', 'GROUP STATUS')`);
assert.equal(run('groupLeader'), 'bob');
await run(`callGroup('INVITE', 'alice')`);
run(`handleServerERR('404 PLAYER_NOT_FOUND Offline', 'GROUP', 'GROUP INVITE alice')`);
assert.equal(run('groupPending'), false, 'errors allow retry');
assert.equal(run('groupId'), 'group.1', 'failed invitations preserve membership');
run(`handleServerOK('', 'GROUP', 'GROUP LEAVE')`);
assert.equal(run('groupId'), '');
assert.equal(run('groupMembers.length'), 0);
run(`handleServerEVT('GROUP INVITE alice'); resetSession()`);
assert.equal(run('groupInvitations.length'), 0, 'disconnect clears session invitations');
run(`connected = true; handleServerEVT('GROUP INVITE alice')`);
await run(`callGroup('JOIN', 'alice')`);
run(`handleServerERR('404 GROUP_NOT_FOUND Missing leader', 'GROUP', 'GROUP JOIN alice')`);
assert.equal(run('groupInvitations.length'), 0, 'unavailable groups remove stale invitations');
resetCalls();
await run(`callGroup('INVITE', '𐐨'.repeat(20))`);
assert.equal(sent.find(({method}) => method === 'Group').args[1], '𐐨'.repeat(20), 'group usernames use Unicode character limits');
run(`handleServerOK('', 'GROUP', 'GROUP INVITE ${'𐐨'.repeat(20)}')`);
resetCalls();
await run(`callGroup('INVITE', 'alice bob')`);
assert.equal(sent.filter(({method}) => method === 'Group').length, 0, 'invalid usernames stay out of protocol commands');
assert.match(source, /<span>GROUP/);
assert.doesNotMatch(source, /Create party|Join party|Leave party|<h3>Party/);

console.log('GUI protocol: replies, all standard/extension events, error routing, refreshes, fallback, combat, groups, quests, Unicode validation, connection setup, network errors, logs, hotkeys, map selection and session reset passed.');

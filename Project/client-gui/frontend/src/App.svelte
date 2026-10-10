<script>
  import { onMount, tick } from 'svelte';
  import RoomMap from './RoomMap.svelte';
  import Icon from './Icon.svelte';
  import Nearby from './Nearby.svelte';
  import PixelSprite from './PixelSprite.svelte';
  import ItemInfo from './ItemInfo.svelte';

  // Connection State
  let connected = false;
  let connecting = false;
  let connectionError = "";
  let host = "127.0.0.1";
  let port = 4242;
  let username = "";

  // Player & Game State
  let playerHP = 100;
  let playerMaxHP = 100;
  let combatState = "HORS_COMBAT";
  let currentTarget = null;
  let roomPlayersCount = 0;
  let serverPlayersCount = 0;

  // Current Room State
  function emptyRoom() {
    return { id: "", name: "Loading room…", description: "", exits: {}, items: [], npcs: [], players: [] };
  }
  let currentRoom = emptyRoom();

  // Room names mapping for intuitive navigation
  let roomNames = {};
  let selectedEntity = '';
  let mapExpanded = false;

  // Inventory & Quests
  let inventory = [];
  let quests = [];

  // Group Management & Modal
  let showGroupModal = false;
  let groupTarget = "";
  let groupLeaderTarget = "";
  let groupId = "";
  let groupLeader = "";
  let groupMembers = [];
  let groupInvitations = [];
  let groupPending = false;

  // NPC Dialogue Modal
  let activeDialogue = null; // { npc: "...", text: "..." }
  let activeItemInfo = null;

  // Chat & Logs Tabs (Mandatory Subject requirement V.4)
  let activeTab = "global"; // "global" | "room" | "group" | "logs"
  let chatInput = "";
  let chatViewport = null;
  let messages = {
    global: [],
    room: [],
    group: [],
    logs: []
  };
  const MESSAGE_LIMIT = 500;
  const CHAT_CHANNELS = ["global", "room", "group"];
  let actionError = "";
  let groupError = "";
  let groupStatus = "";
  let refreshTimer = null;
  const pendingRefresh = new Set();

  // An action's reply and room event often request the same data.
  function scheduleRefresh(...methods) {
    if (!connected) return;
    methods.forEach(method => pendingRefresh.add(method));
    if (refreshTimer !== null) return;
    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      const methods = [...pendingRefresh];
      pendingRefresh.clear();
      methods.forEach(method => invokeBackend(method, [], false));
    }, 50);
  }

  // Auto-scroll chat view to bottom
  async function scrollToBottom() {
    await tick();
    if (chatViewport) chatViewport.scrollTop = chatViewport.scrollHeight;
  }

  // Helper for adding messages
  function addMessage(channel, sender, text, type = "normal", mirrorToLogs = true) {
    if (channel !== "logs" && !CHAT_CHANNELS.includes(channel)) return;
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
    const msg = { time, sender, text, type };
    if (messages[channel]) {
      messages[channel] = [...messages[channel], msg].slice(-MESSAGE_LIMIT);
    }
    // Also push system events or chat into logs
    if (mirrorToLogs && channel !== 'logs') {
      messages.logs = [...messages.logs, { time, sender: `[${channel.toUpperCase()}] ${sender}`, text, type }].slice(-MESSAGE_LIMIT);
    }
    scrollToBottom();
  }

  function addLog(text, type = "info") {
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
    messages.logs = [...messages.logs, { time, sender: "SYSTEM", text, type }].slice(-MESSAGE_LIMIT);
    scrollToBottom();
  }

  // Wails Bindings Safe Access
  function getBackend() {
    return window?.go?.main?.App;
  }

  function resetSession() {
    connected = false;
    connecting = false;
    currentRoom = emptyRoom();
    roomNames = {};
    selectedEntity = '';
    mapExpanded = false;
    inventory = [];
    quests = [];
    playerHP = 100;
    playerMaxHP = 100;
    combatState = "HORS_COMBAT";
    currentTarget = null;
    roomPlayersCount = 0;
    serverPlayersCount = 0;
    activeDialogue = null;
    activeItemInfo = null;
    showGroupModal = false;
    groupTarget = "";
    groupLeaderTarget = "";
    groupId = "";
    groupLeader = "";
    groupMembers = [];
    groupInvitations = [];
    groupPending = false;
    groupStatus = "";
    groupError = "";
    clearTimeout(refreshTimer);
    refreshTimer = null;
    pendingRefresh.clear();
    chatInput = "";
    actionError = "";
    messages = { global: [], room: [], group: [], logs: [] };
  }

  onMount(() => {
    addLog("Ready to connect to a 42 TAP server.", "info");
    getBackend()?.ConnectionDefaults?.().then(defaults => {
      if (!connecting && !connected) { host = defaults.host; port = defaults.port; }
    }).catch(err => addLog(String(err), "error"));

    if (window.runtime) {
      const unsubscribe = [window.runtime.EventsOn("server_reply", ({kind, command, payload, request}) => {
        if (kind === "OK") handleServerOK(payload, command, request); else handleServerERR(payload, command, request);
      }),
      window.runtime.EventsOn("server_evt", (payload) => handleServerEVT(payload)),
      window.runtime.EventsOn("disconnected", (msg) => {
        const wasConnecting = connecting;
        resetSession();
        if (wasConnecting && !connectionError) connectionError = msg || "Connection closed before login.";
        else if (msg !== "Client disconnected") connectionError = msg || "Connection lost. Connect again to resume.";
        addLog(msg || "Disconnected from server.", "info");
      }),
      window.runtime.EventsOn("server_error", (err) => {
        actionError = String(err);
        addLog(`Network error: ${err}`, "error");
      })];
      return () => unsubscribe.forEach(off => off());
    }
  });

  // Server Payload Parsing
  function handleServerOK(payload, command = "", request = "") {
    addLog(`S: OK${payload ? " " + payload : ""}`, "ok");
    if (command === "GROUP") {
      if (payload.startsWith("{")) {
        try {
          const data = JSON.parse(payload);
          if (typeof data.id === "string" && typeof data.leader === "string" && Array.isArray(data.members)) {
            groupId = data.id;
            groupLeader = data.leader;
            groupMembers = data.members;
          }
        } catch (error) { addLog("Could not read group details.", "error"); }
        return;
      }
      groupPending = false;
      groupError = "";
      const [, action, target] = request.trim().split(/\s+/);
      if (action === "CREATE" || action === "JOIN") {
        groupId = payload.replace(/^group=/, "");
        groupLeader = action === "CREATE" ? username : target;
        groupInvitations = [];
      } else if (action === "LEAVE") {
        groupId = ""; groupLeader = ""; groupMembers = [];
      }
      groupStatus = action === "CREATE" ? "Group created. Invite a player below to adventure together."
        : action === "INVITE" ? `Invitation sent to ${target}. Waiting for them to join.`
        : action === "JOIN" ? `You joined ${target}’s group.`
        : action === "LEAVE" ? "You left the group." : "Group updated.";
      addMessage("group", "GROUP", groupStatus, "system", false);
      refreshGroup();
      return;
    }
    if (!payload) return;

    if (command === "CONNECT" && payload === "connected") {
      connected = true;
      connecting = false;
      connectionError = "";
      scheduleRefresh("Look", "Inventory", "Status", "Who", "Quests");
      return;
    }

    if (["ATTACK", "DEFEND", "FLEE"].includes(command)) {
      scheduleRefresh("Status", "Look", "Quests");
      return;
    }
    if (["QUEST", "TALK", "TALKJSON"].includes(command)) scheduleRefresh("Quests");

    // Try parsing JSON payloads (LOOK, STATUS, INVENTORY, WHO, QUESTS)
    if (payload.startsWith("{") || payload.startsWith("[")) {
      try {
        const data = JSON.parse(payload);

        // LOOK payload
        if (command === "LOOK" && data.room && !Array.isArray(data.room) && data.room.id) {
          if (currentRoom.id !== data.room.id) {
            selectedEntity = '';
          }
          currentRoom = {
            id: data.room.id || "unknown",
            name: data.room.name || "Unknown Room",
            description: data.room.description || "",
            exits: data.room.exits || {},
            items: normalizeItems(data.items || []),
            npcs: normalizeNPCs(data.npcs || []),
            players: data.players || []
          };
          roomNames = { ...roomNames, [currentRoom.id]: currentRoom.name };
          roomPlayersCount = currentRoom.players.length;
          return;
        }

        // STATUS payload
        if (command === "INFO" && data.id && data.name) {
          showItemInfo(data);
          return;
        }
        if (command === "STATUS" && data.hp !== undefined && data.max_hp !== undefined) {
          playerHP = data.hp;
          playerMaxHP = data.max_hp || 100;
          combatState = data.state || (data.status === "combat" ? "EN_COMBAT" : "HORS_COMBAT");
          currentTarget = typeof data.combat === "string" ? data.combat : data.combat?.target_id || null;
          return;
        }

        // WHO payload
        if (command === "WHO" && data.room !== undefined && data.server !== undefined) {
          roomPlayersCount = Array.isArray(data.room) ? data.room.length : data.room;
          serverPlayersCount = data.server;
          return;
        }

        // INVENTORY payload (Array)
        if (Array.isArray(data) && ["INVENTORY", "QUESTS"].includes(command)) {
          if (command === "QUESTS") {
            quests = data.map(normalizeQuest);
          } else {
            inventory = normalizeItems(data);
          }
          return;
        }

        // TALK payload
        if (command === "QUEST" && data.quest_id && data.description) {
          const quest = quests.find(q => q.id === data.quest_id);
          showDialogue(quest?.title || displayQuestName(data.quest_id), data.description);
          return;
        }
        if (["TALK", "TALKJSON"].includes(command) && data.npc && data.dialogue) {
          const text = Array.isArray(data.dialogue) ? data.dialogue.join("\n") : String(data.dialogue);
          showDialogue(data.npc, text);
          addMessage("room", data.npc, text, "dialogue", false);
          return;
        }
      } catch (e) {
        console.warn("Could not parse JSON OK payload:", e);
      }
    }

    // Key-value OK responses: OK room=loc.market, OK taken=item.apple, etc.
    if (command === "TALK") {
      showDialogue('NPC', payload);
      addMessage("room", "NPC", payload, "dialogue", false);
    } else if (command === "WHO" && /^players=\d+$/.test(payload)) {
      const count = Number(payload.slice(8));
      if (Number.isSafeInteger(count)) serverPlayersCount = count;
    } else if (command === "MOVE" && payload.startsWith("room=")) {
      // Room changed, refresh look
      scheduleRefresh("Look");
    } else if ((command === "TAKE" && payload.startsWith("taken=")) || (command === "DROP" && payload.startsWith("dropped="))) {
      // Item action confirmed, refresh room & inventory
      scheduleRefresh("Look", "Inventory", "Quests");
    } else if (command === "USE" && payload.startsWith("used=")) {
      scheduleRefresh("Inventory", "Status", "Quests");
    }
  }

  function normalizeItems(rawItems) {
    return rawItems.map(item => {
      if (typeof item === 'string') {
        const cleanName = item.replace(/^item\./, '').replace(/_/g, ' ');
        return { id: item, name: cleanName, obtainable: true };
      }
      return { id: item.id, name: item.name || item.id, description: item.description || '', obtainable: item.obtainable !== false,
        type: item.type || '', heal_value: item.heal_value || 0, damage_bonus: item.damage_bonus || 0, defense_bonus: item.defense_bonus || 0, usage: item.usage || '' };
    });
  }

  function canUseItem(item) {
    return item.type === 'consumable' && item.heal_value > 0;
  }

  function showItemInfo(item) {
    mapExpanded = false;
    activeDialogue = null;
    showGroupModal = false;
    activeItemInfo = normalizeItems([item])[0];
  }

  function showDialogue(npc, text) {
    mapExpanded = false;
    activeItemInfo = null;
    showGroupModal = false;
    activeDialogue = {npc, text};
  }

  function normalizeNPCs(rawNPCs) {
    return rawNPCs.map(npc => {
      if (typeof npc === 'string') {
        const cleanName = npc.replace(/^npc\./, '').replace(/_/g, ' ');
        return { id: npc, name: cleanName, role: 'dialogue', unknownRole: true };
      }
      return { id: npc.id, name: npc.name || npc.id, description: npc.description || '', role: npc.role || 'dialogue' };
    });
  }

  function displayQuestName(id = '') {
    return id.replace(/^(quest|npc)\./, '').replace(/_/g, ' ').replace(/\b\w/g, letter => letter.toUpperCase());
  }

  function normalizeQuest(q) {
    const id = q.quest_id || q.id;
    const status = String(q.status || 'active').toUpperCase();
    const state = {
      NOT_STARTED: 'available', AVAILABLE: 'available', IN_PROGRESS: 'active', ACTIVE: q.progress === '1/1' ? 'ready' : 'active', READY: 'ready',
      OBJECTIVES_MET: 'ready', COMPLETED: 'completed', UNAVAILABLE: 'unavailable'
    }[status] || (q.progress === '1/1' ? 'ready' : 'active');
    return { ...q, id, title: q.title || displayQuestName(id), state, steps: Array.isArray(q.steps) ? q.steps : [] };
  }

  function questStatusLabel(q) {
    return {available: 'Available', active: 'In progress', ready: 'Return to giver', completed: 'Completed', unavailable: 'Unavailable'}[q.state];
  }

  function questNextAction(q) {
    if (q.state === 'completed') return 'Reward received. Quest complete.';
    if (q.state === 'unavailable') return 'Another player has claimed this world’s unique reward.';
    const giver = q.giver_name || displayQuestName(q.giver || 'quest giver');
    const place = q.giver_location_name ? ` at ${q.giver_location_name}` : '';
    const step = q.steps.find(step => !step.completed);
    if (step?.hint) return step.hint;
    if (q.state === 'available') return `Speak to ${giver}${place} to accept this quest.`;
    if (step) return `${step.label}${step.location_name ? ` at ${step.location_name}` : ''}.`;
    if (q.state === 'ready' && q.giver) return `Speak to ${giver}${place} to claim your reward.`;
    return q.description || 'Speak to the quest giver for instructions.';
  }

  function canTalkToQuestGiver(q) {
    return Boolean(q.giver && ['available', 'ready'].includes(q.state) && currentRoom.npcs.some(npc => npc.id === q.giver));
  }

  function showActionError(payload, command, request = "", sender = "SERVER") {
    actionError = payload;
    if (command === "GROUP") { groupPending = false; groupError = payload; groupStatus = ""; }
    // Use the failed request's scope even if the player has switched tabs.
    const channel = command === "CHAT" ? request.trim().split(/\s+/)[1]?.toLowerCase() : command === "GROUP" ? "group" : null;
    if (CHAT_CHANNELS.includes(channel)) {
      addMessage(channel, sender, `Error: ${payload}`, "error", false);
    }
  }

  function handleServerERR(payload, command = "", request = "") {
    addLog(`S: ERR ${payload}`, "error");
    if (request === "GROUP STATUS") return;
    if (command === "GROUP" && /^404 GROUP_NOT_FOUND\b/.test(payload)) {
      const [, action, leader] = request.trim().split(/\s+/);
      if (action === "JOIN") groupInvitations = groupInvitations.filter(name => name !== leader);
    }
    const backend = getBackend();
    if (/^400 (INVALID_ARGUMENTS|UNKNOWN_COMMAND)\b/.test(payload)) {
      if (command === 'INFO' && request.startsWith('INFO ')) {
        const id = request.slice(5);
        const item = [...inventory, ...currentRoom.items].find(item => item.id === id);
        if (item) {
          showItemInfo({...item, usage: item.usage || 'This server does not provide detailed item information.'});
          return;
        }
      }
      const fallback = request === "LOOK DETAILS" ? "LOOK" : request === "INVENTORY DETAILS" ? "INVENTORY" : request === "QUESTS DETAILS" ? "QUESTS" : request.startsWith("TALKJSON ") ? request.replace(/^TALKJSON /, "TALK ") : null;
      if (fallback && backend) { invokeBackend("SendCommand", [fallback], false); return; }
    }
    if (command === "CONNECT") {
      connecting = false; connected = false; connectionError = payload;
      if (backend) backend.Disconnect().catch(e => addLog(String(e), "error"));
    }
    showActionError(payload, command, request);
    if (connecting) {
      connecting = false;
      connectionError = payload;
    }
  }

  function handleServerEVT(payload) {
    addLog(`S: EVT ${payload}`, "event");
    // Format: EVT <SCOPE> <EVENT_TYPE> [DATA...]
    const parts = payload.split(" ");
    const scope = (parts[0] || "").toLowerCase();
    const evtType = parts[1] || "";
    const rest = parts.slice(2).join(" ");

    if (scope === "stats" && /^players=\d+$/.test(evtType) && parts.length === 2) {
      const count = Number(evtType.slice(8));
      if (Number.isSafeInteger(count)) serverPlayersCount = count;
    } else if (scope === "group" && ["INVITE", "JOIN", "LEAVE", "LEADER"].includes(evtType) && parts.length === 3 && rest) {
      addMessage("group", "GROUP", `${evtType}: ${rest}`, "system", false);
      if (evtType === "INVITE") {
        if (!groupId && !groupInvitations.includes(rest)) groupInvitations = [...groupInvitations, rest];
      } else {
        refreshGroup();
      }
    } else if (CHAT_CHANNELS.includes(scope) && evtType === "CHAT" && parts[2] && parts.slice(3).join(" ").trim()) {
      const sender = parts[2];
      const text = parts.slice(3).join(" ");
      addMessage(scope, sender, text, "chat", false);
    } else if (scope === "room" && evtType === "PRESENCE" && ["ENTER", "LEAVE"].includes(parts[2]) && parts.length === 4 && parts[3]) {
      const action = parts[2] || ""; // ENTER or LEAVE
      const targetUser = parts[3] || "";
      const text = `${targetUser} ${action === "ENTER" ? "entered" : "left"} the room.`;
      addMessage("room", "ROOM", text, "presence", false);
      scheduleRefresh("Look", "Who");
    } else if (scope === "room" && evtType === "COMBAT" && rest) {
      addMessage("room", "COMBAT", rest, "combat", false);
      scheduleRefresh("Status", "Look", "Quests");
    } else if (scope === "room" && evtType === "ITEM" && ["TAKE", "DROP", "USE", "DELIVER", "REWARD"].includes(parts[2]) && parts.length === 5 && parts[3] && parts[4]) {
      const action = parts[2] || ""; // TAKE or DROP
      const actor = parts[3] || "";
      const itemId = parts[4] || "";
      const cleanItem = itemId.replace(/^item\./, '').replace(/_/g, ' ');
      const actionVerb = { TAKE: "picked up", DROP: "dropped", USE: "used", DELIVER: "delivered", REWARD: "received" }[action] || action.toLowerCase();
      const text = `${actor} ${actionVerb} ${cleanItem} (${itemId}).`;
      addMessage("room", "ROOM", text, "item", false);
      scheduleRefresh("Look", "Quests");
      if (parts[3] === username.trim()) {
        scheduleRefresh("Inventory", "Status", "Quests");
      }
    }
  }

  // Keep network failures visible and prevent unhandled action promises.
  function invoke(method, ...args) {
    return invokeBackend(method, args, true);
  }

  async function invokeBackend(method, args, clearError) {
    if (!connected) return false;
    const backend = getBackend();
    const request = method === "SendCommand" ? args[0] : `${method} ${args[0] || ""}`;
    const command = request.trim().split(/\s+/)[0].toUpperCase();
    try {
      if (!backend?.[method]) throw new Error("Wails backend unavailable. Launch with make gui.");
      if (clearError) actionError = "";
      if (clearError && command === "GROUP") groupError = "";
      await backend[method](...args);
      return true;
    } catch (err) {
      if (request === "GROUP STATUS") { addLog(String(err), "error"); return false; }
      showActionError(String(err), command, request, "CLIENT");
      addLog(String(err), "error");
      return false;
    }
  }

  async function handleConnect() {
    if (connecting) return;
    connectionError = "";
    username = (username || "").trim().toLowerCase();
    host = (host || "").trim();
    const userLength = [...username].length;
    if (userLength < 3 || userLength > 20 || !/^[\p{Ll}][\p{Ll}\p{Nd}_]*$/u.test(username)) {
      connectionError = "Use 3–20 lowercase letters, digits or underscores, starting with a lowercase letter.";
      return;
    }
    if (!host || /\s/u.test(host) || !Number.isInteger(Number(port)) || Number(port) < 1 || Number(port) > 65535) {
      connectionError = "Enter a server host and a port between 1 and 65535.";
      return;
    }
    const backend = getBackend();
    if (!backend) {
      connectionError = "Launch the desktop application with make gui.";
      return;
    }
    resetSession();
    connecting = true;
    try {
      await backend.Connect(host, Number(port), username);
    } catch (err) {
      connectionError = String(err);
      connecting = false;
      addLog("Connection failed: " + err, "error");
    }
  }

  async function handleDisconnect() {
    try { await getBackend()?.Disconnect(); }
    catch (err) { addLog(String(err), "error"); }
    finally { resetSession(); connectionError = ""; }
  }

  const callMove = direction => invoke("Move", direction);
  const callLook = () => invoke("Look");
  const callDrop = id => invoke("Drop", id);
  const callAttack = id => invoke("Attack", id);
  const callInventory = () => invoke("Inventory");
  const callStatus = () => invoke("Status");
  const callQuests = () => invoke("Quests");
  const callWho = () => invoke("Who");
  const callUse = id => invoke("SendCommand", "USE " + id);
  const callInfo = id => invoke("SendCommand", "INFO " + id);
  const callDefend = () => invoke("SendCommand", "DEFEND");
  const callFlee = () => invoke("SendCommand", "FLEE");
  function callGroup(action, target = "") {
    target = target.trim().toLowerCase();
    if (["INVITE", "JOIN"].includes(action) && ([...target].length < 3 || [...target].length > 20 || !/^[\p{Ll}][\p{Ll}\p{Nd}_]*$/u.test(target))) {
      const error = "Enter a username: 3–20 letters, digits or underscores, starting with a letter.";
      showActionError(error, "GROUP", "", "CLIENT");
      addLog(error, "error");
      return false;
    }
    if (groupPending) return false;
    groupPending = true;
    groupStatus = "";
    return invoke("Group", action, target).then(sent => {
      if (!sent) groupPending = false;
      return sent;
    });
  }
  function refreshGroup() {
    return invokeBackend("SendCommand", ["GROUP STATUS"], false);
  }
  function openGroup() {
    showGroupModal = true;
    refreshGroup();
  }
  async function handleSendMessage() {
    const text = chatInput.trim();
    if (!text || !CHAT_CHANNELS.includes(activeTab)) return;
    if (await invoke("Chat", activeTab.toUpperCase(), text)) chatInput = "";
  }
  function handleKeyDown(e) {
    if (e.key === "Enter" && !e.isComposing) handleSendMessage();
  }
  function selectMapEntity(_kind, id) {
    selectedEntity = id;
    tick().then(() => globalThis.document?.getElementById('nearby-' + id)?.focus());
  }
  function handleGameShortcut(event) {
    if (event.key === "Escape" && showGroupModal) { showGroupModal = false; return; }
    if (!connected || event.repeat || event.isComposing || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || activeDialogue || activeItemInfo || showGroupModal || mapExpanded) return;
    if (event.target?.closest?.('input, textarea, select, [contenteditable="true"], [role="dialog"]')) return;
    const action = {'1': callLook, '2': callStatus, '3': callQuests, '4': callWho, '5': openGroup}[event.key];
    if (action) { event.preventDefault(); action(); }
  }
</script>

<svelte:window on:keydown={handleGameShortcut} />

<main class="app-container" class:game-session={connected}>
  {#if !connected}
    <!-- CONNECTION SCREEN -->
    <div class="connect-overlay">
      <div class="connect-card">
        <h1 class="connect-title">42 TAP</h1>
        <p class="connect-subtitle">A LITTLE TEXT ADVENTURE</p>
        <div class="title-sprite"><PixelSprite name="hero" size={48} /></div>

        <form class="connect-form" on:submit|preventDefault={handleConnect}>
          <div class="form-group username-field">
            <label for="username">Player name</label>
            <input id="username" type="text" bind:value={username} placeholder="e.g. Garfield" required disabled={connecting} autocomplete="username" aria-describedby="username-hint" />
            <small id="username-hint">3–20 letters, digits or underscores. Uppercase letters are converted to lowercase.</small>
          </div>
          <div class="server-fields">
            <div class="form-group">
              <label for="host">Server address</label>
              <input id="host" type="text" bind:value={host} placeholder="127.0.0.1" required disabled={connecting} />
            </div>

            <div class="form-group">
              <label for="port">Port</label>
              <input id="port" type="number" bind:value={port} placeholder="4242" min="1" max="65535" step="1" required disabled={connecting} />
            </div>
          </div>

          {#if connectionError}
            <div class="error-badge" role="alert">{connectionError}</div>
          {/if}

          <button class="btn-primary" type="submit" disabled={connecting}>
            {connecting ? "Connecting…" : "Start"}
          </button>
        </form>
      </div>
    </div>
  {:else}
    <!-- ACTIVE GAME INTERFACE -->
    <header class="top-bar">
      <div class="brand">
        <span class="brand-mark">42 TAP</span>
        <span class="server-address">{host}:{port}</span>
      </div>

      <div class="stats-badges">
        <span class="badge room-badge">Room: {roomPlayersCount}</span>
        <span class="badge server-badge">Online: {serverPlayersCount}</span>
      </div>

      <button class="btn-disconnect" on:click={handleDisconnect}>Quit</button>
    </header>

    {#if actionError}<div class="action-error" role="alert">{actionError}</div>{/if}
    <div class="game-grid">
      <!-- Map and local interactions are driven exclusively by LOOK data. -->
      <section class="panel room-panel">
        <div class="panel-header location-header">
          <h2 class="room-name">{currentRoom.name}</h2>
        </div>
        <RoomMap room={currentRoom} names={roomNames} {username} inCombat={combatState === 'EN_COMBAT'} onmove={callMove} onselect={selectMapEntity} bind:expanded={mapExpanded} />
        <div class="location-caption"><p class="room-desc" title={currentRoom.description}>{currentRoom.description || 'Discovering your surroundings…'}</p></div>
        <Nearby room={currentRoom} {username} {selectedEntity} inCombat={combatState === 'EN_COMBAT'} onaction={invoke} />
      </section>

      <!-- RIGHT COLUMN: PLAYER STATUS & INVENTORY -->
      <aside class="panel status-panel">
        <div class="panel-header">
          <h2>PLAYER</h2>
          <span class="status-tag {combatState === 'EN_COMBAT' ? 'in-combat' : 'out-combat'}">
            {combatState === 'EN_COMBAT' ? 'In combat' : 'Exploring'}
          </span>
        </div>

        <div class="character-summary"><span class="avatar"><PixelSprite name="hero" size={32}/></span><strong>{username}</strong></div>
        <div class="vitality-box">
          <div class="hp-header">
            <span>HP</span>
            <span class="hp-numbers">{playerHP}/{playerMaxHP}</span>
          </div>
          <div class="hp-bar-bg" role="progressbar" aria-label="Health points" aria-valuenow={playerHP} aria-valuemin={0} aria-valuemax={playerMaxHP}>
            <div class="hp-bar-fill" style="width: {Math.max(0, Math.min(100, (playerHP / playerMaxHP) * 100))}%"></div>
          </div>
        </div>

        {#if combatState === 'EN_COMBAT'}
          <div class="combat-bar">
            <span class="combat-target-label">Facing <strong>{currentTarget || 'Hostile Enemy'}</strong></span>
            <div class="combat-buttons">
              {#if currentTarget}
                <button class="btn-action btn-attack" on:click={() => callAttack(currentTarget)}>Attack</button>
              {/if}
              <button class="btn-action btn-talk" on:click={callDefend}>Defend</button>
              <button class="btn-action btn-drop" on:click={callFlee}>Flee</button>
            </div>
          </div>
        {/if}

        <div class="inventory-section">
          <div class="panel-header-sub">
            <h3>Inventory <span class="count">{inventory.length}</span></h3>
            <button class="btn-tiny" on:click={callInventory} aria-label="Refresh inventory"><Icon name="refresh" size={16}/></button>
          </div>
          <div class="inventory-list">
            {#each inventory as item}
              <div class="inventory-item">
                <span class="item-name">{item.name || item.id}</span>
                <div class="npc-buttons">
                  {#if canUseItem(item)}
                    <button class="btn-action btn-talk" disabled={combatState === "EN_COMBAT" || playerHP >= playerMaxHP} on:click={() => callUse(item.id)} title={playerHP >= playerMaxHP ? 'Health is already full' : `Restore up to ${item.heal_value} HP`}>USE</button>
                  {:else}
                    <button class="btn-action btn-talk" on:click={() => callInfo(item.id)}>INFO</button>
                  {/if}
                  <button class="btn-action btn-drop" disabled={combatState === "EN_COMBAT"} on:click={() => callDrop(item.id)}>Drop</button>
                </div>
              </div>
            {/each}
            {#if inventory.length === 0}
              <div class="empty-list">Inventory empty.</div>
            {/if}
          </div>
        </div>

        <!-- ACTIVE QUESTS TRACKER -->
        <div class="quests-section">
          <div class="panel-header-sub">
            <h3>Quests <span class="count">{quests.length}</span></h3>
            <button class="btn-tiny" on:click={callQuests} aria-label="Refresh quests"><Icon name="refresh" size={16}/></button>
          </div>
          <div class="quests-list">
            {#each quests as q (q.id)}
              <div class="quest-card-item">
                <div class="quest-title-row">
                  <span class="quest-title">{q.title}</span>
                  <span class="quest-status-badge {q.state}">{questStatusLabel(q)}</span>
                </div>
                {#if q.description}<p class="quest-desc">{q.description}</p>{/if}
                {#if q.steps.length}
                  <ol class="quest-steps" aria-label="Quest objectives">
                    {#each q.steps as step}
                      <li class:complete={step.completed}>
                        <span class="quest-step-check" aria-label={step.completed ? 'Complete' : 'Incomplete'}>{step.completed ? '✓' : '○'}</span>
                        <span>{step.label}{#if step.location_name}<span class="quest-step-location">{step.location_name}</span>{/if}{#if step.hint && q.state !== 'completed' && q.state !== 'unavailable'}<span class="quest-step-hint">{step.hint}</span>{/if}</span>
                      </li>
                    {/each}
                  </ol>
                {/if}
                {#if q.giver}
                  <div class="quest-meta">Giver: <strong>{q.giver_name || displayQuestName(q.giver)}</strong></div>
                {/if}
                {#if q.reward}<div class="quest-meta">Reward: <strong>{q.reward}</strong>{#if q.reward_hp_heal} + up to {q.reward_hp_heal} HP restored{/if}</div>{/if}
                <p class="quest-next"><strong>{q.state === 'completed' || q.state === 'unavailable' ? '' : 'Next: '}</strong>{questNextAction(q)}</p>
                {#if canTalkToQuestGiver(q)}
                  <button class="btn-action btn-quest" disabled={combatState === 'EN_COMBAT'} on:click={() => invoke(q.state === 'available' ? 'Quest' : 'Talk', q.giver)}>{q.state === 'available' ? 'Accept quest' : 'Claim reward'}</button>
                {/if}
              </div>
            {/each}
            {#if quests.length === 0}
              <div class="empty-list">Talk to a quest giver.</div>
            {/if}
          </div>
          {#if quests.length}<p class="quest-meta quest-world-note">Each reward can be claimed once in this shared world.</p>{/if}
        </div>

        <!-- QUICK ACTION BUTTONS -->
        <div class="actions-section">
          <div class="hotbar-heading"><h3>MENU</h3></div>
          <div class="action-buttons-grid">
            <button class="btn-quick" on:click={callLook} title="Inspect and refresh your surroundings (1)" aria-keyshortcuts="1"><span>Look</span><kbd>1</kbd></button>
            <button class="btn-quick" on:click={callStatus} title="Refresh your health and combat state (2)" aria-keyshortcuts="2"><span>Status</span><kbd>2</kbd></button>
            <button class="btn-quick" on:click={callQuests} title="Refresh your quest journal (3)" aria-keyshortcuts="3"><span>Quests</span><kbd>3</kbd></button>
            <button class="btn-quick" on:click={callWho} title="Refresh the players in your room and online (4)" aria-keyshortcuts="4"><span>Players</span><kbd>4</kbd></button>
            <button class="btn-quick group-action" on:click={openGroup} title="Create, join or manage your group (5)" aria-keyshortcuts="5"><span>GROUP{groupInvitations.length ? ` (${groupInvitations.length})` : ''}</span><kbd>5</kbd></button>
          </div>
        </div>
      </aside>
    </div>

    <!-- BOTTOM ROW: CHAT & LOGS PANEL (MANDATORY CHAPTER V.4) -->
    <section class="panel chat-panel">
      <div class="tabs-header">
        <button class="tab-btn {activeTab === 'global' ? 'active' : ''}" on:click={() => activeTab = 'global'}>
          GLOBAL
        </button>
        <button class="tab-btn {activeTab === 'room' ? 'active' : ''}" on:click={() => activeTab = 'room'}>
          ROOM
        </button>
        <button class="tab-btn {activeTab === 'group' ? 'active' : ''}" on:click={() => activeTab = 'group'}>
          GROUP
        </button>
        <button class="tab-btn {activeTab === 'logs' ? 'active' : ''}" on:click={() => activeTab = 'logs'}>
          Logs
        </button>
      </div>

      <div class="chat-viewport" bind:this={chatViewport}>
        {#each messages[activeTab] as msg}
          <div class="chat-row {msg.type}">
            <span class="msg-time">{msg.time}</span>
            <span class="msg-sender">{msg.sender}</span>
            <span class="msg-text">{msg.text}</span>
          </div>
        {/each}
        {#if messages[activeTab].length === 0}
          <div class="empty-chat">{activeTab === 'logs' ? 'No events yet.' : 'No messages yet.'}</div>
        {/if}
      </div>

      <div class="chat-input-bar">
        <span class="channel-indicator">#{activeTab.toUpperCase()}</span>
        <input
          type="text"
          aria-label="Chat message"
          disabled={activeTab === "logs"}
          placeholder={activeTab === "logs" ? "Read only" : "Message..."}
          bind:value={chatInput}
          on:keydown={handleKeyDown}
        />
        <button class="btn-send" disabled={activeTab === "logs" || !chatInput.trim()} on:click={handleSendMessage}>Send</button>
      </div>
    </section>

    <!-- NPC DIALOGUE MODAL -->
    {#if activeItemInfo}
      <ItemInfo item={activeItemInfo} onclose={() => activeItemInfo = null} />
    {/if}
    {#if activeDialogue}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="NPC Dialogue">
          <div class="dialogue-header">
            <h3>{activeDialogue.npc}</h3>
            <button class="btn-close" aria-label="Close dialog" on:click={() => activeDialogue = null}><Icon name="close" size={16}/></button>
          </div>
          <p class="dialogue-speech">"{activeDialogue.text}"</p>
          <button class="btn-primary" on:click={() => activeDialogue = null}>Continue</button>
        </div>
      </div>
    {/if}

    <!-- GROUP MANAGEMENT MODAL -->
    {#if showGroupModal}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="Group Management">
          <div class="dialogue-header">
            <h3>GROUP</h3>
            <button class="btn-close" aria-label="Close group dialog" on:click={() => showGroupModal = false}><Icon name="close" size={16}/></button>
          </div>
          {#if groupError}<div class="error-badge" role="alert">{groupError}</div>{/if}
          {#if groupStatus}<p class="group-status" role="status">{groupStatus}</p>{/if}
          {#if groupPending}<p role="status">Waiting for the server…</p>{/if}
          {#if groupId}
            <section class="group-section">
              <h4>Your group</h4>
              <p class="subtitle">Leader: {groupLeader || 'Loading…'}</p>
              <ul class="group-members">
                {#each groupMembers as member}
                  <li>{member}{member === username ? ' (you)' : ''}{member === groupLeader ? ' · Leader' : ''}</li>
                {/each}
              </ul>
            </section>
            <form class="group-section" on:submit|preventDefault={() => callGroup("INVITE", groupTarget)}>
              <h4>Invite a player</h4>
              <p class="subtitle">They will receive an invitation and choose whether to join.</p>
              <div class="form-group">
                <label for="groupTarget">Player’s username</label>
                <input id="groupTarget" type="text" bind:value={groupTarget} placeholder="e.g. garfield" />
              </div>
              <button class="btn-action btn-talk" disabled={groupPending || !groupTarget.trim()} type="submit">Send invitation</button>
            </form>
            <button class="btn-action btn-drop" disabled={groupPending} on:click={() => callGroup("LEAVE")}>Leave group</button>
          {:else}
            {#if groupInvitations.length}
              <section class="group-section">
                <h4>Invitations received</h4>
                {#each groupInvitations as leader}
                  <div class="group-invitation">
                    <p>{leader} invited you to their group.</p>
                    <div class="group-actions">
                      <button class="btn-action btn-talk" disabled={groupPending} on:click={() => callGroup("JOIN", leader)}>Accept invitation</button>
                      <button class="btn-action" disabled={groupPending} on:click={() => groupInvitations = groupInvitations.filter(name => name !== leader)}>Dismiss</button>
                    </div>
                  </div>
                {/each}
              </section>
            {/if}
            <section class="group-section">
              <h4>Start a group</h4>
              <p class="subtitle">You are not in a group. Create one, then invite your friends.</p>
              <button class="btn-action btn-talk" disabled={groupPending} on:click={() => callGroup("CREATE")}>Create group</button>
            </section>
            <form class="group-section" on:submit|preventDefault={() => callGroup("JOIN", groupLeaderTarget)}>
              <h4>Join an existing group</h4>
              <p class="subtitle">Know the leader’s username? You can join directly. Names are case-insensitive.</p>
              <div class="form-group">
                <label for="groupLeaderTarget">Group leader’s username</label>
                <input id="groupLeaderTarget" type="text" bind:value={groupLeaderTarget} placeholder="e.g. alice" />
              </div>
              <button class="btn-action btn-take" disabled={groupPending || !groupLeaderTarget.trim()} type="submit">Join group</button>
            </form>
          {/if}
        </div>
      </div>
    {/if}
  {/if}
</main>

<script>
  import { onMount, tick } from 'svelte';

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

  function getDestName(destId) {
    return roomNames[destId] || String(destId).replace(/^loc\./, '').replace(/_/g, ' ');
  }

  // Inventory & Quests
  let inventory = [];
  let quests = [];

  // Group Management & Modal
  let showGroupModal = false;
  let groupTarget = "";

  // NPC Dialogue Modal
  let activeDialogue = null; // { npc: "...", text: "..." }

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
  let actionError = "";
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
      methods.forEach(method => invoke(method));
    }, 50);
  }

  // Auto-scroll chat view to bottom
  async function scrollToBottom() {
    await tick();
    if (chatViewport) chatViewport.scrollTop = chatViewport.scrollHeight;
  }

  // Helper for adding messages
  function addMessage(channel, sender, text, type = "normal") {
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    const msg = { time, sender, text, type };
    if (messages[channel]) {
      messages[channel] = [...messages[channel], msg].slice(-MESSAGE_LIMIT);
    }
    // Also push system events or chat into logs
    if (channel !== 'logs') {
      messages.logs = [...messages.logs, { time, sender: `[${channel.toUpperCase()}] ${sender}`, text, type }].slice(-MESSAGE_LIMIT);
    }
    scrollToBottom();
  }

  function addLog(text, type = "info") {
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
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
    inventory = [];
    quests = [];
    playerHP = 100;
    playerMaxHP = 100;
    combatState = "HORS_COMBAT";
    currentTarget = null;
    roomPlayersCount = 0;
    serverPlayersCount = 0;
    activeDialogue = null;
    showGroupModal = false;
    groupTarget = "";
    groupStatus = "";
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
        if (kind === "OK") handleServerOK(payload, command); else handleServerERR(payload, command, request);
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
  function handleServerOK(payload, command = "") {
    if (command === "GROUP") {
      groupStatus = "Party updated. Check Group Chat for notifications.";
      addMessage("group", "GROUP", groupStatus, "system");
    }
    if (!payload) return;
    addLog(`S: OK ${payload}`, "ok");

    if (payload === "connected") {
      connected = true;
      connecting = false;
      connectionError = "";
      addMessage("global", "SYSTEM", `Connected successfully as ${username}!`, "system");
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
        if (data.room && !Array.isArray(data.room) && data.room.id) {
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
        if (data.hp !== undefined && data.max_hp !== undefined) {
          playerHP = data.hp;
          playerMaxHP = data.max_hp || 100;
          combatState = data.state || (data.status === "combat" ? "EN_COMBAT" : "HORS_COMBAT");
          currentTarget = typeof data.combat === "string" ? data.combat : data.combat?.target_id || null;
          return;
        }

        // WHO payload
        if (data.room !== undefined && data.server !== undefined) {
          roomPlayersCount = Array.isArray(data.room) ? data.room.length : data.room;
          serverPlayersCount = data.server;
          return;
        }

        // INVENTORY payload (Array)
        if (Array.isArray(data)) {
          // Could be INVENTORY or QUESTS
          if (command === "QUESTS" || (data[0] && (data[0].quest_id || data[0].title))) {
            quests = data.map(q => ({...q, id: q.quest_id || q.id, title: q.title || q.quest_id || q.id}));
          } else {
            inventory = normalizeItems(data);
          }
          return;
        }

        // TALK payload
        if (data.npc && data.dialogue) {
          const text = Array.isArray(data.dialogue) ? data.dialogue.join("\n") : String(data.dialogue);
          activeDialogue = { npc: data.npc, text };
          addMessage("room", data.npc, text, "dialogue");
          return;
        }
      } catch (e) {
        console.warn("Could not parse JSON OK payload:", e);
      }
    }

    // Key-value OK responses: OK room=loc.market, OK taken=item.apple, etc.
    if (command === "TALK") {
      activeDialogue = {npc: "NPC", text: payload};
      addMessage("room", "NPC", payload, "dialogue");
    } else if (payload.startsWith("players=")) {
      serverPlayersCount = Number(payload.slice(8));
    } else if (payload.startsWith("room=")) {
      // Room changed, refresh look
      scheduleRefresh("Look");
    } else if (payload.startsWith("taken=") || payload.startsWith("dropped=")) {
      // Item action confirmed, refresh room & inventory
      scheduleRefresh("Look", "Inventory", "Quests");
    } else if (payload.startsWith("used=")) {
      scheduleRefresh("Inventory", "Status", "Quests");
    }
  }

  function normalizeItems(rawItems) {
    return rawItems.map(item => {
      if (typeof item === 'string') {
        const cleanName = item.replace(/^item\./, '').replace(/_/g, ' ');
        return { id: item, name: cleanName, obtainable: true };
      }
      return { id: item.id, name: item.name || item.id, obtainable: item.obtainable !== false };
    });
  }

  function normalizeNPCs(rawNPCs) {
    return rawNPCs.map(npc => {
      if (typeof npc === 'string') {
        const cleanName = npc.replace(/^npc\./, '').replace(/_/g, ' ');
        return { id: npc, name: cleanName, role: 'dialogue', unknownRole: true };
      }
      return { id: npc.id, name: npc.name || npc.id, role: npc.role || 'dialogue' };
    });
  }

  function handleServerERR(payload, command = "", request = "") {
    const backend = getBackend();
    if (/^400 (INVALID_ARGUMENTS|UNKNOWN_COMMAND)\b/.test(payload)) {
      const fallback = request === "LOOK DETAILS" ? "LOOK" : request === "INVENTORY DETAILS" ? "INVENTORY" : request.startsWith("TALKJSON ") ? request.replace(/^TALKJSON /, "TALK ") : null;
      if (fallback && backend) { invoke("SendCommand", fallback); return; }
    }
    if (command === "CONNECT") {
      connecting = false; connected = false; connectionError = payload;
      if (backend) backend.Disconnect().catch(e => addLog(String(e), "error"));
    }
    actionError = payload;
    addLog(`S: ERR ${payload}`, "error");
    addMessage("room", "SERVER", `Error: ${payload}`, "error");
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

    if (scope === "stats" && evtType.startsWith("players=")) {
      serverPlayersCount = Number(evtType.slice(8));
    } else if (scope === "group" && ["INVITE", "JOIN", "LEAVE", "LEADER"].includes(evtType)) {
      addMessage("group", "GROUP", `${evtType}: ${rest}`, "system");
    } else if (evtType === "CHAT") {
      const sender = parts[2] || "someone";
      const text = parts.slice(3).join(" ");
      addMessage(scope, sender, text, "chat");
    } else if (evtType === "PRESENCE") {
      const action = parts[2] || ""; // ENTER or LEAVE
      const targetUser = parts[3] || "";
      const text = `${targetUser} ${action === "ENTER" ? "entered" : "left"} the room.`;
      addMessage("room", "ROOM", text, "presence");
      scheduleRefresh("Look", "Who");
    } else if (evtType === "COMBAT") {
      addMessage("room", "COMBAT", rest, "combat");
      scheduleRefresh("Status", "Look", "Quests");
    } else if (evtType === "ITEM") {
      const action = parts[2] || ""; // TAKE or DROP
      const actor = parts[3] || "";
      const itemId = parts[4] || "";
      const cleanItem = itemId.replace(/^item\./, '').replace(/_/g, ' ');
      const actionVerb = { TAKE: "picked up", DROP: "dropped", USE: "used", DELIVER: "delivered", REWARD: "received" }[action] || action.toLowerCase();
      const text = `${actor} ${actionVerb} ${cleanItem} (${itemId}).`;
      addMessage("room", "ROOM", text, "item");
      scheduleRefresh("Look");
      if (parts[3] === username.trim() && ["USE", "DELIVER", "REWARD"].includes(parts[2])) {
        scheduleRefresh("Inventory", "Status", "Quests");
      }
    }
  }

  // Keep network failures visible and prevent unhandled action promises.
  async function invoke(method, ...args) {
    if (!connected) return false;
    const backend = getBackend();
    try {
      if (!backend?.[method]) throw new Error("Wails backend unavailable. Launch with make gui.");
      actionError = "";
      await backend[method](...args);
      return true;
    } catch (err) {
      actionError = String(err);
      addLog(actionError, "error");
      return false;
    }
  }

  async function handleConnect() {
    if (connecting) return;
    connectionError = "";
    username = (username || "").trim();
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
  const callTake = id => invoke("Take", id);
  const callDrop = id => invoke("Drop", id);
  const callTalk = id => invoke("Talk", id);
  const callAttack = id => invoke("Attack", id);
  const callInventory = () => invoke("Inventory");
  const callStatus = () => invoke("Status");
  const callQuests = () => invoke("Quests");
  const callWho = () => invoke("Who");
  const callQuest = id => invoke("Quest", id);
  const callUse = id => invoke("SendCommand", "USE " + id);
  const callDefend = () => invoke("SendCommand", "DEFEND");
  const callFlee = () => invoke("SendCommand", "FLEE");
  function callGroup(action, target = "") {
    target = target.trim();
    if (["INVITE", "JOIN"].includes(action) && !target) {
      actionError = "Enter the player's username (the leader for JOIN).";
      return false;
    }
    return invoke("Group", action, target);
  }
  async function handleSendMessage() {
    const text = chatInput.trim();
    if (!text || activeTab === "logs") return;
    if (await invoke("Chat", activeTab.toUpperCase(), text)) chatInput = "";
  }
  function handleKeyDown(e) {
    if (e.key === "Enter" && !e.isComposing) handleSendMessage();
  }
</script>

<main class="app-container">
  {#if !connected}
    <!-- CONNECTION SCREEN -->
    <div class="connect-overlay">
      <div class="connect-card">
        <div class="retro-badge">42 CURRICULUM PROJECT</div>
        <h1 class="glow-title">THE ANSWER PROTOCOL</h1>
        <p class="subtitle">Choose your username and connect to start playing.</p>

        <form class="connect-form" on:submit|preventDefault={handleConnect}>
          <div class="form-group">
            <label for="host">SERVER HOST</label>
            <input id="host" type="text" bind:value={host} placeholder="127.0.0.1" required disabled={connecting} />
          </div>

          <div class="form-group">
            <label for="port">PORT</label>
            <input id="port" type="number" bind:value={port} placeholder="4242" min="1" max="65535" step="1" required disabled={connecting} />
          </div>

          <div class="form-group">
            <label for="username">ADVENTURER USERNAME</label>
            <input id="username" type="text" bind:value={username} placeholder="e.g. aris" required disabled={connecting} autocomplete="username" />
          </div>

          {#if connectionError}
            <div class="error-badge">{connectionError}</div>
          {/if}

          <button class="btn-primary" type="submit" disabled={connecting}>
            {connecting ? "CONNECTING..." : "CONNECT & PLAY"}
          </button>
        </form>
      </div>
    </div>
  {:else}
    <!-- ACTIVE GAME INTERFACE -->
    <header class="top-bar">
      <div class="brand">
        <span class="pulse-dot"></span>
        <span class="realm-title">TAP // {host}:{port}</span>
      </div>

      <div class="stats-badges">
        <span class="badge player-badge">USER: <strong>{username}</strong></span>
        <span class="badge room-badge">ROOM PLAYERS: <strong>{roomPlayersCount}</strong></span>
        <span class="badge server-badge">SERVER TOTAL: <strong>{serverPlayersCount}</strong></span>
      </div>

      <button class="btn-disconnect" on:click={handleDisconnect}>QUIT / LEAVE</button>
    </header>

    {#if actionError}<div class="action-error" role="alert">{actionError}</div>{/if}
    <div class="game-grid">
      <!-- LEFT COLUMN: WORLD EXPLORATION -->
      <section class="panel room-panel">
        <div class="panel-header">
          <h2>CURRENT LOCATION</h2>
          <span class="room-id">[{currentRoom.id}]</span>
        </div>

        <div class="room-content">
          <h3 class="room-name">{currentRoom.name}</h3>
          <p class="room-desc">{currentRoom.description}</p>

          <div class="exits-section">
            <h4>AVAILABLE EXITS</h4>
            <div class="exits-grid">
              {#each Object.entries(currentRoom.exits) as [dir, dest]}
                <button class="btn-exit" disabled={combatState === "EN_COMBAT"} on:click={() => callMove(dir)}>
                  <span class="dir-tag">{dir.toUpperCase()}</span>
                  <span class="dest-tag">{getDestName(dest)}</span>
                </button>
              {/each}
              {#if Object.keys(currentRoom.exits).length === 0}
                <div class="empty-hint">No exits available (Dead end)</div>
              {/if}
            </div>
          </div>
        </div>

        <!-- ENTITIES IN ROOM -->
        <div class="entities-grid">
          <!-- GROUND ITEMS -->
          <div class="entity-card">
            <h4>ITEMS ON GROUND ({currentRoom.items.length})</h4>
            <div class="entity-list">
              {#each currentRoom.items as item}
                <div class="entity-item">
                  <span class="entity-name">{item.name}</span>
                  {#if item.obtainable}
                    <button class="btn-action btn-take" disabled={combatState === "EN_COMBAT"} on:click={() => callTake(item.id)}>TAKE</button>
                  {:else}
                    <span class="scenery-tag">SCENERY</span>
                  {/if}
                </div>
              {/each}
              {#if currentRoom.items.length === 0}
                <div class="empty-list">Ground is clear</div>
              {/if}
            </div>
          </div>

          <!-- NPCS IN ROOM -->
          <div class="entity-card">
            <h4>NPCS PRESENT ({currentRoom.npcs.length})</h4>
            <div class="entity-list">
              {#each currentRoom.npcs as npc}
                <div class="entity-item">
                  <span class="entity-name">{npc.name}</span>
                  <div class="npc-buttons">
                    {#if npc.role !== 'enemy'}
                      <button class="btn-action btn-talk" disabled={combatState === "EN_COMBAT"} on:click={() => callTalk(npc.id)}>TALK</button>
                    {/if}
                    {#if npc.role === 'quest_giver' || npc.unknownRole}
                      <button class="btn-action btn-take" disabled={combatState === "EN_COMBAT"} on:click={() => callQuest(npc.id)}>QUEST</button>
                    {/if}
                    {#if npc.role === 'enemy' || npc.unknownRole}
                      <button class="btn-action btn-attack" on:click={() => callAttack(npc.id)}>ATTACK</button>
                    {/if}
                  </div>
                </div>
              {/each}
              {#if currentRoom.npcs.length === 0}
                <div class="empty-list">No one is here</div>
              {/if}
            </div>
          </div>

          <!-- PLAYERS IN ROOM -->
          <div class="entity-card">
            <h4>PLAYERS PRESENT ({currentRoom.players.length})</h4>
            <div class="entity-list">
              {#each currentRoom.players as player}
                <div class="entity-item">
                  <span class="player-tag {player === username ? 'player-self' : 'player-other'}">
                    👤 {player} {player === username ? '(You)' : ''}
                  </span>
                </div>
              {/each}
              {#if currentRoom.players.length === 0}
                <div class="empty-list">No other adventurers here</div>
              {/if}
            </div>
          </div>
        </div>
      </section>

      <!-- RIGHT COLUMN: PLAYER STATUS & INVENTORY -->
      <aside class="panel status-panel">
        <div class="panel-header">
          <h2>CHARACTER VITALITY</h2>
          <span class="status-tag {combatState === 'EN_COMBAT' ? 'in-combat' : 'out-combat'}">
            {combatState}
          </span>
        </div>

        <div class="vitality-box">
          <div class="hp-header">
            <span>HEALTH POINTS</span>
            <span class="hp-numbers">{playerHP} / {playerMaxHP} HP</span>
          </div>
          <div class="hp-bar-bg">
            <div class="hp-bar-fill" style="width: {Math.max(0, Math.min(100, (playerHP / playerMaxHP) * 100))}%"></div>
          </div>
        </div>

        {#if combatState === 'EN_COMBAT'}
          <div class="combat-bar">
            <span class="combat-target-label">TARGET: <strong>{currentTarget || 'Hostile Enemy'}</strong></span>
            <div class="combat-buttons">
              {#if currentTarget}
                <button class="btn-action btn-attack" on:click={() => callAttack(currentTarget)}>ATTACK</button>
              {/if}
              <button class="btn-action btn-talk" on:click={callDefend}>DEFEND</button>
              <button class="btn-action btn-drop" on:click={callFlee}>FLEE</button>
            </div>
          </div>
        {/if}

        <div class="inventory-section">
          <div class="panel-header-sub">
            <h3>INVENTORY ({inventory.length})</h3>
            <button class="btn-tiny" on:click={callInventory}>REFRESH</button>
          </div>
          <div class="inventory-list">
            {#each inventory as item}
              <div class="inventory-item">
                <span class="item-name">{item.name || item.id}</span>
                <div class="npc-buttons">
                  <button class="btn-action btn-talk" disabled={combatState === "EN_COMBAT"} on:click={() => callUse(item.id)}>USE</button>
                  <button class="btn-action btn-drop" disabled={combatState === "EN_COMBAT"} on:click={() => callDrop(item.id)}>DROP</button>
                </div>
              </div>
            {/each}
            {#if inventory.length === 0}
              <div class="empty-list">Backpack is empty</div>
            {/if}
          </div>
        </div>

        <!-- ACTIVE QUESTS TRACKER -->
        <div class="quests-section">
          <div class="panel-header-sub">
            <h3>QUEST TRACKER ({quests.length})</h3>
            <button class="btn-tiny" on:click={callQuests}>REFRESH</button>
          </div>
          <div class="quests-list">
            {#each quests as q}
              <div class="quest-card-item">
                <div class="quest-title-row">
                  <span class="quest-title">{q.title || q.id}</span>
                  <span class="quest-status-badge {q.status || 'active'}">{q.status || 'ACTIVE'}</span>
                </div>
                <p class="quest-desc">{q.description || ''}</p>
                {#if q.progress}<div class="quest-meta">Progress: {q.progress}</div>{/if}
                {#if q.giver}
                  <div class="quest-meta">Giver: <strong>{q.giver.replace(/^npc\./, '')}</strong></div>
                {/if}
              </div>
            {/each}
            {#if quests.length === 0}
              <div class="empty-list">No active quests. Talk to NPCs to explore!</div>
            {/if}
          </div>
        </div>

        <!-- QUICK ACTION BUTTONS -->
        <div class="actions-section">
          <h3>GAME ACTIONS</h3>
          <div class="action-buttons-grid">
            <button class="btn-quick" on:click={callLook}>LOOK</button>
            <button class="btn-quick" on:click={callStatus}>STATUS</button>
            <button class="btn-quick" on:click={callQuests}>QUESTS</button>
            <button class="btn-quick" on:click={callWho}>WHO</button>
            <button class="btn-quick" on:click={() => showGroupModal = true}>GROUP</button>
          </div>
        </div>
      </aside>
    </div>

    <!-- BOTTOM ROW: CHAT & LOGS PANEL (MANDATORY CHAPTER V.4) -->
    <section class="panel chat-panel">
      <div class="tabs-header">
        <button class="tab-btn {activeTab === 'global' ? 'active' : ''}" on:click={() => activeTab = 'global'}>
          GLOBAL CHAT ({messages.global.length})
        </button>
        <button class="tab-btn {activeTab === 'room' ? 'active' : ''}" on:click={() => activeTab = 'room'}>
          ROOM CHAT ({messages.room.length})
        </button>
        <button class="tab-btn {activeTab === 'group' ? 'active' : ''}" on:click={() => activeTab = 'group'}>
          GROUP CHAT ({messages.group.length})
        </button>
        <button class="tab-btn {activeTab === 'logs' ? 'active' : ''}" on:click={() => activeTab = 'logs'}>
          SYSTEM LOGS ({messages.logs.length})
        </button>
      </div>

      <div class="chat-viewport" bind:this={chatViewport}>
        {#each messages[activeTab] as msg}
          <div class="chat-row {msg.type}">
            <span class="msg-time">[{msg.time}]</span>
            <span class="msg-sender">&lt;{msg.sender}&gt;</span>
            <span class="msg-text">{msg.text}</span>
          </div>
        {/each}
        {#if messages[activeTab].length === 0}
          <div class="empty-chat">No messages yet in this channel.</div>
        {/if}
      </div>

      <div class="chat-input-bar">
        <span class="channel-indicator">#{activeTab.toUpperCase()}:</span>
        <input
          type="text"
          aria-label="Chat message"
          disabled={activeTab === "logs"}
          placeholder={activeTab === "logs" ? "System logs — read only" : "Type message and press Enter..."}
          bind:value={chatInput}
          on:keydown={handleKeyDown}
        />
        <button class="btn-send" disabled={activeTab === "logs" || !chatInput.trim()} on:click={handleSendMessage}>SEND</button>
      </div>
    </section>

    <!-- NPC DIALOGUE MODAL -->
    {#if activeDialogue}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="NPC Dialogue">
          <div class="dialogue-header">
            <h3>{activeDialogue.npc} SPEAKS</h3>
            <button class="btn-close" aria-label="Close dialog" on:click={() => activeDialogue = null}>✕</button>
          </div>
          <p class="dialogue-speech">"{activeDialogue.text}"</p>
          <button class="btn-primary" on:click={() => activeDialogue = null}>CONTINUE</button>
        </div>
      </div>
    {/if}

    <!-- GROUP MANAGEMENT MODAL -->
    {#if showGroupModal}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="Group Management">
          <div class="dialogue-header">
            <h3>PARTY & GROUP ACTIONS</h3>
            <button class="btn-close" on:click={() => showGroupModal = false}>✕</button>
          </div>
          <p class="subtitle" style="margin-bottom: 1rem;">Create a party, invite a player, or enter a leader’s username to join.</p>
          {#if actionError}<div class="error-badge" role="alert">{actionError}</div>{/if}
          {#if groupStatus}<p class="group-status" role="status">{groupStatus}</p>{/if}
          <div class="form-group" style="margin-bottom: 1rem;">
            <label for="groupTarget">ADVENTURER USERNAME</label>
            <input id="groupTarget" type="text" bind:value={groupTarget} placeholder="e.g. aris" />
          </div>
          <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
            <button class="btn-action btn-talk" on:click={() => callGroup("CREATE")}>CREATE PARTY</button>
            <button class="btn-action btn-talk" disabled={!groupTarget.trim()} on:click={() => callGroup("INVITE", groupTarget)}>INVITE</button>
            <button class="btn-action btn-take" disabled={!groupTarget.trim()} on:click={() => callGroup("JOIN", groupTarget)}>JOIN PARTY</button>
            <button class="btn-action btn-drop" on:click={() => callGroup("LEAVE", "")}>LEAVE PARTY</button>
            <button class="btn-primary" style="margin-left: auto; padding: 0.35rem 0.8rem;" on:click={() => showGroupModal = false}>CLOSE</button>
          </div>
        </div>
      </div>
    {/if}
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "JetBrains Mono", monospace;
    background-color: #0d1117;
    color: #e6edf3;
    overflow: auto;
  }

  .app-container {
    height: 100vh;
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }

  /* CONNECTION SCREEN */
  .connect-overlay {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: radial-gradient(circle at center, #161b22 0%, #090d13 100%);
  }

  .connect-card {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 8px;
    padding: 2.5rem;
    width: 380px;
    box-shadow: 0 12px 36px rgba(0,0,0,0.6);
    text-align: center;
  }

  .retro-badge {
    display: inline-block;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 2px;
    color: #58a6ff;
    border: 1px solid #1f6feb;
    padding: 3px 8px;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  .glow-title {
    font-size: 1.6rem;
    margin: 0 0 0.5rem;
    letter-spacing: 1px;
    color: #f0f6fc;
  }

  .subtitle {
    color: #8b949e;
    font-size: 0.85rem;
    margin-bottom: 2rem;
  }

  .connect-form {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
    text-align: left;
  }

  .form-group label {
    display: block;
    font-size: 0.75rem;
    font-weight: 600;
    color: #8b949e;
    margin-bottom: 0.35rem;
    letter-spacing: 1px;
  }

  .form-group input {
    width: 100%;
    padding: 0.65rem 0.8rem;
    background: #0d1117;
    border: 1px solid #30363d;
    border-radius: 6px;
    color: #f0f6fc;
    font-size: 0.95rem;
    box-sizing: border-box;
    outline: none;
    transition: border-color 0.2s;
  }

  .form-group input:focus {
    border-color: #58a6ff;
  }

  .error-badge {
    background: rgba(248, 81, 73, 0.15);
    border: 1px solid #f85149;
    color: #f85149;
    font-size: 0.8rem;
    padding: 0.5rem;
    border-radius: 4px;
    text-align: center;
  }

  .btn-primary {
    background: #238636;
    color: #fff;
    border: none;
    padding: 0.75rem;
    font-size: 0.95rem;
    font-weight: 600;
    border-radius: 6px;
    cursor: pointer;
    transition: background 0.2s;
  }

  .btn-primary:hover:not(:disabled) {
    background: #2ea043;
  }

  /* TOP HEADER */
  .top-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: #161b22;
    border-bottom: 1px solid #30363d;
    padding: 0.5rem 1rem;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #3fb950;
    box-shadow: 0 0 8px #3fb950;
  }

  .realm-title {
    font-weight: 700;
    font-size: 0.9rem;
    letter-spacing: 1px;
    color: #58a6ff;
  }

  .stats-badges {
    display: flex;
    gap: 0.6rem;
  }

  .badge {
    background: #21262d;
    border: 1px solid #30363d;
    padding: 0.25rem 0.6rem;
    border-radius: 4px;
    font-size: 0.75rem;
    color: #8b949e;
  }

  .badge strong {
    color: #f0f6fc;
  }

  .btn-disconnect {
    background: #21262d;
    border: 1px solid #f85149;
    color: #f85149;
    padding: 0.3rem 0.7rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
  }

  /* GRID LAYOUT */
  .game-grid {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 0.75rem;
    padding: 0.75rem;
    flex: 1;
    min-height: 0;
  }

  .panel {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0.8rem;
    border-bottom: 1px solid #21262d;
  }

  .panel-header h2 {
    margin: 0;
    font-size: 0.8rem;
    letter-spacing: 1px;
    color: #8b949e;
  }

  .room-id {
    font-size: 0.75rem;
    color: #6e7681;
    font-family: monospace;
  }

  .room-panel { overflow-y: auto; }

  .room-content {
    padding: 0.8rem;
  }

  .room-name {
    margin: 0 0 0.5rem;
    font-size: 1.3rem;
    color: #58a6ff;
  }

  .room-desc {
    margin: 0 0 1rem;
    font-size: 0.95rem;
    line-height: 1.45;
    color: #c9d1d9;
  }

  .exits-section h4 {
    margin: 0 0 0.4rem;
    font-size: 0.75rem;
    color: #8b949e;
    letter-spacing: 1px;
  }

  .exits-grid {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .btn-exit {
    display: flex;
    flex-direction: column;
    align-items: center;
    background: #21262d;
    border: 1px solid #30363d;
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .btn-exit:hover {
    border-color: #58a6ff;
    background: #30363d;
  }

  .dir-tag {
    font-weight: 700;
    font-size: 0.8rem;
    color: #388bfd;
  }

  .dest-tag {
    font-size: 0.7rem;
    color: #8b949e;
  }

  /* ENTITIES */
  .entities-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.6rem;
    padding: 0.8rem;
    border-top: 1px solid #21262d;
    flex: 1;
  }

  .player-tag {
    font-size: 0.85rem;
    font-weight: 600;
  }
  .player-tag.player-self {
    color: #3fb950;
  }
  .player-tag.player-other {
    color: #58a6ff;
  }

  .entity-card {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 4px;
    padding: 0.5rem 0.7rem;
    display: flex;
    flex-direction: column;
  }

  .entity-card h4 {
    margin: 0 0 0.4rem;
    font-size: 0.75rem;
    color: #8b949e;
  }

  .entity-list {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    overflow-y: auto;
  }

  .entity-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.3rem 0.5rem;
    background: #161b22;
    border-radius: 4px;
    font-size: 0.85rem;
  }

  .btn-action {
    border: none;
    padding: 0.25rem 0.5rem;
    border-radius: 3px;
    font-size: 0.7rem;
    font-weight: 700;
    cursor: pointer;
  }

  .btn-take { background: #238636; color: #fff; }
  .btn-drop { background: #da3633; color: #fff; }
  .btn-talk { background: #1f6feb; color: #fff; }
  .btn-attack { background: #b62324; color: #fff; }

  .npc-buttons { display: flex; gap: 0.3rem; }
  .empty-list, .empty-hint { font-size: 0.8rem; color: #6e7681; font-style: italic; }

  /* VITALITY */
  .status-panel { overflow-y: auto; padding: 0.8rem; gap: 0.8rem; }
  .status-tag { font-size: 0.75rem; font-weight: 700; padding: 2px 6px; border-radius: 4px; }
  .status-tag.in-combat { background: #b62324; color: #fff; }
  .status-tag.out-combat { background: #238636; color: #fff; }

  .hp-header { display: flex; justify-content: space-between; font-size: 0.8rem; font-weight: 700; margin-bottom: 0.3rem; }
  .hp-bar-bg { background: #21262d; border-radius: 4px; height: 12px; overflow: hidden; border: 1px solid #30363d; }
  .hp-bar-fill { background: linear-gradient(90deg, #da3633, #238636); height: 100%; transition: width 0.3s; }

  .combat-bar {
    background: rgba(182, 35, 36, 0.15);
    border: 1px solid #b62324;
    border-radius: 4px;
    padding: 0.4rem 0.6rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .combat-target-label {
    font-size: 0.75rem;
    color: #f85149;
    font-weight: 700;
  }
  .combat-buttons {
    display: flex;
    gap: 0.3rem;
  }

  .inventory-section { flex: 1; display: flex; flex-direction: column; min-height: 0; }
  .panel-header-sub { display: flex; justify-content: space-between; align-items: center; }
  .panel-header-sub h3 { margin: 0 0 0.4rem; font-size: 0.8rem; color: #8b949e; }
  .inventory-list { background: #0d1117; border: 1px solid #21262d; border-radius: 4px; padding: 0.4rem; overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: 0.3rem; }
  .inventory-item { display: flex; justify-content: space-between; align-items: center; padding: 0.3rem 0.5rem; background: #161b22; border-radius: 3px; font-size: 0.85rem; }

  /* QUESTS */
  .quests-section { flex: 1; display: flex; flex-direction: column; min-height: 0; }
  .quests-list { background: #0d1117; border: 1px solid #21262d; border-radius: 4px; padding: 0.4rem; overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: 0.35rem; }
  .quest-card-item { background: #161b22; border-left: 3px solid #e3b341; border-radius: 3px; padding: 0.4rem 0.6rem; }
  .quest-title-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.2rem; }
  .quest-title { font-size: 0.8rem; font-weight: 700; color: #f0f6fc; }
  .quest-status-badge { font-size: 0.65rem; padding: 1px 4px; border-radius: 3px; background: #238636; color: #fff; font-weight: 700; }
  .quest-desc { font-size: 0.75rem; color: #8b949e; margin: 0 0 0.2rem; line-height: 1.3; }
  .quest-meta { font-size: 0.7rem; color: #6e7681; }

  .actions-section h3 { margin: 0 0 0.4rem; font-size: 0.8rem; color: #8b949e; }
  .action-buttons-grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 0.35rem; }
  .btn-quick { background: #21262d; border: 1px solid #30363d; color: #c9d1d9; padding: 0.45rem 0.2rem; font-size: 0.75rem; font-weight: 700; border-radius: 4px; cursor: pointer; text-align: center; }
  .btn-quick:hover { border-color: #58a6ff; color: #fff; }

  /* CHAT PANEL */
  .chat-panel { flex-shrink: 0; margin: 0 0.75rem 0.75rem; height: 210px; }
  .tabs-header { display: flex; background: #0d1117; border-bottom: 1px solid #21262d; }
  .tab-btn { background: none; border: none; border-bottom: 2px solid transparent; color: #8b949e; padding: 0.5rem 0.9rem; font-size: 0.75rem; font-weight: 700; cursor: pointer; }
  .tab-btn.active { color: #58a6ff; border-bottom-color: #58a6ff; background: #161b22; }

  .chat-viewport { flex: 1; padding: 0.6rem; overflow-y: auto; display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.85rem; font-family: monospace; }
  .chat-row { display: flex; gap: 0.4rem; }
  .msg-time { color: #6e7681; }
  .msg-sender { color: #58a6ff; font-weight: 700; }
  .chat-row.error .msg-sender, .chat-row.error .msg-text { color: #f85149; }
  .chat-row.dialogue .msg-text { color: #e3b341; }
  .chat-row.combat .msg-text { color: #f78166; }
  .empty-chat { color: #6e7681; font-style: italic; }

  .chat-input-bar { display: flex; align-items: center; gap: 0.5rem; padding: 0.4rem 0.6rem; border-top: 1px solid #21262d; background: #0d1117; }
  .channel-indicator { font-size: 0.75rem; font-weight: 700; color: #58a6ff; }
  .chat-input-bar input { flex: 1; background: #161b22; border: 1px solid #30363d; border-radius: 4px; padding: 0.4rem 0.6rem; color: #f0f6fc; outline: none; }
  .btn-send { background: #238636; border: none; color: #fff; padding: 0.4rem 0.8rem; border-radius: 4px; font-weight: 700; cursor: pointer; }

  /* DIALOGUE MODAL */
  .modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.7); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .dialogue-card { background: #161b22; border: 1px solid #58a6ff; border-radius: 8px; width: 420px; padding: 1.5rem; box-shadow: 0 16px 48px rgba(0,0,0,0.8); }
  .dialogue-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
  .dialogue-header h3 { margin: 0; font-size: 1.1rem; color: #58a6ff; }
  .btn-close { background: none; border: none; color: #8b949e; font-size: 1.1rem; cursor: pointer; }
  .dialogue-speech { white-space: pre-wrap; font-size: 1rem; line-height: 1.5; color: #e3b341; font-style: italic; margin-bottom: 1.5rem; }
  .action-error { background: #3b171b; color: #ffb4b0; padding: 0.5rem 1rem; }
  .group-status { color: #3fb950; overflow-wrap: anywhere; }
  .msg-text, .entity-name, .item-name { overflow-wrap: anywhere; min-width: 0; }
  .msg-text { white-space: pre-wrap; }
  .npc-buttons { flex-wrap: wrap; justify-content: flex-end; }
  .connect-card, .dialogue-card { max-width: calc(100vw - 2rem); }
  .modal-backdrop { padding: 1rem; overflow-y: auto; }
  .dialogue-card { max-height: calc(100vh - 2rem); overflow-y: auto; }
  .connect-overlay { min-height: 100vh; height: auto; padding: 1rem 0; }
  @media (max-width: 1000px) {
    .entities-grid { grid-template-columns: 1fr; }
    .top-bar, .stats-badges { flex-wrap: wrap; gap: 0.5rem; }
    .entity-card { min-height: 100px; }
    .action-buttons-grid { grid-template-columns: repeat(3, 1fr); }
  }
  @media (max-width: 700px), (max-height: 650px) {
    .app-container { height: auto; min-height: 100vh; }
    .game-grid { grid-template-columns: 1fr; }
    .room-panel, .status-panel { overflow: visible; }
    .inventory-list, .quests-list { max-height: 250px; flex: auto; }
    .chat-panel { height: 260px; }
    .tabs-header { flex-wrap: wrap; }
    .connect-card { padding: 1.5rem; }
  }
</style>

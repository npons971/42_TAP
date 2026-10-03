<script>
  import { onMount, tick } from 'svelte';
  import RoomMap from './RoomMap.svelte';
  import Icon from './Icon.svelte';

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
  let nearbyTab = 'npc';
  let selectedEntity = '';
  let mapExpanded = false;

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
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
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
    nearbyTab = 'npc';
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
          if (currentRoom.id !== data.room.id) {
            selectedEntity = '';
            nearbyTab = data.npcs?.length ? 'npc' : data.items?.length ? 'item' : 'player';
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
      return { id: item.id, name: item.name || item.id, description: item.description || '', obtainable: item.obtainable !== false };
    });
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
  function selectMapEntity(kind, id) {
    nearbyTab = kind;
    selectedEntity = id;
  }
  function handleGameShortcut(event) {
    if (!connected || event.repeat || event.isComposing || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || activeDialogue || showGroupModal || mapExpanded) return;
    if (event.target?.closest?.('input, textarea, select, [contenteditable="true"], [role="dialog"]')) return;
    const action = {'1': callLook, '2': callStatus, '3': callQuests, '4': callWho, '5': () => showGroupModal = true}[event.key];
    if (action) { event.preventDefault(); action(); }
  }
</script>

<svelte:window on:keydown={handleGameShortcut} />

<main class="app-container" class:game-session={connected}>
  {#if !connected}
    <!-- CONNECTION SCREEN -->
    <div class="connect-overlay">
      <section class="welcome-panel" aria-label="The Answer Protocol">
        <div class="welcome-brand"><span class="brand-mark">42</span><span>THE ANSWER PROTOCOL</span></div>
        <div class="welcome-story">
          <span class="eyebrow">A SHARED WORLD. YOUR OWN PATH.</span>
          <h1>Every answer begins<br />with an adventure.</h1>
          <p>Wander unfamiliar streets, meet fellow travelers and uncover the stories waiting beyond the next turn.</p>
          <img class="welcome-compass" src="/compass.svg" alt="" />
        </div>
        <div class="welcome-footer"><span>EXPLORE · DISCOVER · CONNECT</span><span>A TEXT ADVENTURE BY 42</span></div>
      </section>
      <div class="connect-card">
        <div class="eyebrow">YOUR NEXT CHAPTER</div>
        <h2 class="connect-title">Step into the world.</h2>
        <p class="subtitle">Pick a name. Find your people. See where the road takes you.</p>

        <form class="connect-form" on:submit|preventDefault={handleConnect}>
          <div class="form-group username-field">
            <label for="username">Adventurer name</label>
            <input id="username" type="text" bind:value={username} placeholder="e.g. aris" required disabled={connecting} autocomplete="username" aria-describedby="username-hint" />
            <small id="username-hint">3–20 lowercase letters, digits or underscores.</small>
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
            <span>{connecting ? "Connecting…" : "Begin adventure"}</span><span aria-hidden="true">↗</span>
          </button>
        </form>
        <div class="connect-note"><span class="note-star" aria-hidden="true">✧</span> A world best discovered together.</div>
      </div>
    </div>
  {:else}
    <!-- ACTIVE GAME INTERFACE -->
    <header class="top-bar">
      <div class="brand">
        <span class="brand-mark">42</span>
        <div><span class="realm-title">THE ANSWER PROTOCOL</span><span class="server-address"><span class="pulse-dot"></span>{host}:{port}</span></div>
      </div>

      <div class="stats-badges">
        <span class="badge room-badge"><strong>{roomPlayersCount}</strong> in this room</span>
        <span class="badge server-badge"><span class="pulse-dot"></span><strong>{serverPlayersCount}</strong> online</span>
      </div>

      <button class="btn-disconnect" on:click={handleDisconnect}>Leave world <span aria-hidden="true">↗</span></button>
    </header>

    <div class="chapter-bar"><span class="eyebrow">WORLD EXPLORATION</span><span>Follow the paths. Discover what lies beyond.</span></div>
    {#if actionError}<div class="action-error" role="alert">{actionError}</div>{/if}
    <div class="game-grid">
      <!-- Map and local interactions are driven exclusively by LOOK data. -->
      <section class="panel room-panel">
        <div class="panel-header location-header">
          <div><span class="eyebrow">EXPLORATION</span><h2 class="room-name">{currentRoom.name}</h2></div>
          <span class="location-paths"><Icon name="map" size={16} /> {Object.keys(currentRoom.exits).length} paths</span>
        </div>
        <RoomMap room={currentRoom} names={roomNames} {username} inCombat={combatState === 'EN_COMBAT'} onmove={callMove} onselect={selectMapEntity} bind:expanded={mapExpanded} />
        <div class="location-caption"><Icon name="pin" size={18} /><p class="room-desc" title={currentRoom.description}>{currentRoom.description || 'Discovering your surroundings…'}</p></div>
        <div class="surroundings-header">
          <span class="eyebrow">NEARBY</span>
          <div class="nearby-tabs" aria-label="Surroundings">
            <button class:active={nearbyTab === 'npc'} aria-pressed={nearbyTab === 'npc'} on:click={() => nearbyTab = 'npc'}><Icon name="person" size={14} /> Characters <span class="count">{currentRoom.npcs.length}</span></button>
            <button class:active={nearbyTab === 'item'} aria-pressed={nearbyTab === 'item'} on:click={() => nearbyTab = 'item'}><Icon name="bag" size={14} /> Objects <span class="count">{currentRoom.items.length}</span></button>
            <button class:active={nearbyTab === 'player'} aria-pressed={nearbyTab === 'player'} on:click={() => nearbyTab = 'player'}><Icon name="users" size={14} /> Travelers <span class="count">{currentRoom.players.length}</span></button>
          </div>
        </div>
        <div class="nearby-content">
          {#if nearbyTab === 'npc'}
            {#each currentRoom.npcs as npc}
              <div class="nearby-card" class:selected={selectedEntity === npc.id}>
                <span class="nearby-icon" class:hostile={npc.role === 'enemy'}><Icon name={npc.role === 'enemy' ? 'sword' : npc.role === 'quest_giver' ? 'flag' : 'person'} size={22}/></span>
                <div class="nearby-detail"><strong>{npc.name}</strong><span>{npc.description || (npc.role === 'enemy' ? 'Hostile creature' : npc.role === 'quest_giver' ? 'Has a quest for you' : 'Available to talk')}</span></div>
                <div class="npc-buttons">
                  {#if npc.role !== 'enemy'}<button class="btn-action btn-talk" disabled={combatState === 'EN_COMBAT'} on:click={() => callTalk(npc.id)}>Talk <Icon name="arrow" size={12}/></button>{/if}
                  {#if npc.role === 'quest_giver' || npc.unknownRole}<button class="btn-action btn-take" disabled={combatState === 'EN_COMBAT'} on:click={() => callQuest(npc.id)}>Quest <Icon name="flag" size={12}/></button>{/if}
                  {#if npc.role === 'enemy' || npc.unknownRole}<button class="btn-action btn-attack" on:click={() => callAttack(npc.id)}>Attack <Icon name="sword" size={12}/></button>{/if}
                </div>
              </div>
            {:else}<div class="empty-list">No characters nearby. Choose a path to keep exploring.</div>{/each}
          {:else if nearbyTab === 'item'}
            {#each currentRoom.items as item}
              <div class="nearby-card" class:selected={selectedEntity === item.id}>
                <span class="nearby-icon"><Icon name={item.obtainable ? 'bag' : 'landmark'} size={22}/></span>
                <div class="nearby-detail"><strong>{item.name}</strong><span>{item.description || (item.obtainable ? 'You can pick this up' : 'Part of the surroundings')}</span></div>
                {#if item.obtainable}<button class="btn-action btn-take" disabled={combatState === 'EN_COMBAT'} on:click={() => callTake(item.id)}>Take <Icon name="bag" size={12}/></button>{:else}<span class="scenery-tag">Scenery</span>{/if}
              </div>
            {:else}<div class="empty-list">No objects to collect in this location.</div>{/each}
          {:else}
            {#each currentRoom.players as player}
              <div class="nearby-card"><span class="nearby-icon"><Icon name="person" size={22}/></span><div class="nearby-detail"><strong>{player}</strong><span>{player === username ? 'You · exploring this location' : 'Fellow adventurer'}</span></div><span class="player-dot"></span></div>
            {:else}<div class="empty-list">No other travelers in this location.</div>{/each}
          {/if}
        </div>
      </section>

      <!-- RIGHT COLUMN: PLAYER STATUS & INVENTORY -->
      <aside class="panel status-panel">
        <div class="panel-header">
          <h2><span class="section-number">02</span> ADVENTURER</h2>
          <span class="status-tag {combatState === 'EN_COMBAT' ? 'in-combat' : 'out-combat'}">
            {combatState === 'EN_COMBAT' ? 'In combat' : 'Exploring'}
          </span>
        </div>

        <div class="character-summary"><span class="avatar" aria-hidden="true">{username.slice(0, 1).toUpperCase()}</span><div><strong>{username}</strong><span>Your adventure is unfolding.</span></div></div>
        <div class="vitality-box">
          <div class="hp-header">
            <span>Vitality</span>
            <span class="hp-numbers">{playerHP} / {playerMaxHP} HP</span>
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
            <h3>Satchel <span class="count">{inventory.length}</span></h3>
            <button class="btn-tiny" on:click={callInventory} aria-label="Refresh inventory">↻</button>
          </div>
          <div class="inventory-list">
            {#each inventory as item}
              <div class="inventory-item">
                <span class="item-name">{item.name || item.id}</span>
                <div class="npc-buttons">
                  <button class="btn-action btn-talk" disabled={combatState === "EN_COMBAT"} on:click={() => callUse(item.id)}>Use</button>
                  <button class="btn-action btn-drop" disabled={combatState === "EN_COMBAT"} on:click={() => callDrop(item.id)}>Drop</button>
                </div>
              </div>
            {/each}
            {#if inventory.length === 0}
              <div class="empty-list">Traveling light.<br />Collected items will appear here.</div>
            {/if}
          </div>
        </div>

        <!-- ACTIVE QUESTS TRACKER -->
        <div class="quests-section">
          <div class="panel-header-sub">
            <h3>Quest journal <span class="count">{quests.length}</span></h3>
            <button class="btn-tiny" on:click={callQuests} aria-label="Refresh quests">↻</button>
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
              <div class="empty-list">A story waiting to be written.<br />Talk to characters to discover quests.</div>
            {/if}
          </div>
        </div>

        <!-- QUICK ACTION BUTTONS -->
        <div class="actions-section">
          <div class="hotbar-heading"><h3>QUICK ACTIONS</h3><span>1 — 5</span></div>
          <div class="action-buttons-grid">
            <button class="btn-quick" on:click={callLook} title="Inspect and refresh your surroundings (1)" aria-keyshortcuts="1"><kbd>1</kbd><Icon name="eye" size={22}/><span>Look</span></button>
            <button class="btn-quick" on:click={callStatus} title="Refresh your health and combat state (2)" aria-keyshortcuts="2"><kbd>2</kbd><Icon name="heart" size={22}/><span>Status</span></button>
            <button class="btn-quick" on:click={callQuests} title="Refresh your quest journal (3)" aria-keyshortcuts="3"><kbd>3</kbd><Icon name="book" size={22}/><span>Quests</span></button>
            <button class="btn-quick" on:click={callWho} title="Refresh the players in your room and online (4)" aria-keyshortcuts="4"><kbd>4</kbd><Icon name="users" size={22}/><span>Players</span></button>
            <button class="btn-quick party-action" on:click={() => showGroupModal = true} title="Create, join or manage your party (5)" aria-keyshortcuts="5"><kbd>5</kbd><Icon name="flag" size={22}/><span>Party</span></button>
          </div>
        </div>
      </aside>
    </div>

    <!-- BOTTOM ROW: CHAT & LOGS PANEL (MANDATORY CHAPTER V.4) -->
    <section class="panel chat-panel">
      <div class="tabs-header">
        <button class="tab-btn {activeTab === 'global' ? 'active' : ''}" on:click={() => activeTab = 'global'}>
          World <span class="count">{messages.global.length}</span>
        </button>
        <button class="tab-btn {activeTab === 'room' ? 'active' : ''}" on:click={() => activeTab = 'room'}>
          Room <span class="count">{messages.room.length}</span>
        </button>
        <button class="tab-btn {activeTab === 'group' ? 'active' : ''}" on:click={() => activeTab = 'group'}>
          Party <span class="count">{messages.group.length}</span>
        </button>
        <button class="tab-btn {activeTab === 'logs' ? 'active' : ''}" on:click={() => activeTab = 'logs'}>
          System logs <span class="count">{messages.logs.length}</span>
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
          <div class="empty-chat">{activeTab === 'logs' ? 'Your journey’s events will appear here.' : 'Every adventure starts with a hello. Say something.'}</div>
        {/if}
      </div>

      <div class="chat-input-bar">
        <span class="channel-indicator">#{activeTab === 'global' ? 'world' : activeTab === 'group' ? 'party' : activeTab}</span>
        <input
          type="text"
          aria-label="Chat message"
          disabled={activeTab === "logs"}
          placeholder={activeTab === "logs" ? "System logs — read only" : "Type message and press Enter..."}
          bind:value={chatInput}
          on:keydown={handleKeyDown}
        />
        <button class="btn-send" disabled={activeTab === "logs" || !chatInput.trim()} on:click={handleSendMessage}>Send <span aria-hidden="true">↗</span></button>
      </div>
    </section>

    <!-- NPC DIALOGUE MODAL -->
    {#if activeDialogue}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="NPC Dialogue">
          <div class="dialogue-header">
            <h3>{activeDialogue.npc}</h3>
            <button class="btn-close" aria-label="Close dialog" on:click={() => activeDialogue = null}>✕</button>
          </div>
          <p class="dialogue-speech">"{activeDialogue.text}"</p>
          <button class="btn-primary" on:click={() => activeDialogue = null}>Continue <span aria-hidden="true">→</span></button>
        </div>
      </div>
    {/if}

    <!-- GROUP MANAGEMENT MODAL -->
    {#if showGroupModal}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="Group Management">
          <div class="dialogue-header">
            <h3>Better together.</h3>
            <button class="btn-close" aria-label="Close party dialog" on:click={() => showGroupModal = false}>✕</button>
          </div>
          <p class="subtitle" style="margin-bottom: 1rem;">Create a party, invite a player, or enter a leader’s username to join.</p>
          {#if actionError}<div class="error-badge" role="alert">{actionError}</div>{/if}
          {#if groupStatus}<p class="group-status" role="status">{groupStatus}</p>{/if}
          <div class="form-group" style="margin-bottom: 1rem;">
            <label for="groupTarget">Adventurer name</label>
            <input id="groupTarget" type="text" bind:value={groupTarget} placeholder="e.g. aris" />
          </div>
          <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
            <button class="btn-action btn-talk" on:click={() => callGroup("CREATE")}>Create party</button>
            <button class="btn-action btn-talk" disabled={!groupTarget.trim()} on:click={() => callGroup("INVITE", groupTarget)}>Invite</button>
            <button class="btn-action btn-take" disabled={!groupTarget.trim()} on:click={() => callGroup("JOIN", groupTarget)}>Join party</button>
            <button class="btn-action btn-drop" on:click={() => callGroup("LEAVE", "")}>Leave party</button>
          </div>
        </div>
      </div>
    {/if}
  {/if}
</main>

<script>
  import { onMount } from 'svelte';

  // Connection State
  let connected = false;
  let connecting = false;
  let connectionError = "";
  let host = "127.0.0.1";
  let port = 4242;
  let username = "Novanns";

  // Player & Game State
  let playerHP = 100;
  let playerMaxHP = 100;
  let combatState = "HORS_COMBAT";
  let currentTarget = null;
  let roomPlayersCount = 1;
  let serverPlayersCount = 1;

  // Current Room State
  let currentRoom = {
    id: "loc.town_square",
    name: "Village Square",
    description: "A bustling cobblestone square bathed in gentle sunlight. In the center stands a weathered stone fountain. Connect to server to explore.",
    exits: { east: "Market", north: "Garden", west: "Dark Alley" },
    items: [],
    npcs: [],
    players: []
  };

  // Inventory & Quests
  let inventory = [];
  let quests = [];

  // NPC Dialogue Modal
  let activeDialogue = null; // { npc: "...", text: "..." }

  // Chat & Logs Tabs (Mandatory Subject requirement V.4)
  let activeTab = "global"; // "global" | "room" | "group" | "logs"
  let chatInput = "";
  let messages = {
    global: [],
    room: [],
    group: [],
    logs: []
  };

  // Helper for adding messages
  function addMessage(channel, sender, text, type = "normal") {
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    const msg = { time, sender, text, type };
    if (messages[channel]) {
      messages[channel] = [...messages[channel], msg];
    }
    // Also push system events or chat into logs
    if (channel !== 'logs') {
      messages.logs = [...messages.logs, { time, sender: `[${channel.toUpperCase()}] ${sender}`, text, type }];
    }
  }

  function addLog(text, type = "info") {
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    messages.logs = [...messages.logs, { time, sender: "SYSTEM", text, type }];
  }

  // Wails Bindings Safe Access
  function getBackend() {
    return window?.go?.main?.App;
  }

  onMount(() => {
    addLog("Client GUI initialized. Ready to connect to RFC 42TAP Server.", "info");

    if (window.runtime) {
      window.runtime.EventsOn("server_ok", (payload) => handleServerOK(payload));
      window.runtime.EventsOn("server_err", (payload) => handleServerERR(payload));
      window.runtime.EventsOn("server_evt", (payload) => handleServerEVT(payload));
      window.runtime.EventsOn("disconnected", (msg) => {
        connected = false;
        connecting = false;
        addLog(msg || "Disconnected from server.", "error");
      });
      window.runtime.EventsOn("server_error", (err) => {
        addLog(`Network error: ${err}`, "error");
      });
    }
  });

  // Server Payload Parsing
  function handleServerOK(payload) {
    if (!payload) return;
    addLog(`S: OK ${payload}`, "ok");

    if (payload === "connected") {
      connected = true;
      connecting = false;
      connectionError = "";
      addMessage("global", "SYSTEM", `Connected successfully as ${username}!`, "system");
      // Request initial room look, inventory, and status
      setTimeout(() => {
        callLook();
        callInventory();
        callStatus();
        callWho();
      }, 150);
      return;
    }

    // Try parsing JSON payloads (LOOK, STATUS, INVENTORY, WHO, QUESTS)
    if (payload.startsWith("{") || payload.startsWith("[")) {
      try {
        const data = JSON.parse(payload);

        // LOOK payload
        if (data.room) {
          currentRoom = {
            id: data.room.id || "unknown",
            name: data.room.name || "Unknown Room",
            description: data.room.description || "",
            exits: data.room.exits || {},
            items: normalizeItems(data.items || []),
            npcs: normalizeNPCs(data.npcs || []),
            players: data.players || []
          };
          roomPlayersCount = (data.players ? data.players.length : 1);
          return;
        }

        // STATUS payload
        if (data.player !== undefined && data.hp !== undefined) {
          playerHP = data.hp;
          playerMaxHP = data.max_hp || 100;
          combatState = data.state || "HORS_COMBAT";
          currentTarget = data.combat || null;
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
          if (data.length === 0) {
            inventory = [];
          } else if (data[0].title || data[0].giver) {
            quests = data;
          } else {
            inventory = normalizeItems(data);
          }
          return;
        }

        // TALK payload
        if (data.npc && data.dialogue) {
          activeDialogue = { npc: data.npc, text: data.dialogue };
          addMessage("room", data.npc, data.dialogue, "dialogue");
          return;
        }
      } catch (e) {
        console.warn("Could not parse JSON OK payload:", e);
      }
    }

    // Key-value OK responses: OK room=loc.market, OK taken=item.apple, etc.
    if (payload.startsWith("room=")) {
      // Room changed, refresh look
      callLook();
    } else if (payload.startsWith("taken=") || payload.startsWith("dropped=")) {
      // Item action confirmed, refresh room & inventory
      callLook();
      callInventory();
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
        return { id: npc, name: cleanName, role: 'dialogue' };
      }
      return { id: npc.id, name: npc.name || npc.id, role: npc.role || 'dialogue' };
    });
  }

  function handleServerERR(payload) {
    addLog(`S: ERR ${payload}`, "error");
    addMessage("room", "SERVER", `Error: ${payload}`, "error");
  }

  function handleServerEVT(payload) {
    addLog(`S: EVT ${payload}`, "event");
    // Format: EVT <SCOPE> <EVENT_TYPE> [DATA...]
    const parts = payload.split(" ");
    const scope = (parts[0] || "").toLowerCase();
    const evtType = parts[1] || "";
    const rest = parts.slice(2).join(" ");

    if (evtType === "CHAT") {
      const sender = parts[2] || "someone";
      const text = parts.slice(3).join(" ");
      addMessage(scope, sender, text, "chat");
    } else if (evtType === "PRESENCE") {
      const action = parts[2] || ""; // ENTER or LEAVE
      const targetUser = parts[3] || "";
      addMessage("room", "ROOM", `${targetUser} ${action === "ENTER" ? "entered" : "left"} the room.`, "presence");
      callLook();
    } else if (evtType === "COMBAT") {
      addMessage("room", "COMBAT", rest, "combat");
      callStatus();
    } else if (evtType === "ITEM") {
      addMessage("room", "ROOM", rest, "item");
      callLook();
      if (parts[3] === username.trim() && ["USE", "DELIVER", "REWARD"].includes(parts[2])) {
        callInventory();
        callStatus();
      }
    }
  }

  // User Actions
  async function handleConnect() {
    connecting = true;
    connectionError = "";
    const backend = getBackend();
    if (!backend) {
      connectionError = "Wails backend not available";
      connecting = false;
      return;
    }
    try {
      await backend.Connect(host, parseInt(port, 10), username);
    } catch (err) {
      connectionError = err.toString();
      connecting = false;
      addLog(`Connection failed: ${err}`, "error");
    }
  }

  async function handleDisconnect() {
    const backend = getBackend();
    if (backend) {
      await backend.Disconnect();
    }
    connected = false;
  }

  async function callMove(direction) {
    const backend = getBackend();
    if (backend) await backend.Move(direction);
  }

  async function callLook() {
    const backend = getBackend();
    if (backend) await backend.Look();
  }

  async function callTake(itemId) {
    const backend = getBackend();
    if (backend) await backend.Take(itemId);
  }

  async function callDrop(itemId) {
    const backend = getBackend();
    if (backend) await backend.Drop(itemId);
  }

  async function callTalk(npcId) {
    const backend = getBackend();
    if (backend) await backend.Talk(npcId);
  }

  async function callAttack(npcId) {
    const backend = getBackend();
    if (backend) await backend.Attack(npcId);
  }

  async function callInventory() {
    const backend = getBackend();
    if (backend) await backend.Inventory();
  }

  async function callStatus() {
    const backend = getBackend();
    if (backend) await backend.Status();
  }

  async function callQuests() {
    const backend = getBackend();
    if (backend) await backend.Quests();
  }

  async function callWho() {
    const backend = getBackend();
    if (backend) await backend.Who();
  }

  async function handleSendMessage() {
    const text = chatInput.trim();
    if (!text) return;
    const backend = getBackend();
    if (backend) {
      const channel = activeTab === "logs" ? "GLOBAL" : activeTab.toUpperCase();
      await backend.Chat(channel, text);
      chatInput = "";
    }
  }

  function handleKeyDown(e) {
    if (e.key === "Enter") {
      handleSendMessage();
    }
  }
</script>

<main class="app-container">
  {#if !connected}
    <!-- CONNECTION SCREEN -->
    <div class="connect-overlay">
      <div class="connect-card">
        <div class="retro-badge">42 CURRICULUM PROJECT</div>
        <h1 class="glow-title">THE ANSWER PROTOCOL</h1>
        <p class="subtitle">Multiplayer Retro Text Adventure Client</p>

        <form class="connect-form" on:submit|preventDefault={handleConnect}>
          <div class="form-group">
            <label for="host">SERVER HOST</label>
            <input id="host" type="text" bind:value={host} placeholder="127.0.0.1" required />
          </div>

          <div class="form-group">
            <label for="port">PORT</label>
            <input id="port" type="number" bind:value={port} placeholder="4242" required />
          </div>

          <div class="form-group">
            <label for="username">ADVENTURER USERNAME</label>
            <input id="username" type="text" bind:value={username} placeholder="Novanns" required maxlength="16" />
          </div>

          {#if connectionError}
            <div class="error-badge">{connectionError}</div>
          {/if}

          <button class="btn-primary" type="submit" disabled={connecting}>
            {connecting ? "CONNECTING..." : "ENTER THE REALM"}
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
                <button class="btn-exit" on:click={() => callMove(dir)}>
                  <span class="dir-tag">{dir.toUpperCase()}</span>
                  <span class="dest-tag">{dest}</span>
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
                    <button class="btn-action btn-take" on:click={() => callTake(item.id)}>TAKE</button>
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
                    <button class="btn-action btn-talk" on:click={() => callTalk(npc.id)}>TALK</button>
                    <button class="btn-action btn-attack" on:click={() => callAttack(npc.id)}>ATTACK</button>
                  </div>
                </div>
              {/each}
              {#if currentRoom.npcs.length === 0}
                <div class="empty-list">No one is here</div>
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

        <div class="inventory-section">
          <div class="panel-header-sub">
            <h3>INVENTORY ({inventory.length})</h3>
            <button class="btn-tiny" on:click={callInventory}>REFRESH</button>
          </div>
          <div class="inventory-list">
            {#each inventory as item}
              <div class="inventory-item">
                <span class="item-name">{item.name || item.id}</span>
                <button class="btn-action btn-drop" on:click={() => callDrop(item.id)}>DROP</button>
              </div>
            {/each}
            {#if inventory.length === 0}
              <div class="empty-list">Backpack is empty</div>
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

      <div class="chat-viewport">
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
          placeholder="Type message and press Enter..."
          bind:value={chatInput}
          on:keydown={handleKeyDown}
        />
        <button class="btn-send" on:click={handleSendMessage}>SEND</button>
      </div>
    </section>

    <!-- NPC DIALOGUE MODAL -->
    {#if activeDialogue}
      <div class="modal-backdrop">
        <div class="dialogue-card" role="dialog" aria-modal="true" aria-label="NPC Dialogue">
          <div class="dialogue-header">
            <h3>{activeDialogue.npc} SPEAKS</h3>
            <button class="btn-close" on:click={() => activeDialogue = null}>✕</button>
          </div>
          <p class="dialogue-speech">"{activeDialogue.text}"</p>
          <button class="btn-primary" on:click={() => activeDialogue = null}>CONTINUE</button>
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
    overflow: hidden;
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
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem;
    padding: 0.8rem;
    border-top: 1px solid #21262d;
    flex: 1;
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
  .status-panel { padding: 0.8rem; gap: 0.8rem; }
  .status-tag { font-size: 0.75rem; font-weight: 700; padding: 2px 6px; border-radius: 4px; }
  .status-tag.in-combat { background: #b62324; color: #fff; }
  .status-tag.out-combat { background: #238636; color: #fff; }

  .hp-header { display: flex; justify-content: space-between; font-size: 0.8rem; font-weight: 700; margin-bottom: 0.3rem; }
  .hp-bar-bg { background: #21262d; border-radius: 4px; height: 12px; overflow: hidden; border: 1px solid #30363d; }
  .hp-bar-fill { background: linear-gradient(90deg, #da3633, #238636); height: 100%; transition: width 0.3s; }

  .inventory-section { flex: 1; display: flex; flex-direction: column; min-height: 0; }
  .panel-header-sub { display: flex; justify-content: space-between; align-items: center; }
  .panel-header-sub h3 { margin: 0 0 0.4rem; font-size: 0.8rem; color: #8b949e; }
  .inventory-list { background: #0d1117; border: 1px solid #21262d; border-radius: 4px; padding: 0.4rem; overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: 0.3rem; }
  .inventory-item { display: flex; justify-content: space-between; align-items: center; padding: 0.3rem 0.5rem; background: #161b22; border-radius: 3px; font-size: 0.85rem; }

  .actions-section h3 { margin: 0 0 0.4rem; font-size: 0.8rem; color: #8b949e; }
  .action-buttons-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.4rem; }
  .btn-quick { background: #21262d; border: 1px solid #30363d; color: #c9d1d9; padding: 0.45rem; font-size: 0.75rem; font-weight: 700; border-radius: 4px; cursor: pointer; }
  .btn-quick:hover { border-color: #58a6ff; color: #fff; }

  /* CHAT PANEL */
  .chat-panel { margin: 0 0.75rem 0.75rem; height: 210px; }
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
  .dialogue-speech { font-size: 1rem; line-height: 1.5; color: #e3b341; font-style: italic; margin-bottom: 1.5rem; }
</style>

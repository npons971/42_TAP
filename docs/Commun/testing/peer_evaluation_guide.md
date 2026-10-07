# 42 Peer-Evaluation & Defense Guide

> **Document Status**: `🔵 IN_REVIEW`  
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

This document is a practical playbook for presenting the project during 42 peer-evaluation and successfully passing the live code modification test specified in Chapter VII.2 of the subject.

---

## 1. Pre-Evaluation Verification Checklist

Before starting the evaluation session with your peers:
- [ ] No Python files in the repository (strict automatic fail rule).
- [ ] Language is C, C++, Rust, Go, or Zig (Go for this project).
- [ ] `README.md` at git root is written in English, includes login notice on line 1, and contains all required sections.
- [ ] Clean build via project targets: `make`, `make lint`, `make test`.

---

## 2. Step-by-Step Peer Evaluation Script

Follow this precise sequence to demonstrate all mandatory features efficiently:

### Step 1: Boot the Server & Verify World Loading
1. Start the server in a terminal:
   ```bash
   make run-server WORLD_FILE=data/world.json
   ```
2. Point out structured JSON logs showing startup, world loading, and port listening (`:4242`).
3. Show that world integrity check passed (exits valid, NPCs valid, items valid).

### Step 2: Connect Both Clients (CLI + GUI)
1. In Terminal 2, launch CLI client:
   ```bash
   ./bin/client-cli --connect 127.0.0.1:4242 --name alice
   ```
2. In Terminal 3, launch GUI client:
   ```bash
   ./bin/client-gui --connect 127.0.0.1:4242 --name bob
   ```
3. Demonstrate real-time mutual presence: both players see each other in `loc.town_square` via `LOOK` and GUI player counter (`room: 2, server: 2`).

### Step 3: Exploration & World Topology
1. Move `alice` around the main circuit:
   `MOVE east` (Market) $\rightarrow$ `MOVE north` (Tavern) $\rightarrow$ `MOVE west` (Alley) $\rightarrow$ `MOVE south` (Square).
2. Prove that a full circuit loop exists (no line-only map).
3. Move into the optional branch (`loc.ruins_den` via `loc.ruins_gate`).

### Step 4: Item Dynamics (Unicité & Multi-Word Names)
1. In `loc.tavern`, pick up the ale: `TAKE Frothy Ale` (or `item.ale`).
2. Show that the item disappears from room view immediately on both CLI and GUI.
3. Attempt to pick it up with `bob`: verify `ERR 404 ITEM_NOT_FOUND`.
4. Drop it with `alice`: `DROP Frothy Ale`.
5. Show that `bob`'s GUI automatically reflects the reappearance of the item.

### Step 5: Chat Channels Isolation
1. `CHAT GLOBAL Hello all`: both receive it.
2. `alice` moves to Market; `bob` remains in Square.
3. `alice` sends `CHAT ROOM Secret here`: verify `bob` does **not** receive it.

### Step 6: Combat & Respawn Mechanics
1. Enter `loc.sewers` with `bob` (rat enemy) or `loc.ruins_den` (bandit enemy).
2. Issue `ATTACK <enemy>`.
3. Show turn-based damage exchange, counter-attack, and `STATUS` readout.
4. Continue combat until player HP reaches 0:
   - Prove player is broadcast as dead.
   - Prove player immediately respawns at `loc.town_square` with reduced HP (30 HP).

### Step 7: Quests Completion
1. Show `QUESTS` list.
2. Complete Quest 1 (Rare Herbs) or Quest 2 (Bandit).
3. Demonstrate reward distribution and quest status update.

### Step 8: Logging Audit
1. Show the server terminal logs:
   - Timestamps and IP addresses recorded.
   - Command parameters and return codes.
   - State mutations and combat logs.

---

## 3. Preparation for Live Code Modification (Chap VII.2) `🔴 TO_FILL`

The subject states that during evaluation, peers may ask you to make a minor code change in a few minutes (e.g., adding an item, a command, or modifying a formula).

### Quick-Win Modification Points to Know:
1. **Adding a new item or room**:
   - Location: `data/world.json`. Edit JSON and restart the server. `Project/data/world.yaml` is the original design snapshot.
2. **Adding a simple command (e.g., `PING` -> `PONG`)**:
   - Location: router dispatch table (`server/router.go`). Add one route and handler function.
3. **Modifying combat damage formula**:
   - Location: `engine/combat.go`. Change the base damage multiplier or `DEFEND` reduction percentage.

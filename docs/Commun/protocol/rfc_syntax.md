# RFC 42TAP — Protocol Syntax & Message Framing

> **Document Status**: `🔵 IN_REVIEW`  
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

---

## 1. Network Framing & Encoding

- **Transport**: TCP sockets.
- **Encoding**: UTF-8 exclusively.
- **Framing**: Line-oriented protocol. Every message sent by client or server MUST terminate with a single newline character `\n` (`LF`, byte `0x0A`).
- **Maximum Line Length**: `🔴 TO_FILL (Decision Pending)` — Recommended: 4096 bytes per message to protect against memory exhaustion.

```text
+------------------------------------------------------+------+
|                  UTF-8 Payload Text                  | '\n' |
+------------------------------------------------------+------+
```

---

## 2. Command Grammar & Message Types

Messages between client and server belong to three categories:

### 2.1 Client Request
Sent from client to server:
```text
COMMAND [ARGUMENTS...]\n
```
* **Command verbs**: uppercase ASCII words (e.g., `CONNECT`, `LOOK`, `MOVE`, `CHAT`, `TAKE`, `DROP`, `ATTACK`).
* **Arguments**: space-separated tokens. Strings with spaces must be handled cleanly.

### 2.2 Server Synchronous Reply
Sent immediately in response to a client command:
- **Success (`OK`)**:
  ```text
  OK [payload]\n
  ```
  *Examples*: `OK connected`, `OK taken=item.ale`, `OK {"room": {...}}`
- **Error (`ERR`)**:
  ```text
  ERR <error_code> [descriptive message]\n
  ```
  *Example*: `ERR item_not_found The item is not in this room`

### 2.3 Server Asynchronous Event (`EVT`)
Pushed unprompted to clients based on world activity:
```text
EVT <SCOPE> <EVENT_TYPE> [EVENT_DATA...]\n
```
- **Scope**:
  - `GLOBAL`: Received by all connected players.
  - `ROOM`: Received by players in the relevant room.
  - `GROUP`: Received by members of the sender's group.
- *Examples*:
  - `EVT GLOBAL CHAT alice Hello adventurers!`
  - `EVT ROOM PRESENCE ENTER bob`
  - `EVT ROOM COMBAT alice ATTACK goblin DAMAGE=12`

---

## 3. Mandatory Protocol Commands

| Verb | Syntax | Response | Description |
|---|---|---|---|
| `CONNECT` | `CONNECT <username>` | `OK connected` / `ERR` | Logs into server with unique username |
| `LOOK` | `LOOK` | `OK <json_room_state>` | Inspects current room details |
| `MOVE` | `MOVE <direction>` | `OK room=<room_id>` / `ERR` | Moves player (`north`, `south`, `east`, `west`, etc.) |
| `CHAT` | `CHAT <channel> <message>` | `OK` / `ERR` | Sends chat (`GLOBAL`, `ROOM`, `GROUP`) |
| `TAKE` | `TAKE <item_identifier>` | `OK taken=<item_id>` / `ERR` | Picks up item from current room |
| `DROP` | `DROP <item_identifier>` | `OK dropped=<item_id>` / `ERR` | Drops item from inventory into room |
| `INVENTORY`| `INVENTORY` | `OK <json_inventory>` | Lists items held by player |
| `TALK` | `TALK <npc_identifier>` | `OK <json_dialogue>` | Interacts with an NPC in room |
| `ATTACK` | `ATTACK <npc_identifier>` | `OK <combat_result>` / `ERR` | Attacks an enemy NPC |
| `STATUS` | `STATUS` | `OK <json_status>` | Checks player health and combat state |
| `QUEST` | `QUEST <quest_id>` | `OK <json_quest_details>` | Queries active quest progress |
| `QUESTS` | `QUESTS` | `OK <json_quest_list>` | Lists all available/active quests |
| `WHO` | `WHO` | `OK <json_who>` | Lists players in room and total server count |
| `GROUP` | `GROUP <action> [args]` | `OK` / `ERR` | Manages party (`CREATE`, `JOIN`, `LEAVE`) |
| `QUIT` | `QUIT` | Connection closed | Gracefully disconnects from server |

---

## 4. Standardized Error Codes `🔵 IN_REVIEW`

To ensure interoperability between groups, the server emits standardized error codes:

| Error Code | Trigger Condition |
|---|---|
| `ERR unknown_command` | Unrecognized command verb |
| `ERR not_authenticated` | Attempting actions before sending `CONNECT` |
| `ERR already_authenticated`| Sending `CONNECT` when already logged in |
| `ERR username_taken` | Requested username is already in use by another player |
| `ERR invalid_arguments` | Missing or malformed parameters |
| `ERR invalid_direction` | Direction is not a valid exit from current room |
| `ERR item_not_found` | Item does not exist in room or inventory |
| `ERR item_not_obtainable` | Item exists in room but cannot be picked up (`obtainable: false`) |
| `ERR target_not_found` | Specified NPC is not present in the room |
| `ERR cannot_attack` | Target NPC is not an enemy / cannot be attacked |
| `ERR in_combat` | Action disallowed while engaged in combat |
| `ERR not_in_combat` | Combat action attempted while outside combat |
| `ERR rate_limited` | Command rejected due to flooding / rate-limiting |

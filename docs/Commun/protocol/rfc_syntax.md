# RFC 42TAP — Implemented protocol

Source: [external RFC 42TAP, December 2024](external_rfc.html).
Updated: 2026-10-07. The standard commands follow the RFC; project extensions
are listed separately. [JSON schemas](json_payloads.md).

## Transport and lifecycle

- TCP, UTF-8, one LF (`0x0A`) per message. CR, invalid UTF-8 and terminal control
  characters are rejected. Tabs receive a command-specific argument error.
- Maximum **4,096 bytes including LF**, in both directions. Count bytes, not
  characters. Oversized requests are drained through LF; the next command
  remains readable. Oversized responses produce `413 RESPONSE_TOO_LARGE`
  without committing the requested mutation.
- The RFC recommends 1,024 bytes. We choose 4,096 to fit detailed room and
  dialogue responses. This is a documented choice under a recommendation.
- Greeting: `OK hello proto=1`. Authenticate with `CONNECT` before playing.
- Commands are case-insensitive. CHAT scopes and GROUP actions are also
  case-insensitive. Directions and resource IDs remain lowercase.
- Every command produces one `OK` or `ERR`. `QUIT` sends `OK bye`, cleans up
  player state and closes the connection, including before authentication.
- Replies preserve request order. Events may occur between replies. Clients
  must read them while waiting; the GUI associates replies with its outgoing
  command queue. A reply is queued before the events caused by that action.
- Limits: 128 simultaneous TCP connections, 32 held items per player, 128 queued
  outgoing lines per connection, 5-second write timeout. All commands, including
  malformed commands, share a 128-command burst and 20 commands/second refill.
  Flooding sends `429 RATE_LIMITED` and closes the connection.

The CONNECT ABNF in §3.1 mentions CRLF, while the transport and general grammar
explicitly specify LF. This implementation follows the general LF framing.

## Standard commands

| Command | Success response |
|---|---|
| `CONNECT alice` | `OK connected` |
| `LOOK` | `OK {"room":{...},"players":[...],"items":["item.apple"],"npcs":["npc.merchant"]}` |
| `MOVE north` | `OK room=loc.garden` |
| `QUIT` | `OK bye`, then EOF |
| `CHAT GLOBAL Hello` | `OK`, then chat events; ROOM and GROUP are also supported |
| `WHO` | `OK players=2` (authenticated players on the server) |
| `GROUP CREATE` | `OK group=group.1` |
| `GROUP INVITE bob` | `OK`; bob receives `EVT GROUP INVITE alice` |
| `GROUP JOIN alice` | `OK group=group.1`; argument is the connected leader's username |
| `GROUP LEAVE` | `OK` |
| `TAKE Fresh Apple` | `OK taken=item.apple` |
| `DROP item.apple` | `OK dropped=item.apple` |
| `INVENTORY` | `OK ["item.apple"]` |
| `TALK Village Guard` | `OK <dialogue text>` (one line) |
| `ATTACK npc.bandit_leader` | `OK <combat JSON>` |
| `STATUS` | `OK <status JSON>` containing hp, max_hp and status |
| `QUEST Village Herbalist` | `OK <quest JSON>`; accepts a quest from a local NPC |
| `QUESTS` | `OK <array of the player's active/completed quests>` |

GROUP IDs are generated and unique during the server lifetime. Any member can
invite; invitations are notifications, not mandatory authorizations for joining
(the RFC does not require invitation-only groups). JOIN targets the current
leader. When the leader leaves or disconnects, the lexicographically first
remaining username becomes leader. Empty groups are deleted. Membership lasts
until LEAVE or disconnect and works across rooms.

QUEST starts a quest only after a bounded successful reply is queued. Repeated
requests return status `active`. Missing, completed or globally claimed rewards
return `406 NO_QUEST_AVAILABLE`. TALK retains acceptance and objective validation
as an extension: talk at the giver to deliver an ingredient or report a victory.
QUESTS omits unstarted definitions; a started quest whose unique reward another
player claimed remains visible with extension status `unavailable`.

## Events

```text
EVT ROOM PRESENCE ENTER bob
EVT ROOM PRESENCE LEAVE bob
EVT ROOM CHAT alice Hello room
EVT GLOBAL CHAT alice Hello everyone
EVT GROUP INVITE alice
EVT GROUP JOIN bob
EVT GROUP LEAVE bob
EVT GROUP CHAT alice Hello group
EVT STATS players=2
```

Presence reaches the other players in the affected room. Chat reaches all
members of its scope, including the sender. INVITE reaches only the invitee;
JOIN reaches group members including the new member; explicit LEAVE reaches
all members including the departing player. On disconnect only remaining
members receive LEAVE. STATS reaches authenticated players on CONNECT and
disconnect, after the response/presence notifications.

Project event extensions:

```text
EVT GROUP LEADER bob
EVT ROOM ITEM TAKE alice item.apple
EVT ROOM ITEM DROP alice item.apple
EVT ROOM ITEM USE alice item.apple
EVT ROOM ITEM DELIVER alice item.rare_herbs
EVT ROOM ITEM REWARD alice item.vigor_potion
EVT ROOM COMBAT <combat JSON>
```

Held items return to the disconnecting player's room; ITEM DROP events precede
PRESENCE LEAVE and STATS. Consumed items stay consumed. Combat events reach the
old room, including the actor; movement/respawn presence events follow.

## Identifiers

- World IDs: lowercase ASCII, at most 64 bytes,
  `^(loc|item|npc|quest)\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`.
  Examples: `loc.town_square`, `item.apple_1`, `npc.guard`, `quest.herbal_cure`.
- One ID per physical item instance; moving, dropping or rewarding it never
  creates a copy or changes its ID. Reward items start in reserve.
- Usernames: 3–20 Unicode codepoints, first character a lowercase letter, then
  lowercase letters, decimal digits or underscore:
  `^[\p{Ll}][\p{Ll}\p{Nd}_]{2,19}$`. `aris` and `élise` work; `ARIS` is rejected.
  Names are compared exactly; no Unicode normalization is performed.
- TAKE/DROP/USE/TALK/ATTACK/QUEST accept an exact ID or full display name,
  without quotes. Names are matched case-insensitively in the local room or
  inventory. Exact IDs take precedence; ambiguous names require an ID.
- Directions: `north`, `south`, `east`, `west`, `up`, `down`. A MOVE error lists
  the current room's sorted available directions (or `none`).

## Errors

Format: `ERR <three-digit code> <stable token> <command-specific explanation>`.
The numeric code and token identify the error; explanation text is for users.

```text
ERR 201 NAME_IN_USE Username "alice" is already connected
ERR 301 NO_EXIT No down exit from loc.town_square; available directions: east, north, west
ERR 400 INVALID_ARGUMENTS Usage: CONNECT <username>
ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before WHO
ERR 404 NPC_NOT_FOUND QUEST NPC is not in your room; use LOOK
ERR 406 NO_QUEST_AVAILABLE QUEST NPC has no available quest or its unique reward has already been awarded
```

Standard server errors: `201 NAME_IN_USE`, `301 NO_EXIT`, `401 NOT_IN_GROUP`,
`402 ALREADY_IN_GROUP`, `404 ITEM_NOT_FOUND`, `404 ITEM_NOT_IN_INVENTORY`,
`404 NPC_NOT_FOUND`, `405 NPC_NOT_HOSTILE`, `406 NO_QUEST_AVAILABLE`.
RFC client-side network codes are `900 CONNECTION_FAILED` and `901 SEND_FAILED`.

Project error extensions:

| Code | Tokens |
|---|---|
| 400 | INVALID_ARGUMENTS, UNKNOWN_COMMAND |
| 403 | NOT_AUTHENTICATED |
| 404 | GROUP_NOT_FOUND, PLAYER_NOT_FOUND, QUEST_NOT_FOUND |
| 405 | ITEM_NOT_OBTAINABLE, ITEM_NOT_USABLE |
| 409 | ALREADY_AUTHENTICATED, IN_COMBAT, NOT_IN_COMBAT, TARGET_BUSY, HEALTH_FULL, INVENTORY_FULL |
| 413 | RESPONSE_TOO_LARGE |
| 429 | RATE_LIMITED |
| 500 | INTERNAL_ERROR |
| 503 | CONNECTION_LIMIT |

All recoverable command errors preserve the connection. A rejected TAKE,
failed reward delivery or USE at full HP preserves inventory and quest state.

## Command extensions

- `LOOK DETAILS`: descriptors `{id,name,obtainable}` for items and `{id,name,role}`
  for NPCs, while standard LOOK keeps ID arrays.
- `INVENTORY DETAILS`: descriptors `{id,name}`.
- `WHO DETAILS`: `{"room":["alice"],"server":2}`.
- `TALKJSON <npc>`: `{"npc":"npc.guard","dialogue":"..."}`; preserves ordered
  dialogue lines as escaped newlines. Standard TALK joins them onto one line.
- `QUESTINFO <quest_id>`: read-only detailed definition and current progress.
- `QUESTS DETAILS`: read-only catalogue with all definitions and internal states
  NOT_STARTED, IN_PROGRESS, OBJECTIVES_MET, COMPLETED, UNAVAILABLE.
- `USE <item>`: consumes an owned healing item, then `OK used=<id> hp=<hp>/100`.
- `DEFEND`: halves the current enemy riposte.
- `FLEE [direction]`: 70% success; omitted direction selects the first sorted exit.

The GUI requests LOOK DETAILS, INVENTORY DETAILS and TALKJSON for its labels and
NPC modal. It uses standard WHO and QUESTS, and falls back to standard LOOK/INVENTORY/TALK
when a server refuses metadata extensions. The CLI accepts all standard commands
and extensions. Detailed combat and quest design choices are justified in README.

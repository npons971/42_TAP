# RFC 42TAP — Protocol Syntax & Message Framing

> **Document Status**: `🔵 IN_REVIEW (Dev A proposal; Dev B review pending)`
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

This is Dev A's proposed wire contract for team review. The attached RFC 42TAP in the subject takes precedence wherever it prescribes a different exact format. Any agreed deviation must be recorded in the root README.

## 1. Transport and framing

- TCP, UTF-8, one message per line. Every client and server line ends with exactly one LF byte (`0x0A`); CR and embedded newlines are invalid in commands. JSON strings encode line breaks as `\n` inside the JSON value.
- **Maximum line length: 4,096 bytes including LF**, in either direction. This counts encoded UTF-8 bytes, not characters. The payload before LF can occupy at most 4,095 bytes.
- The server rejects an overlong or invalid UTF-8 request with `ERR invalid_arguments Message too long` or `ERR invalid_arguments Invalid UTF-8`. It consumes the rest of that request through LF before parsing the next one. Repeated abuse may trigger `ERR rate_limited` and disconnection.
- The server must serialize compact JSON without literal line breaks and keep every response and event within the same 4,096-byte limit. World data and player/item capacity must be checked so state responses can fit; if they cannot, the data, capacity or limit must change before release. Reject a `CHAT` request with `ERR invalid_arguments Message too long` if its resulting event would exceed the limit. Do not silently truncate messages or JSON.
- Immediately after a TCP connection, the server sends `OK hello proto=42TAP/1`. `42TAP/1` is the team's proposed version string; confirm it against the attached RFC before implementation.

## 2. Message grammar and lifecycle

```text
request = VERB [SP arguments] LF
reply   = "OK" [SP payload] LF / "ERR" SP error_code [SP message] LF
event   = "EVT" SP scope SP event_type [SP event_data] LF
scope   = "GLOBAL" / "ROOM" / "GROUP"
```

- Verbs, scopes, event types, and group actions are uppercase ASCII. Directions and error codes are lowercase ASCII. Literal SP means one ASCII space in the fixed fields; free-text chat and names may contain spaces.
- Each request other than `QUIT` produces exactly one `OK` or `ERR`. `QUIT` closes the connection after cleanup and has no reply, matching the current command table.
- Clients should send one request at a time and wait for its reply before sending the next. There are no request IDs. An unrelated `EVT` can arrive while a client waits; clients distinguish replies by the `OK`/`ERR` prefix.
- For an action that produces events for its sender, queue the sender's reply before its related event. A single writer per connection preserves line order. Other players' unrelated events may still arrive between requests and replies.
- `OK <json>` has one ASCII space after `OK` and contains one complete JSON value. The JSON schemas are in [json_payloads.md](json_payloads.md). Clients parse error **codes**; the descriptive text is for people and can change.

## 3. Commands and successful replies

| Request | Success reply | Notes |
|---|---|---|
| `CONNECT alice` | `OK connected` | Username must be unique among connected players. |
| `LOOK` | `OK {"room":...,"players":...,"items":...,"npcs":...}` | Detailed item and NPC descriptors. |
| `MOVE north` | `OK room=loc.garden` | Room ID is the destination. |
| `CHAT ROOM Hello everyone` | `OK` | Channel is `GLOBAL`, `ROOM`, or `GROUP`; rest of line is the message. |
| `TAKE item.apple` | `OK taken=item.apple` | ID or full display name may be supplied. |
| `DROP item.apple` | `OK dropped=item.apple` | ID or full display name may be supplied. |
| `INVENTORY` | `OK [{"id":"item.apple","name":"Fresh Apple"}]` | Detailed item descriptors. |
| `TALK npc.guard` | `OK {"npc":"npc.guard","dialogue":"Stay safe."}` | JSON object. |
| `ATTACK npc.giant_rat` | `OK <combat_result>` | Combat result schema remains to be specified with combat rules. |
| `STATUS` | `OK <json_status>` | See JSON schema. |
| `QUEST quest.herbal_cure` | `OK <json_quest_details>` | See JSON schema. |
| `QUESTS` | `OK <json_quest_list>` | See JSON schema. |
| `WHO` | `OK {"room":["alice","bob"],"server":2}` | `server` is the count of connected players. |
| `GROUP CREATE team_one` | `OK` | Also `GROUP JOIN <name>` and `GROUP LEAVE`; exact group errors remain to be specified. |
| `QUIT` | Connection closed | Remove player before broadcasting departure. |

The exact shapes of `LOOK` and `INVENTORY` differ from the subject's illustrative ID-array examples. Confirm this choice against the attached RFC and document it as a deviation if needed.

## 4. Events

The fixed event fields are positional. Chat message text is the remainder of the line after the username and one space. A sender receives its own chat event so both clients can use events to populate chat history.

```text
EVT GLOBAL CHAT alice Hello everyone
EVT ROOM CHAT bob I found the key
EVT GROUP CHAT alice Meet at the gate
EVT ROOM PRESENCE ENTER bob
EVT ROOM PRESENCE LEAVE bob
EVT ROOM ITEM TAKE bob item.apple
EVT ROOM ITEM DROP bob item.apple
```

- Chat recipients are the connected players in the specified scope, including the sender.
- On `MOVE`, send `LEAVE` to remaining players in the old room and `ENTER` to players already in the new room. On disconnect, remove the player first, then send `LEAVE` to remaining room occupants.
- `ITEM TAKE` and `ITEM DROP` are proposed concrete event forms for live GUI updates. Send them to all players in the room, including the actor, after the actor's `OK` reply is queued. They carry the actor username and the unique item ID. Clients may issue `LOOK` or `INVENTORY` when they need refreshed descriptors.
- Combat and quest events must be specified alongside their mechanics; do not invent incompatible wire formats in either client.

## 5. Identifiers and argument resolution

- World IDs are stable, globally unique, lowercase ASCII and match `^(loc|item|npc|quest)\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`, with at most 64 bytes total.
- Prefixes identify the entity kind: `loc.town_square`, `item.rusty_sword`, `npc.guard_captain`, `quest.herbal_cure`.
- Each physical item instance has its own ID. If two apples exist, use `item.apple_1` and `item.apple_2`; moving an item never changes its ID. Never reuse a live instance ID for another item.
- Display names are separate UTF-8 text (`Épée rouillée`) and may change without changing an ID. Client actions should send IDs. For `TAKE`/`DROP` and NPC targets, the server also accepts the entire remaining argument as a display name, matched case-insensitively within the relevant room or inventory. An ambiguous display name receives `ERR invalid_arguments Ambiguous name`; an ID remains unambiguous.
- Usernames are separate from world IDs and match `^[a-z][a-z0-9_]{2,19}$` (3–20 ASCII characters, lowercase). A differently cased or malformed username receives `ERR invalid_arguments Invalid username`.
- Directions are lowercase `north`, `south`, `east`, `west`, `up`, or `down`; unknown or unavailable exits receive `ERR invalid_direction`.

## 6. Error codes

```text
ERR invalid_direction No exit to the west
ERR item_not_found Item is not in this room
ERR username_taken Username already in use
```

| Error code | Trigger condition |
|---|---|
| `unknown_command` | Unrecognized command verb |
| `not_authenticated` | Action attempted before `CONNECT` |
| `already_authenticated` | Repeated `CONNECT` on one connection |
| `username_taken` | Username already connected |
| `invalid_arguments` | Missing, malformed, ambiguous, oversized or invalid UTF-8 arguments |
| `invalid_direction` | Invalid or unavailable exit |
| `item_not_found` | Item absent from the room |
| `item_not_obtainable` | Item cannot be picked up |
| `not_in_inventory` | Item absent from the player's inventory |
| `target_not_found` | NPC absent from the room |
| `cannot_attack` | Target cannot be attacked |
| `in_combat` | Action unavailable during combat |
| `not_in_combat` | Combat action attempted outside combat |
| `rate_limited` | Command flooding |

Command-specific quest and group errors remain to be specified with those systems.

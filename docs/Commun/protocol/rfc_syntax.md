# RFC 42TAP — Protocol Syntax & Message Framing

> **Document Status**: `🔵 IN_REVIEW (Dev A proposal; Dev B review pending)`
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

This is Dev A's proposed wire contract for team review. The attached RFC 42TAP in the subject takes precedence wherever it prescribes a different exact format. Any agreed deviation must be recorded in the root README.

## 1. Transport and framing

- TCP, UTF-8, one message per line. Every client and server line ends with exactly one LF byte (`0x0A`); CR and embedded newlines are invalid in commands. JSON strings encode line breaks as `\n` inside the JSON value.
- **Maximum line length: 4,096 bytes including LF**, in either direction. This counts encoded UTF-8 bytes, not characters. The payload before LF can occupy at most 4,095 bytes.
- The server rejects an overlong or invalid UTF-8 request with `ERR invalid_arguments Line exceeds 4096 bytes` or `ERR invalid_arguments Line must be valid UTF-8`. It consumes the rest of an overlong request through LF before parsing the next one. Repeated abuse may trigger `ERR rate_limited` and disconnection.
- The server must serialize compact JSON without literal line breaks and keep every response and event within the same 4,096-byte limit. World data and player/item capacity must be checked so state responses can fit; if they cannot, the data, capacity or limit must change before release. A `CHAT` request whose event would exceed the limit receives `ERR invalid_arguments CHAT message makes the event exceed 4096 bytes; shorten it`. Do not silently truncate messages or JSON.
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
- `OK <json>` has one ASCII space after `OK` and contains one complete JSON value. The JSON schemas are in [json_payloads.md](json_payloads.md). Clients parse error **codes**; the descriptive text is for people and can change. Error messages name the failing command or field and describe a correction when possible.

## 3. Commands and successful replies

| Request | Success reply | Notes |
|---|---|---|
| `CONNECT alice` | `OK connected` | Username must be unique among connected players. |
| `LOOK` | `OK {"room":...,"players":...,"items":...,"npcs":...}` | Detailed item and NPC descriptors. |
| `MOVE north` | `OK room=loc.garden` | Room ID is the destination. |
| `CHAT ROOM Hello everyone` | `OK` | Channel is `GLOBAL`, `ROOM`, or `GROUP`; rest of line is the message. |
| `TAKE item.apple` | `OK taken=item.apple` | ID or full display name may be supplied. |
| `USE item.apple` | `OK used=item.apple hp=85/100` | Extension: owned consumable, ID or complete name; restores HP up to 100. |
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
EVT ROOM ITEM USE bob item.apple
EVT ROOM ITEM DELIVER alice item.rare_herbs
EVT ROOM ITEM REWARD alice item.vigor_potion
```

- Chat recipients are the connected players in the specified scope, including the sender.
- On `MOVE`, send `LEAVE` to remaining players in the old room and `ENTER` to players already in the new room. On disconnect, remove the player first, then send `LEAVE` to remaining room occupants.
- `ITEM TAKE` and `ITEM DROP` are proposed concrete event forms for live GUI updates. Send them to all players in the room, including the actor, after the actor's `OK` reply is queued. They carry the actor username and the unique item ID. Clients may issue `LOOK` or `INVENTORY` when they need refreshed descriptors.
- On disconnect, return held item instances to the player's current room and send `ITEM DROP` to the remaining occupants before `PRESENCE LEAVE`. The departing client has already been removed from the recipient list. This keeps unique objects accessible when player state is removed.
- Combat and quest events must be specified alongside their mechanics; do not invent incompatible wire formats in either client.

## 5. Identifiers and argument resolution

- World IDs are stable, globally unique, lowercase ASCII and match `^(loc|item|npc|quest)\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`, with at most 64 bytes total.
- Prefixes identify the entity kind: `loc.town_square`, `item.rusty_sword`, `npc.guard_captain`, `quest.herbal_cure`.
- Each physical item instance has its own ID. If two apples exist, use `item.apple_1` and `item.apple_2`; moving an item never changes its ID. Never reuse a live instance ID for another item.
- Display names are separate UTF-8 text (`Épée rouillée`) and may change without changing an ID. Client actions should send IDs. For `TAKE`/`DROP`, the server accepts the entire remaining argument as a display name, without quotation marks and matched case-insensitively within the relevant room or inventory. An ambiguous name receives `ERR invalid_arguments TAKE name matches multiple items; use an item ID` (or the equivalent `DROP` message).
- `TALK` accepts an exact NPC ID or a complete display name, without quotation marks and matched case-insensitively within the current room. An exact ID takes precedence. An ambiguous name receives `ERR invalid_arguments TALK name matches multiple NPCs; use an NPC ID`. Unknown or remote NPC targets receive `ERR target_not_found TALK target "npc.guard" is not in loc.garden; use LOOK`. A missing argument receives `ERR invalid_arguments Usage: TALK <npc_id or full display name>`; tabs and surrounding spaces are rejected. The dialogue reply is private. Ordinary TALK emits no event; an eligible delivery consumes the ingredient and grants the reserved reward, followed by ITEM DELIVER then ITEM REWARD events. The same complete-name rules apply to USE in the player inventory.
- Usernames are separate from world IDs and match `^[a-z][a-z0-9_]{2,19}$`: 3–20 ASCII characters, starting with a lowercase letter, followed by lowercase letters, digits, or underscores. For example, `CONNECT aris` is valid and `CONNECT ARIS` is rejected. A malformed username receives `ERR invalid_arguments CONNECT username must start with a lowercase letter and contain 3-20 lowercase letters, digits or underscores`.
- Directions are lowercase `north`, `south`, `east`, `west`, `up`, or `down`. An unknown or unavailable direction receives `ERR invalid_direction` followed by the current room's available exits, sorted alphabetically. A missing direction receives `ERR invalid_arguments` with the same available-exit list. A room without exits reports `none`.

## 6. Error codes

```text
ERR invalid_arguments Usage: CONNECT <username>
ERR invalid_arguments CONNECT username must start with a lowercase letter and contain 3-20 lowercase letters, digits or underscores
ERR not_authenticated Send CONNECT <username> before WHO
ERR invalid_direction No west exit from loc.town_square; available directions: north
ERR username_taken Username "alice" is already connected
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
| `item_not_usable` | USE target is not a healing consumable |
| `health_full` | USE attempted at maximum HP; object is kept |
| `reward_unavailable` | Delivery reward is already claimed or unavailable in reserve; ingredients are kept |
| `not_in_inventory` | Item absent from the player's inventory |
| `target_not_found` | NPC absent from the room |
| `cannot_attack` | Target cannot be attacked |
| `in_combat` | Action unavailable during combat |
| `not_in_combat` | Combat action attempted outside combat |
| `rate_limited` | Command flooding |
| `group_required` | `CHAT GROUP` used without group membership; groups are not yet implemented |
| `response_too_large` | A state response would exceed the 4,096-byte line limit |
| `internal_error` | Server could not encode a response |
| `not_implemented` | Recognized mandatory command awaiting implementation; development-only code |

`USE`, the ITEM USE/DELIVER/REWARD actions, `item_not_usable`, `health_full`, `reward_unavailable`, `group_required`, `response_too_large`, `internal_error`, and `not_implemented` are proposed project extensions pending comparison with the attached RFC. `not_implemented` must disappear when all mandatory commands are complete. Command-specific quest and group errors remain to be specified with those systems.

## 7. Consumable and reward lifecycle

USE checks authentication, ownership, consumable type and missing HP under the
state lock. It queues `OK used=<id> hp=<current>/100`, consumes exactly one
instance, heals without overflow, then emits ITEM USE. At full HP the object
is kept. A consumed instance is absent from LOOK and INVENTORY, cannot be used,
taken or dropped, and is not returned on disconnect.

For fetch_and_deliver definitions, TALK to the local giver automatically
validates possession; no acceptance phase is implemented yet. The existing
TALK JSON schema is preserved, with completion_dialogue on success. The
ingredient, reward transfer, healing and global claim are committed together
after queuing a bounded success reply. No mutation occurs if the reply fails.
Only the first delivery in this world receives the unique reward. The winning
username gets an already-complete dialogue on repeats, including after
reconnect; other players get reward_unavailable. Consumed ingredients and
claimed rewards reset only on restart. Ordinary objects remain shareable with
DROP/TAKE. Claims are not reset when a reward is dropped or consumed.

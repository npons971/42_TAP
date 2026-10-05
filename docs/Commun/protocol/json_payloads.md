# RFC 42TAP — JSON payloads

Updated: 2026-09-03. Source: [external RFC](external_rfc.html).
All JSON is compact on the wire; examples below may be formatted for reading.
Maximum complete line: 4,096 UTF-8 bytes including `OK ` and LF.

## LOOK

```json
{"room":{"id":"loc.town_square","name":"Village Square","description":"A village square.","exits":{"north":"loc.garden"}},"players":["alice"],"items":["item.fountain"],"npcs":["npc.guard"]}
```

Arrays contain IDs, are sorted, and use `[]` when empty; exits uses `{}` when
empty. Players includes the requester. Dead enemies and held/reserved/consumed
items are omitted. LOOK DETAILS replaces the item/NPC arrays with descriptors:

```json
{"items":[{"id":"item.fountain","name":"Stone Fountain","obtainable":false,"description":"A permanent stone fountain.","type":"scenery","usage":"Scenery: inspect it with INFO; it cannot be picked up or consumed."}],"npcs":[{"id":"npc.guard","name":"Village Guard","role":"dialogue"}]}
```

The room and players fields remain identical in that extension.

## INVENTORY

```json
["item.apple","item.rusty_sword"]
```

INVENTORY DETAILS returns item descriptors, as does LOOK DETAILS:

```json
[{"id":"item.rusty_sword","name":"Rusty Sword","obtainable":true,"description":"A worn blade.","type":"weapon","damage_bonus":10,"usage":"Attack bonus applies automatically while carried; only the strongest attack bonus applies."}]
```

Descriptors include id, name, obtainable, description, type and usage. Positive
heal_value, damage_bonus and defense_bonus are included; zero stats are omitted.
Both return `[]` for an empty inventory.

## INFO (extension)

`INFO <item ID or full display name>` returns one item descriptor with the same
fields. Names are case-insensitive; IDs are exact. The player must be authenticated
and the object must be in their inventory or on the ground in their current room,
including scenery. Reserved, consumed, remote and other players' held instances
are inaccessible. Ambiguous full names require an ID. Inspection is allowed
during combat and never changes possession, health, combat or quest progress.
The standard complete-line size limit applies. Usage explains automatic equipment
bonuses, consumable healing or quest delivery. The GUI chooses USE only for
consumable objects with positive healing; all other inventory objects offer INFO.

## STATUS

```json
{"status":"combat","player":"alice","hp":80,"max_hp":100,"state":"EN_COMBAT","combat":{"target_id":"npc.bandit_leader","target_name":"Bandit Leader","target_hp":48,"target_max_hp":70}}
```

Required base fields: hp, max_hp, status. Status is `healthy` at full HP,
`injured` below full HP outside combat, or `combat` during an engagement.
Additional fields retain GUI details. Outside combat, state is HORS_COMBAT and
combat is null.

## ATTACK / DEFEND / FLEE

```json
{"attacker_hp":80,"status":"combat","player":"alice","target":"npc.bandit_leader","damage":22,"counter_damage":20,"player_hp":80,"target_hp":48,"outcome":"ongoing","room":"loc.ruins_den"}
```

RFC example fields attacker_hp, target_hp, damage and status are present.
Project details include counter_damage, target ID, room and outcome. Damage
values are applied amounts bounded by remaining HP. attacker_hp and player_hp
are equal and reflect post-respawn HP where applicable. status is combat while
engaged; terminal values are victory, respawn or fled. outcome additionally
supports ongoing, defended and flee_failed. EVT ROOM COMBAT uses the same JSON.

## QUEST <npc>

```json
{"quest_id":"quest.herbal_cure","description":"Bring rare herbs to the herbalist.","reward":"item.vigor_potion","status":"available"}
```

The giver must be present. A successful first request accepts the quest;
repeated requests return active. Reward is a canonical item ID.

## QUESTS

```json
[{"quest_id":"quest.herbal_cure","status":"active","progress":"1/1"},{"quest_id":"quest.bandit_bounty","status":"completed"}]
```

Only started or completed quests are listed, sorted by quest_id. Empty result:
`[]`. Active progress is 0/1 or 1/1: ownership proves delivery objectives and a
recorded final blow proves defeat objectives. Completed entries omit progress.
The extension status unavailable means another player claimed the unique reward
of a quest this player had already accepted.

QUESTS DETAILS exposes all definitions, sorted by id, with internal states
NOT_STARTED, IN_PROGRESS, OBJECTIVES_MET, COMPLETED, UNAVAILABLE. Each entry
includes id, title, description, status, giver (NPC ID), giver_name,
giver_location_id, giver_location_name, reward (display name), optional
reward_hp_heal (positive HP restored on turn-in), and steps. The GUI journal
requests this extension and falls back to standard QUESTS when it is refused.
The standard QUESTS and QUEST replies retain their RFC fields.

QUESTINFO <quest_id> returns the same details plus the existing objective
object. Both extensions are read-only and never accept a quest:

```json
{"id":"quest.herbal_cure","title":"The Apothecary's Remedy","description":"Bring rare herbs.","status":"IN_PROGRESS","giver":"npc.herbalist","giver_name":"Village Herbalist","giver_location_id":"loc.garden","giver_location_name":"Overgrown Garden","reward":"Vigor Potion","reward_hp_heal":100,"steps":[{"type":"FETCH","label":"Collect Rare Herbs","target_id":"item.rare_herbs","location_id":"loc.garden","location_name":"Overgrown Garden","completed":false},{"type":"DELIVER","label":"Deliver Rare Herbs to Village Herbalist","target_id":"npc.herbalist","location_id":"loc.garden","location_name":"Overgrown Garden","completed":false}],"objective":{"type":"FETCH","target_item":"item.rare_herbs","target_npc":"npc.herbalist","completed":false}}
```

DEFEAT objectives use target_npc for the enemy and omit target_item.
The ordered steps are FETCH then DELIVER, or DEFEAT then REPORT. Every step
contains type, label, target_id, location_id, location_name and completed;
hint is optional. The first step records current possession or a personal
final blow; the second records delivery/report and reward attribution. Both
stay completed after successful turn-in, including after an ingredient is
consumed. Dropping an ingredient clears first-step completion.

Names and initial placements come from world definitions. A ground ingredient
uses its current room, including after DROP or disconnection; a carried or
consumed ingredient uses its original placement as context. If another player
carries it, hint explicitly states it is not currently on the ground. If an
ingredient shared with another quest has already been consumed, hint explains
that it was delivered and is no longer available in this world. If an
enemy has died without this player's final blow, hint explains that another
player defeated it and it does not respawn. UNAVAILABLE means the unique reward
belongs to another player; it does not complete either step for this player.
The 4,096-byte complete-line limit still applies; oversized extension responses
return ERR 413 RESPONSE_TOO_LARGE without changing progress.

## Non-JSON standard replies

WHO returns `OK players=2`. TALK returns `OK <dialogue text>` with ordered world
lines joined by spaces. QUIT returns `OK bye` then closes the connection.

WHO DETAILS returns `{"room":["alice"],"server":2}`.
TALKJSON returns `{"npc":"npc.guard","dialogue":"First line.\nSecond line."}`.
Both TALK formats share acceptance/delivery logic and private replies; ITEM
DELIVER/REWARD events follow successful completion. Full inventory blocks a
reward requiring an extra slot; delivery replacing one ingredient with one
reward remains possible at the inventory limit.

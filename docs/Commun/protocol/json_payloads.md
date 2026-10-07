# RFC 42TAP — JSON payloads

Updated: 2026-10-07. Source: [external RFC](external_rfc.html).
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
{"items":[{"id":"item.fountain","name":"Old Fountain","obtainable":false}],"npcs":[{"id":"npc.guard","name":"Village Guard","role":"dialogue"}]}
```

The room and players fields remain identical in that extension.

## INVENTORY

```json
["item.apple","item.rusty_sword"]
```

INVENTORY DETAILS returns `[{"id":"item.apple","name":"Fresh Apple"}]`.
Both return `[]` for an empty inventory.

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

QUESTS DETAILS exposes all definitions as `{id,title,status}` with internal
states NOT_STARTED, IN_PROGRESS, OBJECTIVES_MET, COMPLETED, UNAVAILABLE.
QUESTINFO <quest_id> is a read-only extension:

```json
{"id":"quest.herbal_cure","title":"The Apothecary's Remedy","giver":"npc.herbalist","description":"Bring rare herbs.","status":"IN_PROGRESS","objective":{"type":"FETCH","target_item":"item.rare_herbs","target_npc":"npc.herbalist","completed":false},"reward":"Vigor Potion"}
```

DEFEAT objectives use target_npc for the enemy and omit target_item.

## Non-JSON standard replies

WHO returns `OK players=2`. TALK returns `OK <dialogue text>` with ordered world
lines joined by spaces. QUIT returns `OK bye` then closes the connection.

WHO DETAILS returns `{"room":["alice"],"server":2}`.
TALKJSON returns `{"npc":"npc.guard","dialogue":"First line.\nSecond line."}`.
Both TALK formats share acceptance/delivery logic and private replies; ITEM
DELIVER/REWARD events follow successful completion. Full inventory blocks a
reward requiring an extra slot; delivery replacing one ingredient with one
reward remains possible at the inventory limit.

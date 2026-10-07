# RFC 42TAP — JSON Payloads Specification

> **Document Status**: `🔵 IN_REVIEW (Dev A proposal; Dev B review pending)`
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

The protocol returns one compact JSON value after `OK ` for complex state responses. These are the proposed field names and types for both clients. The wire-format rules and identifier conventions are in [rfc_syntax.md](rfc_syntax.md). The attached RFC takes precedence if it mandates a different exact shape.

All listed fields are present, even when their arrays are empty. IDs are stable and globally unique; display names are UTF-8 text. The server serializes each payload on one line and keeps the complete line, including LF, within 4,096 bytes.

---

## 1. `LOOK` Payload

Sent in response to the `LOOK` command.

### Proposed schema `🔵 IN_REVIEW`

```json
{
  "room": {
    "id": "loc.town_square",
    "name": "Village Square",
    "description": "A bustling cobblestone square surrounded by warm timber houses.",
    "exits": {
      "north": "loc.garden",
      "east": "loc.market",
      "west": "loc.dark_alley"
    }
  },
  "players": ["alice", "bob"],
  "items": [
    {
      "id": "item.apple",
      "name": "Fresh Apple",
      "obtainable": true
    }
  ],
  "npcs": [
    {
      "id": "npc.guard",
      "name": "Village Guard",
      "role": "dialogue"
    }
  ]
}
```

`room.id`, `room.name`, and `room.description` are strings. `room.exits` maps lowercase directions to room IDs. `players` is an array of usernames in the room, including the requester. `items` is an array of `{id:string,name:string,obtainable:boolean}` objects; `npcs` is an array of `{id:string,name:string,role:string}` objects. Use `[]` for empty lists and `{}` for a room without exits. The detailed descriptors let the GUI render without a separate catalogue request.

---

## 2. `STATUS` Payload

Sent in response to `STATUS`.

### Proposed schema `🔵 IN_REVIEW`

```json
{
  "player": "alice",
  "hp": 85,
  "max_hp": 100,
  "state": "EN_COMBAT",
  "combat": {
    "target_id": "npc.bandit_leader",
    "target_name": "Bandit Leader",
    "target_hp": 48,
    "target_max_hp": 80
  }
}
```
`player` is a username; `hp` and `max_hp` are integers. Outside combat, `"combat":null` and `"state":"HORS_COMBAT"`. In combat, `"state":"EN_COMBAT"` and `combat` has the four fields shown above. Combat mechanics and any additional states need a separate team decision.

---

## 3. `WHO` Payload

Sent in response to `WHO`.

### Existing schema `🟢 VALIDATED`

```json
{
  "room": ["alice", "bob"],
  "server": 6
}
```

---

## 4. `INVENTORY` Payload

Sent in response to `INVENTORY`.

### Proposed schema `🔵 IN_REVIEW`

```json
[
  {"id": "item.rusty_sword", "name": "Rusty Sword"},
  {"id": "item.rare_herbs", "name": "Rare Herbs"}
]
```

The response is an array of `{id:string,name:string}`. An empty inventory is `OK []`. The GUI sends the `id` from this array for `DROP`. Item-specific attributes such as damage bonus belong to the item model and can be added only through an agreed schema revision.

---

## 5. `QUESTS` & `QUEST <id>` Payloads

### `QUESTS` (List of Quests) `🔵 IN_REVIEW`

```json
[
  {
    "id": "quest.herbal_cure",
    "title": "The Apothecary's Remedy",
    "status": "IN_PROGRESS"
  },
  {
    "id": "quest.bandit_bounty",
    "title": "Bounty on the Cutthroat",
    "status": "NOT_STARTED"
  }
]
```

### `QUEST <id>` (Quest Details) `🔵 IN_REVIEW`

```json
{
  "id": "quest.herbal_cure",
  "title": "The Apothecary's Remedy",
  "giver": "npc.herbalist",
  "description": "Collect rare herbs from the abandoned garden and bring them back to the herbalist.",
  "status": "IN_PROGRESS",
  "objective": {
    "type": "FETCH",
    "target_item": "item.rare_herbs",
    "completed": false
  },
  "reward": "Vigor Potion (+100 HP)"
}
```

---

## 6. `TALK <npc_id>` Payload

Sent in response to `TALK`.

### Existing schema `🟢 VALIDATED`

```json
{
  "npc": "npc.guard",
  "dialogue": "Stay safe, traveler. The ruins to the east are full of cutthroats."
}
```

The `npc` value is the stable NPC ID; `dialogue` is UTF-8 text encoded as a JSON string.

---

## 7. Compatibility note

The subject's example exchanges show arrays of item IDs for `LOOK` and `INVENTORY`. This proposal uses descriptor objects so the GUI can show names without a local copy of world data. Confirm that the attached RFC permits these shapes. If it does not, update both clients and this document to match the RFC, and record any approved deviation in the root README.

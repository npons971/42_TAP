# RFC 42TAP — JSON Payloads Specification

> **Document Status**: `🔴 TO_FILL (Team Agreement Required)`  
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

The RFC protocol returns JSON payloads for complex state responses. This document formalizes the canonical JSON schema for each command to guarantee seamless interoperability between the server and both clients (CLI & GUI).

---

## 1. `LOOK` Payload

Sent in response to the `LOOK` command.

### Proposed Schema `🔴 TO_FILL`

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
  "players": ["bob", "charlie"],
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

> **Design Choice Pending `🔴 TO_FILL`**:  
> - **Option A**: Return compact arrays of string IDs for items and npcs: `"items": ["item.apple"], "npcs": ["npc.guard"]`.  
> - **Option B**: Return full descriptor objects (name, description, obtainable) as shown above so GUI can render immediately without extra queries.  
> *(Recommendation: Option B simplifies GUI rendering and reduces round-trip queries).*

---

## 2. `STATUS` Payload

Sent in response to `STATUS`.

### Proposed Schema `🔵 IN_REVIEW`

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
*Note: If player is outside combat, `"combat": null` and `"state": "HORS_COMBAT"`.*

---

## 3. `WHO` Payload

Sent in response to `WHO`.

### Standard RFC Schema `🟢 VALIDATED`

```json
{
  "room": ["alice", "bob"],
  "server": 6
}
```

---

## 4. `INVENTORY` Payload

Sent in response to `INVENTORY`.

### Proposed Schema `🔴 TO_FILL`

- **Option A (Compact ID list)**:
  ```json
  ["item.rusty_sword", "item.rare_herbs", "item.apple"]
  ```
- **Option B (Detailed item array)**:
  ```json
  [
    {
      "id": "item.rusty_sword",
      "name": "Rusty Sword",
      "type": "weapon",
      "damage_bonus": 10
    },
    {
      "id": "item.rare_herbs",
      "name": "Rare Herbs",
      "type": "quest"
    }
  ]
  ```

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

### Standard RFC Schema `🟢 VALIDATED`

```json
{
  "npc": "npc.guard",
  "dialogue": "Stay safe, traveler. The ruins to the east are full of cutthroats."
}
```

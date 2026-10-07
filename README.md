# 42 TAP

This repository is a work in progress for the 42 TAP multiplayer text adventure.

## Current server milestone

The Go TCP server accepts multiple simultaneous connections, sends
`OK hello proto=42TAP/1` to each client, and supports `CONNECT`, `LOOK`,
`MOVE`, `CHAT` (`GLOBAL` and `ROOM`), `WHO`, `STATUS`, `TAKE`, `DROP`,
`INVENTORY`, `TALK`, `USE`, and `QUIT`. It loads Dev B’s complete nine-room world from
[`data/world.json`](data/world.json): ten unique objects, eight NPCs and two
quest definitions. Combat and full quest progression are future milestones; delivery and unique
reward allocation are implemented.
`CHAT GROUP` will become available when groups are implemented.
The proposed wire contract is in [the protocol documentation](docs/Commun/protocol/rfc_syntax.md).
Errors use `ERR <code> <specific message>`. For example, `CONNECT ARIS` explains
the lowercase username rule, `MOVE down` lists the current room's available
exits, and `WHO extra` says that `WHO` takes no arguments. Recognized
commands still under development return `ERR not_implemented`.

Go 1.22 or newer is required. Server Makefile targets provision the local Go
SDK if needed and use repository-local dependency and build caches. Run the
server on the default port 4242:

```sh
make run-server
```

To use another listen address:

```sh
make run-server SERVER_ADDR=127.0.0.1:4243
```

Usernames must be 3–20 ASCII characters, start with a lowercase letter, and
contain only lowercase letters, digits, or underscores. For example,
`CONNECT aris` is valid; `CONNECT ARIS` is rejected. After a failed `CONNECT`,
you can retry on the same connection. `QUIT` closes that connection.

Use `nc 127.0.0.1 4242` from another terminal to connect. Each client first
receives the greeting. Then try:

```text
CONNECT alice
LOOK
MOVE north
LOOK
WHO
STATUS
CHAT GLOBAL Bonjour
QUIT
```

## World and commands

`data/world.json` is the runtime source of truth. `Project/data/world.yaml`
is the original Dev B design snapshot; the server does not load YAML. Edit
JSON for future world changes. All English names and descriptions from Dev B
are preserved. The old two-room French world is a regression fixture in
`internal/server/testdata/two_rooms.json`.

On the square, `TALK npc.guard` and `TALK Village Guard` are equivalent.
Visit the market with `MOVE east`, then try `TAKE Fresh Apple`, `INVENTORY`
and `DROP item.apple`. The sword is in the dark alley, the herbalist and
herbs in the garden, and the rat in the sewers. Targets must be in the
current room; full names are case-insensitive, IDs exact. Dialogue is private.
All ordered dialogue lines are joined with a newline inside the JSON string;
JSON escapes it as `\n`, keeping each response on one TCP line.

World fields:

- `metadata`: name and version; `start`: connection room; `respawn`: future
  defeat destination, defaulting to `start`. Both rooms must exist.
- `locations`: map of room IDs with name, description, exits and `items`/`npcs`
  arrays containing unique instance IDs. NPC spawn counts are represented by
  explicit IDs, one placed instance per NPC.
- `items` and `npcs`: catalogues as arrays of entries with an explicit `id`.
  Roles are `dialogue`, `quest_giver` and `enemy`.
- NPC `dialogue`: nonempty text or a nonempty ordered list of nonblank texts.
- Item `initial_location: {"kind":"reserve"}`: off-map reward instance.
  The Ancient Key and Vigor Potion start in reserve and cannot be taken,
  dropped or displayed in `LOOK`/`INVENTORY`. Every other item must be placed
  in exactly one room; a reserved item cannot also be placed in a room.
- Item attributes (type, description, healing, damage and defense), NPC
  combat attributes, and `quests` definitions are preserved for future
  engines. Consumable healing and delivery rewards are active; combat and
  full quest progression are pending.

The loader rejects invalid IDs, names, roles, dialogues, references,
duplicate instances and conflicting or missing placements. Room exits may
be directed: the square’s `west` exit leads to the dark alley, whose `south`
exit returns to the square. All nine rooms are reachable from the square.
`LOOK` retains descriptors `{id,name,obtainable}` for items and
`{id,name,role}` for NPCs; `INVENTORY` retains `{id,name}`. The 4,096-byte wire
limit, including final LF, is unchanged.

Each physical object has one location: a room, a player's inventory or the
reserve, or a terminal consumed state. `TAKE`/`DROP` transfer it atomically; everyone in the room receives
an item event after the actor's reply. Non-obtainable scenery stays in its
room. Disconnect drops carried items in the current room; restart restores
the initial world. Consumed instances stay unavailable until restart, including after disconnect.
World-wide reward claims also survive disconnect and reconnect.

Use a different JSON world with `make run-server WORLD_FILE=path/to/world.json`.

Open a second `nc` session with `CONNECT bob` to see presence events. `QUIT`
closes only that client connection; the server keeps accepting clients until
you stop `make run-server` with Ctrl+C.

Run the networking tests with the race detector:

```sh
make test-server
```

`make build-server` writes an executable to `.build/tap-server`.

## Consumables and unique delivery rewards

`USE <item_id or full display name>` consumes an owned consumable and restores
its configured `heal_value`, capped at 100 HP. Apple restores 10, ale 15,
potion 100. Example: `OK used=item.apple hp=85/100`. At full HP, `ERR health_full`
keeps the object. Unknown/unowned items return `ERR not_in_inventory`; weapons,
scenery, tools and quest ingredients cannot be consumed (`ERR item_not_usable`).
Names and whitespace rules match TAKE/DROP. Healing is atomic with consumption.

Try the delivery from the square:

```text
CONNECT aris
MOVE north
TAKE item.rare_herbs
TALK npc.herbalist
INVENTORY
STATUS
USE item.vigor_potion
```

TALK consumes the herbs, transfers the unique potion from reserve and restores
HP to 100. The final USE therefore reports full health and keeps the potion.
Combat will later provide ordinary ways to lose HP; tests also exercise USE
with injured players. No client command sets arbitrary HP or grants rewards.

The first eligible delivery wins globally for this server lifetime. No quest
acceptance is required at this stage: ownership and presence at the giver
prove the delivery. Repeating TALK as the winning username produces an
already-complete private dialogue without another reward. Other players
receive `ERR reward_unavailable` once the reward has been awarded. The herbs
stay consumed and do not respawn; players can share unconsumed objects via
DROP/TAKE. A reward may be transferred, dropped or used, but never allocated
a second time. No player-specific copies are created. Restart resets both
claims and placements; there is no disk persistence yet.

`EVT ROOM ITEM USE`, `DELIVER`, and `REWARD` carry `<username> <item_id>`.
Recipients include the actor; the actor's OK is queued first, then DELIVER,
then REWARD for a successful delivery. Clients refresh inventory/status after
these events. Failed or repeated actions emit no item events. A full outgoing
queue or oversized dialogue cannot consume ingredients or allocate rewards.
The bandit reward allocator is ready; its actual eligibility requires the
future combat/quest engine and cannot be triggered by TALK today.

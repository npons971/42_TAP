# 42 TAP

This repository is a work in progress for the 42 TAP multiplayer text adventure.

## Play with the GUI

For a multiplayer demonstration, run `make run-all` in a graphical session.
It starts the server in the current terminal, waits for the TAP greeting, then
opens two separate terminals running `make run-cli` and `make gui`. The GUI uses
this server without starting another one. Press Ctrl+C in the original terminal
to stop the server. A supported terminal emulator must be installed (GNOME
Terminal, Konsole, Xfce Terminal, MATE Terminal, Kitty, Alacritty or xterm).
Use `make build` to compile all three components without launching them.

```sh
make gui
```

The command prepares repository-local tools, builds and opens the desktop client,
and starts a local TAP server when needed. Choose a username and click
**CONNECT & PLAY**. Closing the GUI stops the server started by the launcher;
an existing server remains running. Use `make dev-gui` for live development,
`make build-gui` to compile, and `make check-gui` to diagnose local dependencies.
For a remote server: `make gui GUI_HOST=192.168.1.10 GUI_PORT=4242 GUI_SERVER=off`.
See the [GUI guide](Project/client-gui/README.md) for platform requirements,
installation and gameplay controls. Run `make` for command help.

## Current server milestone

The Go TCP server accepts multiple simultaneous connections, sends
`OK hello proto=1` to each client, and supports `CONNECT`, `LOOK`,
`MOVE`, `CHAT` (`GLOBAL`, `ROOM` and `GROUP`), `WHO`, `STATUS`, `TAKE`, `DROP`,
`INVENTORY`, `TALK`, `ATTACK`, `QUEST`, `QUESTS`, `GROUP`, `USE`,
`DEFEND`, `FLEE`, and `QUIT`. It loads Dev B’s complete nine-room world from
[`data/world.json`](data/world.json): ten unique objects, eight NPCs and two
quest definitions. The standard command signatures, replies and events follow
[the external RFC](docs/Commun/protocol/external_rfc.html), with documented
USE, DEFEND, FLEE and metadata extensions. GROUP supports CREATE, INVITE,
JOIN by leader username, LEAVE and chat across rooms. See the
[conformance report](docs/Commun/protocol/rfc_conformance.md).
The implemented wire contract is in [the protocol documentation](docs/Commun/protocol/rfc_syntax.md).
Errors use `ERR <code> <specific message>`. For example, `CONNECT ARIS` explains
the lowercase username rule, `MOVE down` lists the current room's available
exits, and `WHO extra` says that `WHO` takes no arguments. Unknown verbs return `ERR 400 UNKNOWN_COMMAND`.

Go 1.22 or newer is required. Server Makefile targets provision the local Go
SDK if needed and use repository-local dependency and build caches. Run the
server on the default port 4242:

```sh
make run-server
```

Once listening, the server prints ready-to-copy `make run-cli` and
`make run-gui` commands with its network addresses and actual port. Use the
address on the same network as the other PC. If several interfaces are active,
several command pairs are displayed. A loopback-only listener prints local
commands instead.

To use another listen address:

```sh
make run-server SERVER_ADDR=127.0.0.1:4243
```

Usernames must be 3–20 Unicode codepoints, start with a lowercase letter, and
contain only lowercase Unicode letters, decimal digits, or underscores. For example,
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

- `metadata`: name and version; `start`: connection room; `respawn`: defeat destination, defaulting to `start`. Both rooms must exist.
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
  combat attributes, and `quests` definitions are preserved for the
  implemented combat, healing and quest engines.

The loader rejects invalid IDs, names, roles, dialogues, references,
duplicate instances and conflicting or missing placements. Room exits may
be directed: the square’s `west` exit leads to the dark alley, whose `south`
exit returns to the square. All nine rooms are reachable from the square.
`LOOK DETAILS` returns descriptors `{id,name,obtainable}` for items and
`{id,name,role}` for NPCs; `INVENTORY DETAILS` returns `{id,name}`. The 4,096-byte wire
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

Run all automated tests with the race detector:

```sh
make test
```

`make build-server` writes an executable to `.build/tap-server`.

## Consumables and unique delivery rewards

`USE <item_id or full display name>` consumes an owned consumable and restores
its configured `heal_value`, capped at 100 HP. Apple restores 10, ale 15,
potion 100. Example: `OK used=item.apple hp=85/100`. At full HP, `ERR 409 HEALTH_FULL`
keeps the object. Unknown/unowned items return `ERR 404 ITEM_NOT_IN_INVENTORY`; weapons,
scenery, tools and quest ingredients cannot be consumed (`ERR 405 ITEM_NOT_USABLE`).
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
Combat provides ordinary ways to lose HP; tests also exercise USE with
injured players. No client command sets arbitrary HP or grants rewards.

The first eligible delivery wins globally for this server lifetime. TALK accepts the quest; if ownership already proves the objective, the same
TALK completes the delivery. Repeating TALK as the winning username produces an
already-complete private dialogue without another reward. Other players
receive `ERR 406 NO_QUEST_AVAILABLE` once the reward has been awarded. The herbs
stay consumed and do not respawn; players can share unconsumed objects via
DROP/TAKE. A reward may be transferred, dropped or used, but never allocated
a second time. No player-specific copies are created. Restart resets both
claims and placements; there is no disk persistence yet.

`EVT ROOM ITEM USE`, `DELIVER`, and `REWARD` carry `<username> <item_id>`.
Recipients include the actor; the actor's OK is queued first, then DELIVER,
then REWARD for a successful delivery. Clients refresh inventory/status after
these events. Failed or repeated actions emit no item events. A full outgoing
queue or oversized dialogue cannot consume ingredients or allocate rewards.
The bandit bounty requires a real recorded victory; TALK to its captain then
grants the key. Victories and quest acceptance survive reconnect by username.

## CLI client

```sh
make run-cli
make run-cli CLI_ADDR=127.0.0.1:4243
make build-cli
```

The CLI uses direct protocol commands and displays asynchronous replies/events.
Linux terminals preserve partially typed UTF-8 input when an event arrives;
backspace and Ctrl-U edit input, Ctrl-C/Ctrl-D quit. Pipes and other platforms
use plain line mode. QUIT/EOF/cancellation close the client and restore terminal
settings. Commands, replies and timeouts are bounded.

## Combat, quests and groups

ATTACK accepts a living local hostile NPC ID or full name. One player engages
one enemy; a second attacker receives target_busy. Each ATTACK is one turn:
player damage **12 + strongest carried weapon bonus**, then a surviving enemy
counterattacks for **max(1, attack_power - strongest armor bonus)**. Bonuses do
not stack. Deterministic damage makes balancing and outcomes reproducible.
DEFEND halves the counterattack, rounded down. FLEE [direction] succeeds 70%
of the time; without a direction it selects the first sorted exit. Failed
fleeing costs a counterattack. MOVE/TAKE/DROP/TALK/USE/QUEST require leaving combat.
Read commands (including QUESTINFO/QUESTS), chat and groups stay usable. STATUS reflects actual HP and target.

At zero HP, the player respawns in world.respawn with **30 HP**, keeps inventory
and leaves combat. Disconnect/flee releases the enemy without resetting its HP.
A defeated enemy disappears until restart; the finishing player's victory is
recorded for quests. The single engagement prevents duplicated turns and kills.

TALK at a giver accepts its quest or completes it if the objective is already
met. QUESTINFO/QUESTS DETAILS report NOT_STARTED, IN_PROGRESS, OBJECTIVES_MET, COMPLETED or
UNAVAILABLE. Dropping an ingredient makes its objective incomplete again.
The bandit requires a recorded final blow followed by a report to the captain.
A pre-acceptance victory also counts. All unique rewards remain world-wide;
acceptance/victory proof survives reconnect by username, but not server restart.
CONNECT has no password, so usernames are not secure persistent identities.

GROUP CREATE returns OK group=<generated-id>. GROUP INVITE username notifies
the invitee with EVT GROUP INVITE <leader>; GROUP JOIN <leader-username> joins
that leader's group and returns OK group=<id>. GROUP LEAVE returns OK. JOIN
and LEAVE events update members; disconnect leaves the group. If its leader
leaves, the first remaining username in sorted order becomes leader and an
extension EVT GROUP LEADER announces it. Empty groups are deleted. Invitations
are informational: the RFC does not require invitation-only membership.

QUEST <local-npc-ID-or-name> accepts a quest and returns quest_id, description,
reward item ID and status (available on first acceptance, active on repeats).
QUESTS lists only the player's active/completed quests, with progress 0/1 or
1/1 while active. A started quest becomes unavailable if another player claims
its unique reward. TALK at the giver still accepts or completes quests for the
existing gameplay flow. QUESTINFO <quest-id> and QUESTS DETAILS expose read-only
definitions and the internal states listed above.

## RFC formats and extensions

WHO returns OK players=<count>. LOOK items/npcs and INVENTORY contain ID arrays.
TALK returns dialogue text on one line. QUIT sends OK bye before closing.
STATUS includes hp, max_hp and status (healthy, injured or combat); ATTACK
includes attacker_hp, target_hp, damage and status, plus documented game details.
Verbs are case-insensitive. CONNECT ARIS remains invalid under our lowercase
username policy, while CONNECT élise is accepted.

The GUI uses LOOK DETAILS, INVENTORY DETAILS and TALKJSON to retain display
names, item flags and ordered dialogue lines without shipping a local world
catalogue. These are explicit extensions; default commands use RFC formats.
Responses are correlated with outgoing commands so an empty QUESTS does not
clear inventory. STATS events update the authenticated player count.

We retain a 4,096-byte line limit including LF instead of the RFC's recommended
1,024 bytes, to fit room metadata and NPC dialogues. Oversized requests are
drained and rejected; oversized responses fail without committing an action.
We follow LF framing, as specified in the RFC transport/general grammar, despite
a contradictory CRLF production in its CONNECT example. Limits are 128 active
TCP sessions and 32 inventory slots; fetch rewards can replace an ingredient
at capacity, while other rewards require a free slot. A refused reward remains
available for retry after DROP.

## Logging and robustness

Structured JSON logs include timestamp, level, connections, commands/parameters,
all written replies/events and error codes, state changes, NPC interactions,
combat and quest progress. Use `.build/tap-server -log-file server.log` to append
to a file instead of stdout. Command flooding is limited to a 128-command burst
with 20/s refill, then rate_limited and disconnect. More than five connections
from an IP in two seconds produces a warning; shared school IPs are not banned.
IP tracking is bounded. Per-client queues and write deadlines prevent an idle
receiver from blocking other sessions. SIGINT/SIGTERM stop the server cleanly.

The shared mutex protects all state; typed handlers check and queue their OK
before committing mutations. Events are queued after the actor's reply, with
network writes isolated in the writer goroutine. The protocol contract and its
extensions are documented locally; the external RFC is covered by dedicated TCP tests that assert standard frames
and event order. The [conformance report](docs/Commun/protocol/rfc_conformance.md)
records the migration and justified extensions. Testing against another team’s
independent client remains an external interoperability check.

## Verification

```sh
make test
make lint
make build-server build-cli
```

`make test` provisions local Go, Node and C tools as needed, runs the server/CLI
and GUI TCP backend tests with the race detector, checks frontend protocol
handlers and builds Svelte. Go tests always run again (no result cache), with
a two-minute timeout per package. On Linux, CLI tests also exercise a real
pseudo-terminal, including remote disconnect, input, SIGTERM and restoration.
No graphical session or GTK/WebKit installation is needed for these checks.

`make lint` checks Go formatting without modifying files, runs `go vet` on the
server/CLI and GUI TCP backend, checks JavaScript syntax and rejects Svelte
compiler warnings. Generated Wails JavaScript is excluded. Use `V=1` for full
command output; failures otherwise display the log location in `.build/logs/`.

Tests cover complete bounty/delivery runs, quest status/reconnect, group chat
isolation across rooms, concurrent enemy/group/reward contention, defend/flee,
respawn, transactional failures, UTF-8/line bounds, logs/abuse detection and CLI
async input/cancellation. A real CLI-to-server test completes the bounty and
receives the key. See docs/Commun/testing/aris_acceptance.md for the test matrix.

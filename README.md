*This project has been created as part of the 42 curriculum by allasser, arvella, nopons.*

# 42 TAP — The Answer Protocol

## Description

TAP is a multiplayer text adventure in a shared world. Players explore rooms, chat, exchange unique items, interact with NPCs, fight enemies and complete quests. A Go TCP server owns the game state; a Go CLI and a Wails desktop GUI connect through RFC 42TAP. The GUI uses Svelte for its web interface. State resets when the server restarts; disk persistence is not required.

## Instructions

Run commands from the repository root. The current installer targets Linux x86_64. A graphical session is needed to launch the desktop client. Installation downloads dependencies and therefore requires network access on the first run.

```sh
make install
make build
```

All SDKs, runtimes, native libraries and caches are installed inside the repository. No administrator access or global package installation is required. Use the local `./go`, `./node`, `./npm` and `./wails` wrappers. The Makefile exports the local PATH, GOPATH, Go cache and npm cache. See [AGENTS.md](AGENTS.md) for environment rules and the [GUI guide](Project/client-gui/README.md) for GTK/WebKit setup.

Run `make check-gui` to diagnose the local tools and native dependencies.

## Building and Running

| Command | Purpose |
|---|---|
| `make install` | Install repository-local dependencies |
| `make build` | Compile the server, CLI and GUI |
| `make build-server` | Write `.build/tap-server` |
| `make build-cli` | Write `.build/tap-cli` |
| `make build-gui` | Write `Project/client-gui/build/bin/tap-gui` |
| `make run-server` | Start the TCP server |
| `make run-client` | Start the terminal client |
| `make run-client-gui` | Build and launch the graphical client |
| `make run-all` | Start the server and open both clients in separate terminals |
| `make dev-gui` | Launch the GUI with frontend live reload |
| `make test` | Run automated tests and build the frontend |
| `make lint` | Check Go, JavaScript and Svelte |
| `make clean` | Remove downloaded tools, archives and local tool caches |

The `gui` target remains a shorthand for `run-client-gui`. `clean` does not remove `.build`, frontend dependencies or generated GUI build artifacts. Use `V=1` for detailed command output.

### Multiplayer launch

For a multiplayer demonstration, run `make run-all` in a graphical session.
It starts the server in the current terminal, waits for the TAP greeting, then
opens two separate terminals running `make run-client` and `make run-client-gui`. The GUI uses
this server without starting another one. Press Ctrl+C in the original terminal
to stop the server started by this command. An existing TAP server is reused and
remains running; if another service occupies the port, the command exits with an
explanation. A supported terminal emulator must be installed (GNOME
Terminal, Konsole, Xfce Terminal, MATE Terminal, Kitty, Alacritty or xterm).
Use `make build` to compile all three components without launching them.

```sh
make run-client-gui
```

The command prepares repository-local tools, builds and opens the desktop client,
and starts a local TAP server when needed. Choose a username and click
**CONNECT & PLAY**. Closing the GUI stops the server started by the launcher;
an existing server remains running. Use `make dev-gui` for live development,
`make build-gui` to compile, and `make check-gui` to diagnose local dependencies.
For a remote server: `make run-client-gui GUI_HOST=192.168.1.10 GUI_PORT=4242 GUI_SERVER=off`.
See the [GUI guide](Project/client-gui/README.md) for platform requirements,
installation and gameplay controls. Run `make` for command help.

### Server and protocol session

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

The server and CLI require Go 1.22 or newer; the GUI module requires Go 1.25 or newer. Makefile targets provision the local Go
SDK if needed and use repository-local dependency and build caches. Run the
server on the default port 4242:

```sh
make run-server
```

Once listening, the server prints ready-to-copy `make run-client` and
`make run-client-gui` commands with its network addresses and actual port. Use the
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

## CLI Client

```sh
make run-client
make run-client CLI_ADDR=127.0.0.1:4243
make build-cli
```

The CLI uses direct protocol commands and displays asynchronous replies/events.
Linux terminals preserve partially typed UTF-8 input when an event arrives;
backspace and Ctrl-U edit input, Ctrl-C/Ctrl-D quit. Pipes and other platforms
use plain line mode. QUIT/EOF/cancellation close the client and restore terminal
settings. Commands, replies and timeouts are bounded.

## Architecture

The server uses a command dispatcher with separate handlers for protocol, items, NPCs, combat, quests and groups. `internal/server/world.go` loads and validates immutable JSON world definitions. Runtime player sessions, item locations, enemy HP, groups and quest progress are owned by `Server`.

Each TCP session has a reader and a writer goroutine. A shared `sync.RWMutex` protects game state. Handlers validate actions and queue the successful response before committing mutations; events follow the actor's response. Bounded outgoing queues and write deadlines prevent a slow or disconnected receiver from blocking other players. Disconnect removes the player before broadcasting departure, releases combat and group membership, and drops carried items. SIGINT/SIGTERM close the listener and active sessions.

The CLI receives messages independently of keyboard input. The GUI has a Go TCP backend and a Svelte interface, with asynchronous Wails events and command/reply correlation. The server remains authoritative; the GUI does not ship a separate world catalogue.

See the [server architecture](docs/Dev_A/architecture/server_architecture.md) and [concurrency model](docs/Dev_A/architecture/concurrency_model.md).

## Protocol Implementation

WHO returns OK players=<count>. LOOK items/npcs and INVENTORY contain ID arrays.
TALK returns dialogue text on one line. QUIT sends OK bye before closing.
STATUS includes hp, max_hp and status (healthy, injured or combat); ATTACK
includes attacker_hp, target_hp, damage and status, plus documented game details.
Verbs are case-insensitive. CONNECT ARIS remains invalid under our lowercase
username policy, while CONNECT élise is accepted.
This username policy is a project choice, not an RFC requirement. The GUI
converts uppercase player names to lowercase before validation and sending,
so entering ARIS in the login form sends CONNECT aris.

The GUI uses LOOK DETAILS, INVENTORY DETAILS, QUESTS DETAILS, INFO and TALKJSON to retain display
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

The [implemented wire contract](docs/Commun/protocol/rfc_syntax.md) and [RFC conformance report](docs/Commun/protocol/rfc_conformance.md) describe standard frames, error codes, event ordering and extensions. The local [RFC 42TAP](docs/Commun/protocol/external_rfc.html) is the protocol reference. Independent cross-team client/server testing remains to be performed.

## World Design

The active world contains nine interconnected rooms, ten unique objects, eight NPCs with dialogue, quest-giver and enemy roles, and two quests. The main loop is Village Square → Central Market → Tavern → Dark Alley → Village Square. A second loop runs through Tavern Cellar and Damp Sewers. The Overgrown Garden and the ruins form optional branches. Six obtainable objects start in rooms; two more are reserved quest rewards. The fountain and chest are fixed scenery.

`data/world.json` is the runtime source of truth. `Project/data/world.yaml`
is the original nopons design snapshot; the server does not load YAML. Edit
JSON for future world changes. All English names and descriptions from nopons
are preserved. The old two-room French world is a regression fixture in
`internal/server/testdata/two_rooms.json`.

On the square, `TALK npc.guard` and `TALK Village Guard` are equivalent.
Visit the market with `MOVE east`, then try `TAKE Fresh Apple`, `INVENTORY`
and `DROP item.apple`. The sword is in the dark alley, the herbalist and
herbs in the garden, and the rat in the sewers. Targets must be in the
current room; full names are case-insensitive, IDs exact. Dialogue is private.
With TALKJSON, ordered dialogue lines are joined with a newline inside the JSON string;
JSON escapes it as `\n`, keeping each response on one TCP line. Standard TALK
joins the dialogue onto one text line.

World fields:

- `metadata`: name and version; `start`: connection room; `respawn`: defeat destination, defaulting to `start`. Both rooms must exist.
- `locations`: map of room IDs with name, description, exits and `items`/`npcs`
  arrays containing unique instance IDs. NPC spawn counts are represented by
  explicit IDs, one placed instance per NPC.
- `items` and `npcs`: catalogues as arrays of entries with an explicit `id`.
  Roles are `dialogue`, `quest_giver` and `enemy`.
- NPC `dialogue`: nonempty text or a nonempty ordered list of nonblank texts.
- Item `initial_location: {"kind":"reserve"}`: off-map reward instance.
  The Captain's Shield and Vigor Potion start in reserve and cannot be taken,
  dropped or displayed in `LOOK`/`INVENTORY`. Every other item must be placed
  in exactly one room; a reserved item cannot also be placed in a room.
- Item attributes (type, description, healing, damage and defense), NPC
  combat attributes, and `quests` definitions are preserved for the
  implemented combat, healing and quest engines.

The loader rejects invalid IDs, names, roles, dialogues, references,
duplicate instances and conflicting or missing placements. Room exits may
be directed: the square’s `west` exit leads to the dark alley, whose `south`
exit returns to the square. All nine rooms are reachable from the square.
`LOOK DETAILS` and `INVENTORY DETAILS` return item descriptors with ID, name,
description, type, healing, attack/defense bonuses and usage instructions;
NPC descriptors retain `{id,name,role}`. `INFO <item ID or full name>` returns
the same item information for an owned item or one on the ground in your room,
including during combat. Equipment bonuses apply automatically while carried;
only the strongest bonus for each stat applies. The GUI offers USE for healing
consumables and INFO for other inventory items. Healing Salve restores 25 HP;
the bandit bounty grants Captain's Shield with 8 defense. The 4,096-byte wire
limit, including final LF, is unchanged; standard LOOK/INVENTORY still return IDs.

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

See the [world design](docs/Commun/gdd/05_world_design.md).

## Combat System

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

The player always acts first; each command advances one turn. Fixed damage and strongest-only equipment bonuses keep outcomes reproducible and avoid stacking bonuses. FLEE adds a documented chance of failure, while DEFEND gives a defensive turn. All combat results are logged and broadcast to players in the affected room. See the [combat design](docs/Commun/gdd/03_systeme_combat.md).

## Quest System

The two quest types are fetch-and-deliver (Rare Herbs for the herbalist) and defeat-and-report (the Bandit Leader bounty for the captain). Completion checks actual item ownership or a recorded personal victory rather than trusting client declarations. World-wide unique rewards preserve the item-instance model and prevent repeated reward allocation.

TALK at a giver accepts its quest or completes it if the objective is already
met. QUESTINFO/QUESTS DETAILS report NOT_STARTED, IN_PROGRESS, OBJECTIVES_MET, COMPLETED or
UNAVAILABLE. Dropping an ingredient makes its objective incomplete again.
The bandit requires a recorded final blow followed by a report to the captain.
A pre-acceptance victory also counts. All unique rewards remain world-wide;
acceptance/victory proof survives reconnect by username, but not server restart.
CONNECT has no password, so usernames are not secure persistent identities.

QUEST <local-npc-ID-or-name> accepts a quest and returns quest_id, description,
reward item ID and status (available on first acceptance, active on repeats).
QUESTS lists only the player's active/completed quests, with progress 0/1 or
1/1 while active. A started quest becomes unavailable if another player claims
its unique reward. TALK at the giver still accepts or completes quests for the
existing gameplay flow. QUESTINFO <quest-id> and QUESTS DETAILS expose read-only
definitions and the internal states listed above.

### Consumables and delivery rewards

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
grants the Captain's Shield. Victories and quest acceptance survive reconnect by username.

See the [quest design](docs/Commun/gdd/04_systeme_quetes.md).

## Group Management

GROUP CREATE returns OK group=<generated-id>. GROUP INVITE username notifies
the invitee with EVT GROUP INVITE <leader>; GROUP JOIN <leader-username> joins
that leader's group and returns OK group=<id>. GROUP LEAVE returns OK. JOIN
and LEAVE events update members; disconnect leaves the group. If its leader
leaves, the first remaining username in sorted order becomes leader and an
extension EVT GROUP LEADER announces it. Empty groups are deleted. Invitations
are informational: the RFC does not require invitation-only membership.
GROUP INVITE/JOIN player names are converted to lowercase before lookup;
aris, ARIS and ArIs therefore target the same player in the CLI and GUI.

The GUI's **GROUP (5)** dialog shows separate flows for creating a group,
inviting players and joining. Received invitations display an **Accept invitation**
button; joining by leader username remains available. Group members and the
current leader are refreshed through the authenticated `GROUP STATUS` extension,
which returns `OK {"id":"group.1","leader":"alice","members":["alice","bob"]}`
with sorted members, or `OK {"id":"","leader":"","members":[]}` when ungrouped.

## Server Logging

Structured JSON logs include timestamp, level, connections, commands/parameters,
all written replies/events and error codes, state changes, NPC interactions,
combat and quest progress. Use `.build/tap-server -log-file server.log` to append
to a file instead of stdout. Command flooding is limited to a 128-command burst
with 20/s refill, then rate_limited and disconnect. More than five connections
from an IP in two seconds produces a warning; shared school IPs are not banned.
IP tracking is bounded. Per-client queues and write deadlines prevent an idle
receiver from blocking other sessions. SIGINT/SIGTERM stop the server cleanly.

Logs use INFO for normal activity, WARN for rejected requests and abuse patterns, and ERROR for failures. The server logs written frames from the writer goroutine rather than performing socket writes inside state handlers. Inspect the terminal output or the selected log file; rapid-connection and command-flood records identify suspicious activity. See the [logging documentation](docs/Dev_A/logging/server_logging.md).

## Group Contributions

The project is organized into three areas:

| Member | Current responsibility |
|---|---|
| arvella (Dev A) | TCP server, sessions, RFC parsing and routing, replies and events, social commands, concurrency, logging and abuse detection |
| nopons (Dev B) | GUI, TCP integration, asynchronous updates, gameplay views and controls, chat and player counters |
| allasser (Dev C) | Game engine, world data and validation, exploration, items, NPCs, combat, quests, rewards and CLI |

arvella handles transport and protocol compliance; allasser handles game rules and
state mutations. They jointly define synchronization and the result/event
contract. nopons renders server data and sends actions through the GUI.
Each member owns tests and documentation for their area. Gameplay design,
build tooling, integration, interoperability and peer review are shared.

See the [team organization](docs/Commun/TAP_taches_communes.md) and
[individual responsibility documents](docs/README.md).

## Testing

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
receives the Captain's Shield. See docs/Commun/testing/aris_acceptance.md for the test matrix.

### Manual multiplayer checks

1. Start `make run-server`, then `make run-client` and `make run-client-gui GUI_SERVER=off` in two other terminals. In the CLI, enter `CONNECT alice`; in the GUI, connect as `bob`. Check room and server counters and presence notifications.
2. Send `CHAT GLOBAL Hello`, move one player to another room, and verify room chat isolation. Create a group and check group chat across rooms.
3. Move both players to the market. Let one use `TAKE Fresh Apple`, confirm the object disappears for both players, then DROP it and let the other player take it.
4. Complete the herbalist delivery and the bandit bounty. Inspect QUESTS and INVENTORY, then verify that repeating completion never duplicates rewards.
5. Fight the rat or bandit, try DEFEND/FLEE, and verify STATUS. Trigger defeat and confirm respawn on the square at 30 HP.
6. Close one client abruptly. Confirm that the other remains responsive, counters update and carried items return to the room. Inspect the server logs for commands, errors, combat and quest events.

Automated tests do not replace a visual GUI session, installation on a clean machine, or interoperability checks with another team. The [acceptance matrix](docs/Commun/testing/aris_acceptance.md) documents additional scenarios.

## Resources

- [RFC 42TAP](docs/Commun/protocol/external_rfc.html): commands, replies, events and transport requirements.
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119): interpretation of protocol requirement keywords.
- [RFC 5234](https://www.rfc-editor.org/rfc/rfc5234): ABNF syntax.
- [Go documentation](https://go.dev/doc/) and [net package](https://pkg.go.dev/net): language and TCP networking.
- [Go race detector](https://go.dev/doc/articles/race_detector): concurrent-state verification.
- [log/slog documentation](https://pkg.go.dev/log/slog): structured logging.
- [Wails documentation](https://wails.io/docs/introduction): desktop Go/web integration.
- [Svelte documentation](https://svelte.dev/docs): reactive GUI components.
- [Project documentation index](docs/README.md): architecture, gameplay design and client guides.

### AI usage

For this revision, AI assistance was used to compare the subject and RFC with the existing code and documentation, run and interpret repository checks, restructure this README, and update client launch commands in the Makefile, server startup output, launcher and related guides. These checks are assistance for contributor review; they do not establish independent cross-team interoperability or replace the team's understanding of the implementation. Contributors remain responsible for reviewing generated changes, explaining the code and validating it with peers.

*This project has been created as part of the 42 curriculum by allasser, arvella, nopons.*

# 42 TAP — The Answer Protocol

## Description

TAP is a multiplayer text adventure in a shared world. Players explore a village and its surroundings, chat, collect unique items, talk to NPCs, fight enemies and complete quests. The project includes a Go TCP server, a Go terminal client and a Wails desktop client with a Svelte interface. Both clients use RFC 42TAP to communicate with the same server.

The server owns the game state. Items and quest rewards are shared between players, and the world resets when the server restarts.

## Instructions

Run commands from the repository root. The installer targets Linux x86_64 and downloads dependencies on the first run. The desktop client needs a graphical session.

```sh
make install
make build
make run-all
```

`make run-all` starts the server and opens the CLI and GUI in separate terminals. Enter `CONNECT alice` in the CLI; choose `bob` and click **CONNECT & PLAY** in the GUI. Both players start on Village Square. Press Ctrl+C in the original terminal to stop the server started by the launcher. An existing TAP server is reused and remains running.

This launcher needs a supported terminal emulator: GNOME Terminal, Konsole, Xfce Terminal, MATE Terminal, Kitty, Alacritty or xterm. You can also launch each component separately as shown below.

All SDKs, native libraries and caches stay inside the repository; no administrator access is needed. Use the local `./go`, `./node`, `./npm` and `./wails` tools. The Makefile configures their environment. See [AGENTS.md](AGENTS.md) for the local tooling rules.

## Building and Running

| Command | Purpose |
|---|---|
| `make install` | Install local Go, Node, Wails and GTK/WebKit dependencies |
| `make build` | Compile the server, CLI and GUI |
| `make build-server` | Build `.build/tap-server` |
| `make build-cli` | Build `.build/tap-cli` |
| `make build-gui` | Build `Project/client-gui/build/bin/tap-gui` |
| `make run-server` | Start the TCP server on port 4242 |
| `make run-client` | Connect the CLI to `127.0.0.1:4242` |
| `make run-client-gui` | Build and open the GUI, starting a local server if needed |
| `make run-all` | Start the server and open both clients |
| `make dev-gui` | Launch the GUI with frontend live reload |
| `make check-gui` | Check local tools and GTK/WebKit dependencies |
| `make test` | Run automated tests and build the frontend |
| `make lint` | Check Go, JavaScript and Svelte |
| `make clean` | Remove downloaded tools, archives and tool caches |

To launch the components yourself, run each command in a separate terminal:

```sh
make run-server
make run-client
make run-client-gui GUI_SERVER=off
```

The server prints client launch commands with its network addresses. For another computer on the same network, use the server's address:

```sh
make run-client CLI_ADDR=192.168.1.10:4242
make run-client-gui GUI_HOST=192.168.1.10 GUI_PORT=4242 GUI_SERVER=off
```

Change the listener with `make run-server SERVER_ADDR=127.0.0.1:4243`, or load another world with `make run-server WORLD_FILE=path/to/world.json`. Set the clients' address and port accordingly.

The server and CLI modules require Go 1.22 or newer; the GUI module requires Go 1.25 or newer. Makefile targets provision the local SDK as needed. Use `V=1` for detailed output; failed build and check commands report their logs in `.build/logs/`. `make clean` keeps `.build`, frontend dependencies and generated GUI artifacts. Run `make` for command help and see the [GUI guide](Project/client-gui/README.md) for desktop setup and controls.

### CLI usage

The CLI sends typed RFC commands directly to the server. Replies and events arrive while you type; on Linux, incoming events preserve the current input line. Other platforms and pipes use plain line mode. `QUIT`, EOF or Ctrl+C closes the client.

```text
CONNECT alice
LOOK
MOVE east
TAKE Fresh Apple
INVENTORY
CHAT GLOBAL Hello everyone
DROP item.apple
QUIT
```

Item and NPC targets accept an exact ID or a full display name, without quotes. Display names are case-insensitive, and targets must be in the current room or your inventory as appropriate.

## Architecture

The server uses a command dispatcher with separate handlers for sessions, social commands and gameplay. This keeps TCP handling separate from the rules for items, NPCs, combat and quests. The world loader validates the JSON definitions before accepting players; runtime state is held by `Server`.

Each TCP session has a reader and a writer goroutine. A shared `sync.RWMutex` protects game state so competing actions cannot duplicate items or rewards. Handlers queue the actor's successful reply before committing an action, then send its events. Bounded outgoing queues and write deadlines keep slow clients from blocking other players.

Disconnect removes the player before broadcasting departure, releases combat and group membership, and drops carried items in the current room. SIGINT/SIGTERM close the listener and active sessions.

The CLI receives messages independently of keyboard input. The GUI uses a Go TCP backend and asynchronous Wails events to update Svelte views. It reads world information from the server rather than maintaining a separate world catalogue.

See the [server architecture](docs/Dev_A/architecture/server_architecture.md) and [concurrency model](docs/Dev_A/architecture/concurrency_model.md).

## Protocol Implementation

TCP messages use UTF-8 and one LF-terminated line per message. A new connection receives `OK hello proto=1`. Commands return `OK` or `ERR <code> <token> <explanation>`; asynchronous updates use `EVT`. Standard `LOOK` and `INVENTORY` responses contain item IDs, `WHO` returns `OK players=<count>`, and `QUIT` sends `OK bye` before closing.

Our choices and extensions to [RFC 42TAP](docs/Commun/protocol/external_rfc.html) are:

- **Line length:** 4,096 bytes including LF, instead of the recommended 1,024, to fit room metadata and dialogues. Oversized requests are rejected; an action whose reply is too large is not committed.
- **Line ending:** LF follows the RFC transport and general grammar sections; its CONNECT example contains a conflicting CRLF production.
- **Usernames:** 3–20 Unicode codepoints, starting with a lowercase letter and containing only lowercase letters, decimal digits or underscores. This is a project policy. The GUI lowercases input before sending CONNECT; GROUP INVITE/JOIN also normalize target names. Command verbs are case-insensitive.
- **Metadata commands:** `LOOK DETAILS`, `INVENTORY DETAILS`, `WHO DETAILS`, `INFO`, `TALKJSON`, `QUESTINFO`, `QUESTS DETAILS` and `GROUP STATUS` expose names, attributes, ordered dialogue, quest progress or group membership. They support the GUI without changing default command formats.
- **Extension discovery:** the authenticated `CAPABILITIES` command lists the metadata extensions supported by the server, so the GUI can select compatible requests.
- **Gameplay commands:** `USE`, `DEFEND` and `FLEE` provide healing and additional combat actions. `TALK` can also accept or complete a quest. `QUESTS` uses the additional status `unavailable` when another player has claimed a started quest's unique reward.
- **Events and errors:** `EVT ROOM ITEM` actions (`TAKE`, `DROP`, `USE`, `DELIVER`, `REWARD`), `EVT ROOM COMBAT` and `EVT GROUP LEADER` notify clients of game and group changes. Additional error codes cover invalid arguments, combat restrictions, capacity limits and rate limiting; error explanations give command-specific advice.

The server allows 128 active TCP sessions and 32 inventory slots per player. Recoverable command errors leave the connection open. The [wire contract](docs/Commun/protocol/rfc_syntax.md) and [conformance report](docs/Commun/protocol/rfc_conformance.md) detail frames, codes and extensions. Independent client/server testing with another team remains to be completed.

## World Design

The world has nine rooms, eight NPCs, ten unique objects and two quests. Its main circuit is Village Square → Central Market → Tavern → Dark Alley → Village Square. Another circuit connects the tavern, cellar, sewers and alley. The garden and ruins are optional branches. Some return paths use a different direction: the square's west exit leads to the alley, whose south exit returns to the square.

NPCs have three roles: dialogue characters around the village, quest givers in the garden and at Ruins Gate, and enemies in the sewers and ruins. Six obtainable objects start in rooms: an apple, ale, sword, healing salve, shield and herbs. The fountain and chest are fixed scenery. The Captain's Shield and Vigor Potion start in reserve, hidden and unavailable until awarded; afterwards they can be carried, dropped and displayed like other inventory items.

Each object has one location: a room, an inventory, the reserve or a consumed state. TAKE/DROP transfers it without creating copies. Equipment bonuses apply automatically while carried; only the strongest bonus for each stat applies.

The active data is [data/world.json](data/world.json). [Project/data/world.yaml](Project/data/world.yaml) preserves the design reference originally created by nopons; the server loads JSON only. World maintenance now belongs to allasser. See the [world design](docs/Commun/gdd/05_world_design.md) for the map and content details.

## Combat System

Players start with 100 HP. The sewer rat has 20 HP and 6 attack power; the bandit leader has 70 HP and 20 attack power. One player can engage a given enemy at a time.

Each `ATTACK` advances one turn. The player acts first and deals **12 + strongest carried weapon bonus** damage. If the enemy survives, it counters for **max(1, enemy attack power − strongest carried armor bonus)**. Fixed damage keeps the rules easy to understand and test; equipment bonuses do not stack.

`DEFEND` spends a turn taking half the counterattack damage, rounded down. `FLEE [direction]` succeeds with a 70% chance; failure costs a counterattack. Without a direction, it selects the first exit in sorted order. Movement, item transfers, healing, dialogue and quest acceptance are blocked during combat; read commands, chat and groups remain available.

At zero HP, the player respawns on Village Square with 30 HP and keeps their inventory. Fleeing or disconnecting releases the enemy without resetting its HP. Defeated enemies disappear until server restart. Results are logged and broadcast to players in the affected room, and `STATUS` shows current HP and combat state.

See the [combat design](docs/Commun/gdd/03_systeme_combat.md).

## Quest System

The herbalist's quest requires collecting Rare Herbs and delivering them in the garden. Completion consumes the herbs, restores the player's health to 100 HP and awards the Vigor Potion. The captain's bounty requires defeating the Bandit Leader, landing the final blow, then reporting back at Ruins Gate for the Captain's Shield.

`QUEST <NPC ID or full name>` accepts a quest; `QUESTS` lists accepted quests and their progress. Talking to the giver also accepts or completes it when the objective is met. Completion checks actual item ownership or a recorded personal victory. Dropping the herbs makes the delivery objective incomplete again; a victory before accepting the bounty still counts.

Each reward can be claimed once for the whole server lifetime. Other players cannot receive another copy. Quest acceptance and victory records survive reconnect by username, but usernames have no password and are not secure identities. Restart clears progress and restores the world. A full inventory blocks a reward unless delivery frees a slot; the player can drop an item and retry.

`USE <item ID or full name>` consumes an owned healing item outside combat, capped at 100 HP. At full health, it leaves the item untouched. Consumed items stay unavailable until restart.

See the [quest design](docs/Commun/gdd/04_systeme_quetes.md).

## Group Management

Use `GROUP CREATE`, `GROUP INVITE bob`, `GROUP JOIN alice` and `GROUP LEAVE`. JOIN targets the leader's username. Invitations notify players but are not required to join. Group chat works across rooms. When a leader leaves, the first remaining username in sorted order becomes leader; empty groups are removed.

The GUI group dialog provides creation, invitation and join controls, including an acceptance button for received invitations.

## Server Logging

The server writes structured JSON logs to stdout by default. Logs include precise timestamps, INFO/WARN/ERROR levels, connection and disconnection IP addresses, received commands and parameters, written replies/events and error codes, item movements, NPC interactions, combat and quest progress.

To append logs to a file:

```sh
make build-server
.build/tap-server -log-file server.log
```

Inspect the terminal or log file for normal activity and warnings. `ABUSE_FLOOD` identifies command flooding: clients have a burst allowance of 128 commands, refilled at 20 per second, and are disconnected when it is exhausted. More than five connections from an IP in two seconds triggers a warning; shared school IPs are not automatically banned.

Connection tracking is bounded, and written frames are logged by the writer goroutine outside game-state handlers. See the [logging documentation](docs/Dev_A/logging/server_logging.md) for event fields and monitoring details.

## Group Contributions

The current division of work is:

| Member | Responsibilities |
|---|---|
| **arvella (Dev A)** | TCP server and sessions; RFC parsing, routing, replies and events; CONNECT/CHAT/WHO/GROUP/QUIT; network concurrency, disconnects, logging and abuse detection; network tests and documentation |
| **nopons (Dev B)** | Wails/Svelte GUI and TCP integration; asynchronous updates, room and inventory views, gameplay controls, chat, groups and player counters; GUI tests and documentation |
| **allasser (Dev C)** | Game engine and world data; exploration, unique items, NPCs, combat, quests and rewards; CLI and asynchronous terminal input; gameplay/CLI tests and documentation |

arvella and allasser share the server engine: arvella owns the network layer, the dispatcher and the event contract, allasser owns the terminal client, the world data and the gameplay rules (items, NPCs, rewards, combat, quests). All three contribute to gameplay decisions, local build tooling, integration tests and peer review.

See the [team task breakdown](docs/Commun/TAP_taches_communes.md) and [individual task documents](docs/README.md).

## Testing

```sh
make test
make lint
```

`make test` runs server, CLI and GUI TCP backend tests with the race detector, frontend protocol/map checks and a Svelte build. Linux CLI tests use a real pseudo-terminal to check asynchronous input, disconnects and terminal restoration. These checks do not need a graphical session. `make lint` checks Go formatting and vet results, JavaScript syntax and Svelte compiler warnings.

Tests cover concurrent item and reward claims, combat and respawn, complete delivery/bounty quests, reconnects, groups, protocol errors and logging. The [acceptance matrix](docs/Commun/testing/aris_acceptance.md) describes the scenarios.

### Manual multiplayer checks

Start a fresh server and connect a CLI as `alice` and a GUI as `bob`, using the separate launch commands above.

1. Check presence notifications and room/server counters. Send `CHAT GLOBAL Hello`, then move one player away and check that ROOM chat stays local.
2. Create a group as alice, join it as bob, and send `CHAT GROUP Hello` from different rooms.
3. Move both players east to the market. Have one take `Fresh Apple`; check that it disappears for both. Drop it and let the other player take it.
4. From the square, run the delivery and bounty sequences below. Check QUESTS, HP and inventory in both clients; repeat TALK to verify that rewards are not duplicated.
5. Fight an enemy, try DEFEND/FLEE and inspect STATUS. For a defeat check, use a fresh world and attack the bandit repeatedly without equipment; confirm respawn on the square at 30 HP.
6. Close a client abruptly. Check that the other stays responsive, counters update and carried items return to the room. Inspect logs for commands, errors, combat and quest events.

Herbalist delivery, starting on Village Square:

```text
MOVE north
QUEST npc.herbalist
TAKE item.rare_herbs
TALK npc.herbalist
QUESTS
INVENTORY
STATUS
MOVE south
```

Bandit bounty, starting on Village Square at full health:

```text
MOVE west
TAKE item.rusty_sword
MOVE south
MOVE east
MOVE east
TAKE item.wooden_shield
QUEST npc.guard_captain
MOVE north
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
MOVE south
TALK npc.guard_captain
QUESTS
INVENTORY
```

These sequences assume the items and rewards have not been claimed. Restart the server before repeating them. A visual GUI session, installation on a clean machine and interoperability with another team's client/server remain separate manual checks.

## Resources

- [RFC 42TAP](docs/Commun/protocol/external_rfc.html): commands, replies, events and transport.
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and [RFC 5234](https://www.rfc-editor.org/rfc/rfc5234): requirement keywords and ABNF syntax.
- [Go documentation](https://go.dev/doc/) and [net package](https://pkg.go.dev/net): language and TCP networking.
- [Go race detector](https://go.dev/doc/articles/race_detector): concurrency checks.
- [log/slog](https://pkg.go.dev/log/slog): structured logging.
- [Wails documentation](https://wails.io/docs/introduction) and [Svelte documentation](https://svelte.dev/docs): desktop integration and interface components.
- [Project documentation](docs/README.md): architecture, design and client guides.

### AI usage

AI assisted with code and documentation reviews against the subject and RFC, running and interpreting repository checks, and editing the README and team task breakdown. It also helped modify the Makefile, server startup messages, client launcher and related guides. The team remains responsible for reviewing the changes and validating the project.

# Server architecture — implemented

The Go module is rooted at the repository. `cmd/server` loads and validates
`data/world.json`, configures JSON logging, opens TCP and handles SIGINT/SIGTERM.
All SDK/cache paths are local to the repository through Makefile targets.

`internal/server.Server` owns immutable world definitions and mutable runtime
maps: clients, connected usernames, item positions, NPC HP/engagements, groups,
quest progress by username and global unique reward claims.

Each connection has a reading goroutine and a dedicated writing goroutine.
The writer consumes a bounded channel and uses a five-second write deadline.
The reader frames LF lines, validates UTF-8/length, applies the command limiter,
logs the command and dispatches to typed handlers. A switch is used because
the command set is small and it keeps argument conventions explicit. TAKE,
DROP, TALK, ATTACK, QUEST and USE retain the complete rest of the line for display names.
Fixed-argument commands reject empty tokens and tabs.

A single RWMutex protects mutable state. LOOK/WHO/STATUS/INVENTORY/QUESTINFO/QUESTS
read it; all mutations and target/ownership checks use its write lock. Every
transaction validates and queues its success reply before changing gameplay
state. Broadcasts enqueue, never write sockets, while the lock is held.

Disconnect removes the player, releases the enemy and group membership, drops
only owned items, broadcasts departure, closes the channel, waits for the writer
and closes TCP. Groups disappear when their last member leaves. NPC deaths,
consumed objects, quest proofs and reward claims remain until server restart;
quest progress survives reconnect by username. There is no disk persistence.

World loading validates IDs, initial placements, exits, roles, NPC health,
healing, quest title/description, givers, target items/enemies and unique reserved
rewards. The original YAML is a design snapshot; JSON is the runtime source.

# Concurrency Model & Thread-Safety

> **Document Status**: `🟡 IN_PROGRESS (Initial model implemented)`
> **Assigned to**: Aris (Dev A) & Dev C<br>
> **Last Updated**: 2026-10-07

The server must handle multiple concurrent TCP connections, process asynchronous world events, and prevent data race conditions without deadlocking or crashing during client disconnections.

---

## 1. Goroutine Topology per Client

```text
               +----------------------------------+
               |        TCP Accept Goroutine      |
               +-----------------+----------------+
                                 | accepts conn
                                 v
        +------------------------------------------------+
        |                 Client Session                 |
        |                                                |
        |  [Read Goroutine]           [Write Goroutine]  |
        |   bufio.Scanner             Outbox Channel     |
        |   Reads TCP '\n'            chan string (buf)  |
        |         |                           |          |
        |         v                           v          |
        |   Command Parser            Writes to TCP      |
        +------------------------------------------------+
```

### Why a Dedicated Write Goroutine?
If the server writes directly to a TCP socket in the broadcaster loop, a slow, unresponsive, or stalled client will block the entire server or cause goroutine leaks.
* Each client owns an outbound buffered channel `outbox := make(chan string, 128)`.
* Broadcasting sends to the channel using a non-blocking `select`.

---

## 2. Shared State Synchronization `🟡 IMPLEMENTED`

The initial implementation uses option A. Option B remains an alternative if later measurements justify changing the model.

### Option A: Read-Write Mutex (`sync.RWMutex`)
- Protect `GameState` with a global `sync.RWMutex` (or fine-grained per-room mutexes).
- Read-heavy commands (`LOOK`, `WHO`, `STATUS`, `INVENTORY`, `QUEST`, `QUESTS`) acquire `mu.RLock()`.
- State-mutating commands (`MOVE`, `TAKE`, `DROP`, `USE`, `TALK`, `ATTACK`, `DEFEND`, `FLEE`, `GROUP`) acquire `mu.Lock()`.
- *Pros*: Natural and idiomatic in Go; high read throughput.
- *Risks*: Must never hold locks while writing to network sockets (causes deadlocks).

### Option B: Single-Threaded Event Loop (Channel-Based Actor Pattern)
- State mutation commands are packed as messages and sent to a central coordinator goroutine through an input channel.
- Only the coordinator modifies `GameState` sequentially.
- *Pros*: Completely eliminates mutex contention and data race conditions by design.
- *Risks*: Slightly higher boilerplate for command return channels.

> **Initial decision**: Use a single `sync.RWMutex` around the connected-player maps, player positions, item locations, NPC engagements/HP, groups and quest progress. `LOOK`, `WHO`, `STATUS` and `INVENTORY` take a read lock; `CONNECT`, `MOVE`, `TAKE`, `DROP`, `USE`, `TALK` and disconnect take a write lock. Broadcasts only enqueue into bounded per-client channels while the lock is held; socket writes happen in dedicated writer goroutines. Revisit lock granularity if measurements show contention when combat is added.

Each unique item has one authoritative runtime position: a room ID, a player owner, an explicit off-map reserve or a terminal consumed state. Reserved reward instances are initialized once and are excluded from TAKE/DROP resolution and room/inventory views. `TAKE` and `DROP` resolve the target, check ownership, enqueue the success response and change ownership under the same write lock. Two concurrent `TAKE` commands therefore cannot acquire the same instance. The actor receives the success response before their `EVT ROOM ITEM` event. The catalogue and initial room placements remain static; `LOOK` uses runtime positions.

The NPC catalogue and room placements are immutable after loading. TALK
resolves local targets under the write lock because it may deliver an owned
ingredient and allocate a unique reward. USE also holds that lock through
ownership checks, reply queuing, consumption and healing. Global rewardClaims
survive disconnect; disconnect returns only owned items, never consumed ones.
TALK preflights encoding/line length and reply queuing before any delivery
mutation. No network writes occur under this lock; broadcasts enqueue events.

---

## 3. Resilient Broadcasting & Safe Disconnection `🔵 IN_REVIEW`

### Subject Requirement
> *"Broadcasts messages without interruption if a client disconnects mid-send."*

### Implementation Pattern
```go
func (b *Broadcaster) BroadcastToRoom(roomID string, message string) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    for _, client := range b.rooms[roomID] {
        select {
        case client.outbox <- message:
            // Successfully queued
        default:
            // Buffer full or client lagging: drop message or schedule eviction
            log.Warn("Client outbox full, dropping broadcast", "player", client.Name)
        }
    }
}
```

### Safe Disconnect Sequence
1. TCP read loop detects `QUIT`, `io.EOF` or connection error.
2. Under the write lock, remove the player from the connected-player map.
3. Return each held item to the player's current room and enqueue `EVT ROOM ITEM DROP <username> <item_id>` for remaining occupants, sorted by item ID.
4. Enqueue `EVT ROOM PRESENCE LEAVE <username>` and unregister the connection while still under the lock.
5. Release the lock, close the client's `outbox`, let the writer finish, close the TCP socket and log the disconnect. Removal from the maps prevents subsequent broadcasts from targeting this channel.

## Combat, groups and quest lifetime

NPC runtime HP and the engaged opponent are separate from immutable catalogue
data. ATTACK/DEFEND/FLEE compute and encode the turn before committing it under
the write lock. Disconnect releases its target and group before removing items.
Empty groups are deleted. Quest acceptance and final-blow proofs are stored by
username and survive reconnect, while live group membership and engagement do
not. All runtime data resets on restart. Timed input/rate limiting uses only
per-connection reader state; response logging snapshots identity under a read
lock and writes outside it.

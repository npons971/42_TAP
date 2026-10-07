# Concurrency Model & Thread-Safety

> **Document Status**: `🟡 IN_PROGRESS (Initial model implemented)`
> **Assigned to**: Aris (Dev A)  
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

## 2. Shared State Synchronization `🔴 TO_FILL`

The team must choose between two concurrency strategies:

### Option A: Read-Write Mutex (`sync.RWMutex`)
- Protect `GameState` with a global `sync.RWMutex` (or fine-grained per-room mutexes).
- Read-heavy commands (`LOOK`, `WHO`, `STATUS`) acquire `mu.RLock()`.
- State-mutating commands (`MOVE`, `TAKE`, `DROP`, `ATTACK`) acquire `mu.Lock()`.
- *Pros*: Natural and idiomatic in Go; high read throughput.
- *Risks*: Must never hold locks while writing to network sockets (causes deadlocks).

### Option B: Single-Threaded Event Loop (Channel-Based Actor Pattern)
- State mutation commands are packed as messages and sent to a central coordinator goroutine through an input channel.
- Only the coordinator modifies `GameState` sequentially.
- *Pros*: Completely eliminates mutex contention and data race conditions by design.
- *Risks*: Slightly higher boilerplate for command return channels.

> **Initial decision**: Use a single `sync.RWMutex` around the connected-player maps and positions. `LOOK` takes a read lock; `CONNECT`, `MOVE`, and disconnect take a write lock. Broadcasts only enqueue into bounded per-client channels while the lock is held; socket writes happen in dedicated writer goroutines. Revisit lock granularity if measurements show contention when combat and items are added.

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
1. TCP read loop detects `io.EOF` or connection error.
2. Unregister player from `GameState` and close client's `outbox` channel.
3. Broadcast `EVT ROOM PRESENCE LEAVE <username>` to room occupants.
4. Close TCP socket and terminate goroutines.
5. Log event with timestamp and IP address.

# Server Architecture & Design Choices

> **Document Status**: `🔴 TO_FILL (Architecture Decisions to Finalize)`  
> **Assigned to**: Aris (Dev A)  
> **Last Updated**: 2026-10-07

The 42 subject requires the team to document and justify server design choices (specifically *dispatcher/router vs inline handling*, *state management*, and *world loader integrity*). This document defines the architectural blueprint.

---

## 1. High-Level Layered Architecture

```text
+-------------------------------------------------------------+
|                        NETWORK LAYER                        |
|  - TCP Listener (port 4242)                                 |
|  - Client Connection Manager (goroutine per socket)        |
+------------------------------+------------------------------+
                               | raw line string
                               v
+-------------------------------------------------------------+
|                       PROTOCOL LAYER                        |
|  - Command Parser (tokens, ABNF validation)                 |
|  - Command Router / Dispatcher                              |
|  - JSON Serialization / Response Formatter                  |
+------------------------------+------------------------------+
                               | typed command struct
                               v
+-------------------------------------------------------------+
|                      GAME ENGINE LAYER                      |
|  - GameState (Source of Truth)                              |
|  - Player & Room managers                                   |
|  - Turn-based Combat Engine                                 |
|  - Quest Progression Engine                                 |
|  - Broadcaster (Non-blocking EVT distributor)               |
+------------------------------+------------------------------+
                               ^
                               | loads static data on boot
+------------------------------+------------------------------+
|                     WORLD LOADER / DATA                     |
|  - YAML/JSON parser & integrity validator                   |
+-------------------------------------------------------------+
```

---

## 2. Command Handling Pattern `🔴 TO_FILL`

### Architectural Alternatives

* **Option A: Router / Command Dispatcher (Recommended)**
  * A central map/table mapping verbs to handler functions:
    ```go
    type CommandHandler func(ctx *Context, args []string) error
    var routes = map[string]CommandHandler{
        "CONNECT": handleConnect,
        "LOOK":    handleLook,
        "MOVE":    handleMove,
        // ...
    }
    ```
  * *Pros*: High maintainability, clean separation of concerns, easy unit testing of isolated handlers, simple to add new commands during peer-evaluation.
  * *Cons*: Slight abstraction overhead.

* **Option B: Inline Switch-Case Handling**
  * Single central loop evaluating command verbs through a monolithic `switch cmd { case "CONNECT": ... }`.
  * *Pros*: Direct code flow, no function pointer indirection.
  * *Cons*: Quickly becomes a giant file prone to merge conflicts; harder to isolate for testing.

> **Decision**: `🔴 TO_FILL` — Record final choice and justification for the README here.

---

## 3. GameState: Central Source of Truth

The server maintains the authoritative in-memory state. Neither CLI nor GUI holds independent game truth.

```go
type GameState struct {
    mu       sync.RWMutex
    Players  map[string]*Player
    Rooms    map[string]*Room
    Items    map[string]*Item
    NPCs     map[string]*NPC
    Quests   map[string]*Quest
    Broadcaster *Broadcaster
}
```

### Domain Models
- **`Player`**: Username, Connection pointer, Current Room ID, HP (base 100), Inventory (`[]*Item`), Active Quests, Combat Status (`InCombat`, `TargetID`).
- **`Room`**: ID, Name, Description, Exits (`map[string]string`), Items (`[]*Item`), NPCs (`[]*NPC`), Players present (`map[string]*Player`).
- **`Item`**: ID, Display Name, Description, Obtainable boolean.
- **`NPC`**: ID, Display Name, Role (`dialogue`, `quest_giver`, `enemy`), HP, Attack power, Dialogues.
- **`Quest`**: ID, Title, Steps, Target Item/NPC, Reward.

---

## 4. Static World Loader & Validation `🔵 IN_REVIEW`

On startup, the server loads static world definitions from `world.yaml` or `world.json`:
1. **Parsing**: Deserializes raw file into intermediate Go structs.
2. **Integrity Checks (Mandatory Validation)**:
   - Check that all room exit targets point to existing room IDs.
   - Check that referenced item IDs in rooms exist in the items catalogue.
   - Check that referenced NPC IDs in rooms exist in the NPC catalogue.
   - Verify that all quests reference valid NPCs and valid items.
   - Validate world topological invariants (at least 8 rooms, presence of a loop, presence of an optional branch).
3. If any reference is broken, the server logs a fatal error with precise location and exits immediately.

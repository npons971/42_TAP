# CLI Client Specifications

> **Document Status**: `🔵 IN_REVIEW`  
> **Assigned to**: Aris (Dev A)  
> **Last Updated**: 2026-10-07

The CLI client provides a lightweight, retro terminal interface to the TAP server. Both clients must be interchangeable and remain responsive to asynchronous incoming events while waiting for keyboard input.

---

## 1. Asynchronous I/O Architecture

To prevent terminal input from blocking incoming event display, the CLI client runs two concurrent goroutines:

```text
                  +---------------------------+
                  |         TERMINAL          |
                  +-------------+-------------+
                                |
             +------------------+------------------+
             |                                     |
             v                                     v
   [Goroutine 1 : User Input]             [Goroutine 2 : Socket Listener]
    - Reads line from os.Stdin            - Reads line from net.Conn
    - Formats/validates command           - Parses OK, ERR, EVT
    - Writes to TCP socket                - Prints to terminal
```

---

## 2. Command Interface Choice `🔴 TO_FILL`

The subject explicitly provides two design choices:

* **Approach 1: Direct RFC Syntax**
  - User types raw RFC commands directly into terminal (e.g., `LOOK`, `MOVE north`, `TAKE item.ale`).
  - *Pros*: Zero translation layer, direct reflection of the protocol.
  - *Cons*: Less intuitive for players who do not know the technical command verbs.

* **Approach 2: User-Friendly Command Translation (Recommended)**
  - Translates natural verbs into RFC commands:
    - `go north` / `n` $\rightarrow$ `MOVE north`
    - `get sword` / `take sword` $\rightarrow$ `TAKE item.rusty_sword`
    - `inv` / `i` $\rightarrow$ `INVENTORY`
    - `look` / `l` $\rightarrow$ `LOOK`
    - `say hello` $\rightarrow$ `CHAT ROOM hello`
  - Also accepts raw RFC syntax as fallback.
  - *Pros*: Significantly better player ergonomics during peer-evaluations.

> **Decision**: `🔴 TO_FILL` — Record chosen approach and justification for the README.

---

## 3. Terminal Prompt & Visual Formatting

- **Incoming Events (`EVT`)**: Formatted with distinct ANSI colors:
  - Chat: `[CHAT/GLOBAL] <Alice>: Hello!` (Cyan)
  - Presence: `[PRESENCE] Bob entered the room.` (Yellow)
  - Combat: `[COMBAT] Alice attacks Goblin for 14 damage.` (Red)
- **Prompt Preservation**:
  - Ensure incoming messages do not corrupt half-typed input lines (using line clearing `\r\033[K` or standard line editing libraries).

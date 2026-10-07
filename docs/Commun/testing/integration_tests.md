# Integration & Automated Testing Suite

> **Document Status**: `🟡 IN_PROGRESS`  
> **Assigned to**: Aris (Dev A) & Novanns (Dev B)  
> **Last Updated**: 2026-10-07

The README requires a dedicated "Testing" section demonstrating how multiplayer functionality, combat mechanics, and quest progression are verified. This document outlines the test strategy and test harnesses.

---

## 1. Test Pyramid & Scope

```text
                  / \
                 /   \
                / E2E \       Multi-Client Concurrency Harness
               /-------\
              / Integr. \     TCP Socket & State Integration Tests
             /-----------\
            /    Unit     \   Protocol Parsers, Combat Formulas, Quests
           /---------------\
```

---

## 2. Unit Tests (`go test ./...`) `🟢 VALIDATED`

* **Protocol Parser Tests**:
  * Validation of ABNF command tokens.
  * Correct parsing of multi-word item and NPC names.
  * Handling of malformed, oversized, or empty lines (`\n` framing).
* **Combat Engine Tests**:
  * Damage calculations with and without equipped weapons.
  * Enemy counter-attack logic and `DEFEND` damage reduction (50%).
  * Death trigger at 0 HP and respawn coordinates/HP reset.
* **Quest Engine Tests**:
  * State transition sequence: `NOT_STARTED` $\rightarrow$ `IN_PROGRESS` $\rightarrow$ `COMPLETED`.
  * Item consumption on quest delivery.

---

## 3. Network Integration & Concurrency Harness `🔵 IN_REVIEW`

### Test Case 1: Multi-Client Presence & Chat
1. Launch test server on an ephemeral port.
2. Connect Client A (`alice`) and Client B (`bob`).
3. Have `bob` enter `alice`'s room; assert `alice` receives `EVT ROOM PRESENCE ENTER bob`.
4. Send `CHAT ROOM Hello` from `bob`; assert `alice` receives the message.
5. Move `bob` to another room; assert `alice` no longer receives subsequent room messages from `bob`.

### Test Case 2: Dynamic Item Concurrency (No Duplication)
1. Spawn an item in room (`item.rare_herbs`).
2. Client A and Client B concurrently issue `TAKE item.rare_herbs` at the exact same millisecond.
3. Assert that exactly one client receives `OK taken=item.rare_herbs`.
4. Assert that the second client receives `ERR 404 ITEM_NOT_FOUND`.
5. Verify total item instance count in the world remains 1.

### Test Case 3: Mid-Broadcast Abrupt Disconnect
1. Connect 10 simulated clients in room `loc.town_square`.
2. Concurrently broadcast `CHAT ROOM Test broadcast`.
3. Abruptly kill the TCP sockets of 5 clients mid-broadcast without sending `QUIT`.
4. Assert that the server logs disconnection cleanly, does not panic, and remaining clients receive the full message.

### Test Case 4: Command Flood Throttling
1. Connect client and fire 50 rapid commands in under 100ms.
2. Assert server responds with `ERR 429 RATE_LIMITED` and records a `WARN ABUSE_FLOOD` log.

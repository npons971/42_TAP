# Server Logging & Security Monitoring

> **Document Status**: `🔵 IN_REVIEW`  
> **Assigned to**: Aris (Dev A)  
> **Last Updated**: 2026-10-07

The 42 subject mandates comprehensive, structured server logging and abuse pattern detection. This document specifies the JSON logging schema, log levels, audit trails, and anti-flood protections.

---

## 1. Structured Logging Schema (JSON)

All logs are emitted as single-line JSON objects to standard output (or log file) using standard Go logging (`log/slog` or custom JSON encoder).

### Standard Fields
```json
{
  "timestamp": "2026-10-07T13:45:00.123Z",
  "level": "INFO",
  "event": "COMMAND_RECEIVED",
  "client_ip": "192.168.1.42:54321",
  "player": "alice",
  "command": "MOVE",
  "params": ["north"],
  "response": "OK room=loc.market",
  "duration_ms": 0.42
}
```

---

## 2. Event Types & Log Levels

| Log Level | Event Category | Mandatory Logged Information |
|---|---|---|
| `INFO` | `CLIENT_CONNECT` | Client IP, port, connection timestamp |
| `INFO` | `CLIENT_DISCONNECT` | Client IP, player name, session duration, graceful vs abrupt |
| `INFO` | `COMMAND_EXECUTED` | Player, command verb, parameters, reply (`OK`/`ERR`) |
| `INFO` | `STATE_CHANGE` | Item taken/dropped, room changes, quest stage advancement |
| `INFO` | `COMBAT_EVENT` | Combat initiated, attacker, target, damage, death, respawn |
| `WARN` | `ABUSE_FLOOD` | Rate limit exceeded, client IP, player, command count |
| `WARN` | `RAPID_RECONNECT` | Multiple connections within brief threshold from same IP |
| `ERROR` | `INTERNAL_ERROR` | World data parsing failure, socket write failures |

---

## 3. Abuse Pattern Detection & Throttling `🔴 TO_FILL`

### 3.1 Command Flooding Detection
- **Mechanism**: Token Bucket or Sliding Window rate limiter per client connection.
- **Proposed Threshold `🔴 TO_FILL`**:
  - Maximum **20 commands per second** per socket.
  - Burst capacity: 30 tokens.
- **Action upon trigger**:
  - Respond with: `ERR rate_limited Slow down, command flood detected\n`.
  - Emit log:
    ```json
    {
      "timestamp": "2026-10-07T13:45:01.000Z",
      "level": "WARN",
      "event": "ABUSE_FLOOD",
      "client_ip": "192.168.1.42:54321",
      "player": "alice",
      "msg": "Exceeded rate limit (25 req/s)"
    }
    ```

### 3.2 Rapid Reconnections Monitor
- Track connection timestamps per client IP.
- If an IP initiates more than 5 connections in 2 seconds, emit a `WARN RAPID_RECONNECT` log and throttle connection accept.

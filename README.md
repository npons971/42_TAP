# 42 TAP

This repository is a work in progress for the 42 TAP multiplayer text adventure.

## Current server milestone

The Go TCP server accepts multiple simultaneous connections, sends
`OK hello proto=42TAP/1` to each client, and supports `CONNECT`, `LOOK`,
`MOVE`, `CHAT` (`GLOBAL` and `ROOM`), `WHO`, `STATUS`, `TAKE`, `DROP`,
`INVENTORY`, and `QUIT`. It loads the temporary two-room world in
[`data/world.json`](data/world.json). The full world and remaining commands
are future milestones.
`CHAT GROUP` will become available when groups are implemented.
The proposed wire contract is in [the protocol documentation](docs/Commun/protocol/rfc_syntax.md).
Errors use `ERR <code> <specific message>`. For example, `CONNECT ARIS` explains
the lowercase username rule, `MOVE west` lists the current room's available
exits, and `WHO extra` says that `WHO` takes no arguments. Recognized
commands still under development return `ERR not_implemented`.

Go 1.22 or newer is required. Run the server on the default port 4242:

```sh
make run-server
```

To use another listen address:

```sh
make run-server SERVER_ADDR=127.0.0.1:4243
```

Usernames must be 3–20 ASCII characters, start with a lowercase letter, and
contain only lowercase letters, digits, or underscores. For example,
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

Each physical object has its own stable ID. On the village square, try
`TAKE item.apple`, `INVENTORY`, and `DROP Pomme fraîche`. Full display names
are accepted without quotation marks and matched without case sensitivity.
`LOOK` reflects the objects currently on the ground. Item events are sent to
everyone in the room, including the actor, after their command reply.

The JSON world contains an `items` catalogue with one entry per instance and
an `items` array of IDs in each room. Every instance must start in exactly one
room. The loader rejects duplicate IDs, unknown references and repeated or
missing placements. A non-obtainable object, such as `item.fountain`, stays in
its room. On disconnect, carried objects return to the player's current room.
The sample placements are for testing; Dev B will provide the full world.

Open a second `nc` session with `CONNECT bob` to see presence events. `QUIT`
closes only that client connection; the server keeps accepting clients until
you stop `make run-server` with Ctrl+C.

Run the networking tests with the race detector:

```sh
make test-server
```

`make build-server` writes an executable to `.build/tap-server`.

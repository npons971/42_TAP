# Server logging and abuse monitoring — implemented

The server uses a standard `log/slog` JSON handler. Entries have RFC3339 timestamps,
INFO/WARN/ERROR levels and one JSON object per line. Default output is stdout.
`-log-file path` appends to a file opened with mode 0600; startup failures are
reported and exit nonzero. Log files are deployment output, not repository data.

Logged information:

- Connection/disconnection: remote IP/port, player and session duration.
- COMMAND_RECEIVED: player, remote address, verb and complete remaining arguments.
- SERVER_RESPONSE: every written OK/ERR/EVT, recipient, remote address, response
  text and error_code for ERR. Socket failures use ERROR.
- State changes: room/item movement, consumption, NPC interaction, group joins
  and departures, combat turns, quest start/objective/completion.
- WARN: invalid frames, full output queues, command flood and rapid connections.

The command token bucket is per connection: burst **128**, refill **20/s**.
The larger burst accommodates startup refreshes and scripted clients; sustained
flooding receives `ERR 429 RATE_LIMITED Command flood detected; connection closing`
and closes the session, releasing combat/groups and dropping held objects.
Malformed frames also count toward this limit.

Connection monitoring warns after more than **5 connections from one IP in
2 seconds**. Old windows expire, and at most 1024 IP windows are stored. This
monitor does not ban an IP, since multiple users can share a school/NAT address.

Broadcasts only enqueue to bounded client channels. Socket writes and response
logging happen in each client's dedicated writer; network operations never
hold the shared state lock. The logger writes to a local output stream; select
a suitable file or consuming stdout reader when running the server.

Tests check the burst/refill/cap deterministically and inspect structured TCP
logs after all writers stop, including command arguments, responses, error
codes, timestamps, interactions and rapid-connection warnings. A TCP flood test verifies that only the abusive
session closes while a second client remains usable.

# CLI client — implemented

Current owner: Alexis (Dev C), game engine/world/CLI pole.
Historical implementation: Aris. This document retains its original path;
see [team organization](../../Commun/TAP_taches_communes.md).

The client lives in `cmd/client-cli` and `internal/cli`. It uses the Go standard
library and accepts **direct protocol syntax**. This keeps commands identical
between the terminal, the GUI backend and peer clients, without guessing IDs
from short names. Full display names remain supported by the server.

```sh
make run-cli
make run-cli CLI_ADDR=127.0.0.1:4243
make build-cli
.build/tap-cli -addr 127.0.0.1:4242
```

Enter CONNECT alice, then any documented protocol command. QUIT closes the
session and client; input EOF also sends QUIT. Ctrl-C/Ctrl-D quit in interactive
Linux mode, and SIGTERM cancels the client. The client waits at most five seconds
for a server to acknowledge closure after QUIT; socket writes also have a
five-second deadline.

Two goroutines independently send input and receive OK/ERR/EVT. A mutex
protects terminal rendering, so events remain visible while the user is typing.
On Linux TTYs, a small stdlib termios adapter disables local echo and canonical
input. The client redraws `> ` and the buffered text after events, supports
UTF-8, backspace and Ctrl-U, and ignores arrow escape sequences. Original
terminal attributes are restored before stdin closes, including on errors,
remote disconnection and cancellation. Other platforms and pipes use portable
line mode without ANSI rendering.

```sh
printf 'CONNECT alice\nLOOK\nQUESTS\nQUIT\n' | .build/tap-cli
```

Replies are displayed verbatim. The client does not consume responses as if
unsolicited events were replies to commands. Commands and received lines are
bounded at 4096 bytes including LF and validated as UTF-8. In line mode, CRLF
input is normalized to LF; embedded CR/LF are rejected. Local oversized or
invalid interactive commands display LOCAL ERROR and are not sent.

Tests cover piped commands, asynchronous rendering with half-typed input,
UTF-8 editing, EOF, cancellation, malformed framing and an entire bounty run
against the real TCP server. The binary is also checked on a Linux pseudo-terminal
for idle remote disconnection and terminal restoration.

package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/npons971/42_TAP/internal/server"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeOutput struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	changed chan struct{}
}

func (o *safeOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	n, err := o.buffer.Write(p)
	o.mu.Unlock()
	select {
	case o.changed <- struct{}{}:
	default:
	}
	return n, err
}
func (o *safeOutput) text() string { o.mu.Lock(); defer o.mu.Unlock(); return o.buffer.String() }
func (o *safeOutput) wait(t *testing.T, text string) {
	t.Helper()
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for !strings.Contains(o.text(), text) {
		select {
		case <-o.changed:
		case <-timeout.C:
			t.Fatalf("missing %q: %s", text, o.text())
		}
	}
}
func newOutput() *safeOutput { return &safeOutput{changed: make(chan struct{}, 128)} }

func TestCLIPipedCommandsAndReplies(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	commands := make(chan string, 3)
	go func() {
		defer server.Close()
		fmt.Fprintln(server, "OK hello proto=1")
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			command := scanner.Text()
			commands <- command
			if command == "QUIT" {
				return
			}
			fmt.Fprintln(server, "OK connected")
			fmt.Fprintln(server, "EVT ROOM PRESENCE ENTER bob")
		}
	}()
	out := newOutput()
	if err := Run(context.Background(), client, io.NopCloser(strings.NewReader("CONNECT alice\nLOOK\nQUIT\n")), out, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CONNECT alice", "LOOK", "QUIT"} {
		if got := <-commands; got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
	if !strings.Contains(out.text(), "EVT ROOM PRESENCE ENTER bob") {
		t.Fatal(out.text())
	}
}

func TestCLIAsyncDisplayAndPromptPreservation(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	input, keyboard := io.Pipe()
	defer keyboard.Close()
	out := newOutput()
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), client, input, out, true) }()
	if _, err := keyboard.Write([]byte("CHA")); err != nil {
		t.Fatal(err)
	}
	out.wait(t, "> CHA")
	fmt.Fprintln(server, "EVT GLOBAL CHAT bob hello")
	out.wait(t, "EVT GLOBAL CHAT bob hello\r\n> CHA")
	commands := make(chan string, 2)
	go func() {
		defer server.Close()
		r := bufio.NewScanner(server)
		for r.Scan() {
			commands <- r.Text()
			if r.Text() == "QUIT" {
				return
			}
			fmt.Fprintln(server, "OK")
		}
	}()
	keyboard.Write([]byte("T ROOM hi\rQUIT\r"))
	for _, want := range []string{"CHAT ROOM hi", "QUIT"} {
		if got := <-commands; got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCLICancellationUnblocksInputAndSocket(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	input, keyboard := io.Pipe()
	defer keyboard.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, client, input, io.Discard, false) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation hung")
	}
}

func TestCLIRejectsInvalidWireAndInput(t *testing.T) {
	for _, line := range []string{"unknown\n", "OK bad\r\n", "OK " + strings.Repeat("x", 4096) + "\n", "OK \xff\n"} {
		if _, err := readWireLine(bufio.NewReaderSize(strings.NewReader(line), 4096)); err == nil {
			t.Fatalf("accepted %q", line)
		}
	}
	for _, line := range []string{strings.Repeat("x", 4096), "LOOK\nQUIT", "LOOK\r", "CHAT ROOM \xff"} {
		client, server := net.Pipe()
		if err := sendLine(client, line); err == nil {
			t.Fatalf("accepted input %q", line)
		}
		client.Close()
		server.Close()
	}
}

func TestCLIUnicodeBackspaceAndEOF(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	input, keyboard := io.Pipe()
	out := newOutput()
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), client, input, out, true) }()
	commands := make(chan string, 2)
	go func() {
		defer server.Close()
		r := bufio.NewScanner(server)
		for r.Scan() {
			commands <- r.Text()
			if r.Text() == "QUIT" {
				return
			}
			fmt.Fprintln(server, "OK")
		}
	}()
	keyboard.Write([]byte("TAKE Pomme fraîchX\x7fe\r"))
	keyboard.Close()
	if got := <-commands; got != "TAKE Pomme fraîche" {
		t.Fatal(got)
	}
	if got := <-commands; got != "QUIT" {
		t.Fatal(got)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCLIWithActualServer(t *testing.T) {
	world, err := server.LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), world).Serve(ctx, listener) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	script := `CONNECT traveler
GROUP CREATE
CHAT GROUP hello
MOVE west
TAKE Rusty Sword
MOVE south
MOVE east
MOVE east
TALK Guard Captain
QUEST npc.guard_captain
MOVE north
ATTACK Bandit Leader
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
ATTACK npc.bandit_leader
MOVE south
TALK npc.guard_captain
QUESTS
INVENTORY
QUIT
`
	out := newOutput()
	if err := Run(context.Background(), conn, io.NopCloser(strings.NewReader(script)), out, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OK hello proto=1", "EVT GROUP CHAT traveler hello", `"outcome":"victory"`, "EVT ROOM ITEM REWARD traveler item.ancient_key", `"status":"completed"`, `"item.ancient_key"`} {
		if !strings.Contains(out.text(), want) {
			t.Fatalf("missing %s: %s", want, out.text())
		}
	}
	if strings.Contains(out.text(), "ERR ") {
		t.Fatal(out.text())
	}
}

func TestCLITerminalControlEscapingAndTruncatedReply(t *testing.T) {
	out := newOutput()
	d := &terminalDisplay{out: out}
	if err := d.message("EVT GLOBAL CHAT bob \x1b[2J\a"); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.text(), "\x1b\a") || !strings.Contains(out.text(), `\u001b`) {
		t.Fatal(out.text())
	}
	if _, err := readWireLine(bufio.NewReader(strings.NewReader("OK truncated"))); err == nil || err == io.EOF {
		t.Fatal("unterminated response accepted as clean EOF")
	}
}

func TestCLILowercaseQuitStopsSending(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	commands := make(chan string, 3)
	go func() {
		defer close(commands)
		defer peer.Close()
		fmt.Fprintln(peer, "OK hello proto=1")
		r := bufio.NewScanner(peer)
		for r.Scan() {
			line := r.Text()
			commands <- line
			if strings.EqualFold(line, "QUIT") {
				fmt.Fprintln(peer, "OK bye")
				return
			}
			fmt.Fprintln(peer, "OK connected")
		}
	}()
	out := newOutput()
	if err := Run(context.Background(), client, io.NopCloser(strings.NewReader("CONNECT alice\nquit\nLOOK\n")), out, false); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for c := range commands {
		got = append(got, c)
	}
	if len(got) != 2 || got[1] != "quit" || !strings.Contains(out.text(), "OK bye") {
		t.Fatal("commands after QUIT or missing acknowledgement:", got, out.text())
	}
}

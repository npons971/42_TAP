package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func startTestServer(t *testing.T) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	world := &World{Start: "loc.town_square", Locations: map[string]Room{
		"loc.town_square": {ID: "loc.town_square", Name: "Square", Description: "Town square", Exits: map[string]string{"north": "loc.garden"}},
		"loc.garden":      {ID: "loc.garden", Name: "Garden", Description: "A quiet garden", Exits: map[string]string{"south": "loc.town_square"}},
	}}
	go func() {
		done <- New(slog.New(slog.NewTextHandler(io.Discard, nil)), world).Serve(ctx, listener)
	}()
	return listener.Addr().String(), cancel, done
}

func dialTestClient(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	expectLine(t, reader, "OK hello proto=42TAP/1")
	return conn, reader
}

func expectLine(t *testing.T, reader *bufio.Reader, want string) {
	t.Helper()
	got, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if got != want+"\n" {
		t.Fatalf("got %q, want %q", got, want+"\n")
	}
}

func TestConcurrentClientsAndShutdown(t *testing.T) {
	addr, cancel, done := startTestServer(t)
	defer cancel()
	alice, aliceReader := dialTestClient(t, addr)
	bob, bobReader := dialTestClient(t, addr)

	// An idle first connection must not block replies to a second one.
	if _, err := io.WriteString(bob, "PING\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, bobReader, "ERR unknown_command Unknown command")
	if _, err := io.WriteString(alice, "PING\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, aliceReader, "ERR unknown_command Unknown command")

	if _, err := io.WriteString(alice, "QUIT\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := aliceReader.ReadByte(); err != io.EOF {
		t.Fatalf("QUIT should close only Alice's connection, got %v", err)
	}
	if _, err := io.WriteString(bob, "PING\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, bobReader, "ERR unknown_command Unknown command")

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after context cancellation")
	}
	if _, err := bobReader.ReadByte(); err != io.EOF {
		t.Fatalf("shutdown should close Bob's connection, got %v", err)
	}
}

func TestOversizedLineDoesNotBreakFollowingRequest(t *testing.T) {
	addr, cancel, done := startTestServer(t)
	defer func() {
		cancel()
		<-done
	}()
	conn, reader := dialTestClient(t, addr)
	if _, err := io.WriteString(conn, strings.Repeat("x", maxLineBytes)+"\nPING\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, reader, "ERR invalid_arguments Message too long")
	expectLine(t, reader, "ERR unknown_command Unknown command")
}

func sendCommand(t *testing.T, conn net.Conn, command string) {
	t.Helper()
	if _, err := io.WriteString(conn, command+"\n"); err != nil {
		t.Fatal(err)
	}
}

func expectLook(t *testing.T, reader *bufio.Reader, roomID string, players []string) {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "OK ") {
		t.Fatalf("LOOK returned %q", line)
	}
	var payload struct {
		Room struct {
			ID    string            `json:"id"`
			Exits map[string]string `json:"exits"`
		} `json:"room"`
		Players []string          `json:"players"`
		Items   []json.RawMessage `json:"items"`
		NPCs    []json.RawMessage `json:"npcs"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "OK ")), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Room.ID != roomID || !reflect.DeepEqual(payload.Players, players) {
		t.Fatalf("LOOK room=%q players=%v; want room=%q players=%v", payload.Room.ID, payload.Players, roomID, players)
	}
	if payload.Items == nil || payload.NPCs == nil {
		t.Fatal("LOOK must include empty items and npcs arrays")
	}
}

func TestConnectLookMoveAndPresence(t *testing.T) {
	addr, cancel, done := startTestServer(t)
	defer func() { cancel(); <-done }()
	alice, aliceReader := dialTestClient(t, addr)
	bob, bobReader := dialTestClient(t, addr)

	sendCommand(t, alice, "LOOK")
	expectLine(t, aliceReader, "ERR not_authenticated Send CONNECT first")
	sendCommand(t, alice, "CONNECT")
	expectLine(t, aliceReader, "ERR invalid_arguments CONNECT needs one username")
	sendCommand(t, alice, "CONNECT N")
	expectLine(t, aliceReader, "ERR invalid_arguments Invalid username")
	sendCommand(t, alice, "CONNECT  alice")
	expectLine(t, aliceReader, "ERR invalid_arguments Invalid spacing")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, aliceReader, "OK connected")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, aliceReader, "ERR already_authenticated Already connected")
	sendCommand(t, bob, "MOVE north")
	expectLine(t, bobReader, "ERR not_authenticated Send CONNECT first")
	sendCommand(t, bob, "CONNECT alice")
	expectLine(t, bobReader, "ERR username_taken Username already in use")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, bobReader, "OK connected")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE ENTER bob")

	sendCommand(t, alice, "LOOK")
	expectLook(t, aliceReader, "loc.town_square", []string{"alice", "bob"})
	sendCommand(t, alice, "LOOK extra")
	expectLine(t, aliceReader, "ERR invalid_arguments LOOK takes no arguments")
	sendCommand(t, alice, "MOVE")
	expectLine(t, aliceReader, "ERR invalid_arguments MOVE needs one direction")
	sendCommand(t, alice, "MOVE north")
	expectLine(t, aliceReader, "OK room=loc.garden")
	expectLine(t, bobReader, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, bob, "LOOK")
	expectLook(t, bobReader, "loc.town_square", []string{"bob"})
	sendCommand(t, bob, "MOVE north")
	expectLine(t, bobReader, "OK room=loc.garden")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "LOOK")
	expectLook(t, aliceReader, "loc.garden", []string{"alice", "bob"})
	sendCommand(t, bob, "MOVE west")
	expectLine(t, bobReader, "ERR invalid_direction No exit in that direction")
	sendCommand(t, bob, "QUIT now")
	expectLine(t, bobReader, "ERR invalid_arguments QUIT takes no arguments")
	sendCommand(t, bob, "QUIT")
	if _, err := bobReader.ReadByte(); err != io.EOF {
		t.Fatalf("QUIT should close Bob's socket, got %v", err)
	}
	expectLine(t, aliceReader, "EVT ROOM PRESENCE LEAVE bob")
	sendCommand(t, alice, "LOOK")
	expectLook(t, aliceReader, "loc.garden", []string{"alice"})
}

func TestLoadWorldRejectsBrokenExit(t *testing.T) {
	path := t.TempDir() + "/world.json"
	data := []byte(`{"world":{"start":"loc.square","locations":{"loc.square":{"name":"Square","description":"Here","exits":{"north":"loc.missing"}}}}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWorld(path); err == nil || !strings.Contains(err.Error(), "unknown room") {
		t.Fatalf("expected broken exit error, got %v", err)
	}
}

func TestLoadBundledWorld(t *testing.T) {
	world, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	if world.Start != "loc.town_square" || world.Locations["loc.town_square"].Exits["north"] != "loc.garden" {
		t.Fatalf("unexpected bundled world: %+v", world)
	}
}

func TestChatWhoAndStatus(t *testing.T) {
	addr, cancel, done := startTestServer(t)
	defer func() { cancel(); <-done }()
	alice, aliceReader := dialTestClient(t, addr)
	bob, bobReader := dialTestClient(t, addr)

	sendCommand(t, alice, "WHO")
	expectLine(t, aliceReader, "ERR not_authenticated Send CONNECT first")
	sendCommand(t, alice, "STATUS")
	expectLine(t, aliceReader, "ERR not_authenticated Send CONNECT first")
	sendCommand(t, alice, "CHAT GLOBAL Hello")
	expectLine(t, aliceReader, "ERR not_authenticated Send CONNECT first")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, aliceReader, "OK connected")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, bobReader, "OK connected")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE ENTER bob")

	sendCommand(t, alice, "WHO")
	expectLine(t, aliceReader, `OK {"room":["alice","bob"],"server":2}`)
	sendCommand(t, bob, "MOVE north")
	expectLine(t, bobReader, "OK room=loc.garden")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE LEAVE bob")
	sendCommand(t, alice, "WHO")
	expectLine(t, aliceReader, `OK {"room":["alice"],"server":2}`)

	sendCommand(t, alice, "CHAT ROOM Hello  from the square")
	expectLine(t, aliceReader, "OK")
	expectLine(t, aliceReader, "EVT ROOM CHAT alice Hello  from the square")
	sendCommand(t, bob, "WHO")
	expectLine(t, bobReader, `OK {"room":["bob"],"server":2}`)
	sendCommand(t, bob, "CHAT GLOBAL Hello everyone")
	expectLine(t, bobReader, "OK")
	expectLine(t, bobReader, "EVT GLOBAL CHAT bob Hello everyone")
	expectLine(t, aliceReader, "EVT GLOBAL CHAT bob Hello everyone")

	sendCommand(t, bob, "STATUS")
	expectLine(t, bobReader, `OK {"player":"bob","hp":100,"max_hp":100,"state":"HORS_COMBAT","combat":null}`)
	sendCommand(t, bob, "CHAT ROOM")
	expectLine(t, bobReader, "ERR invalid_arguments CHAT needs a channel and message")
	sendCommand(t, bob, "CHAT PRIVATE Hello")
	expectLine(t, bobReader, "ERR invalid_arguments Invalid chat channel")
	sendCommand(t, bob, "CHAT GROUP Hello")
	expectLine(t, bobReader, "ERR invalid_arguments Not in a group")
}

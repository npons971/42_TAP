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

func startTestServer(t *testing.T, worlds ...*World) (string, context.CancelFunc, <-chan error) {
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
	if len(worlds) > 0 {
		world = worlds[0]
	}
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
	expectLine(t, reader, "OK hello proto=1")
	return conn, reader
}

func expectLine(t *testing.T, reader *bufio.Reader, want string) {
	t.Helper()
	got, err := readTestLine(reader)
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
	expectLine(t, bobReader, `ERR 400 UNKNOWN_COMMAND Unknown command "PING"`)
	if _, err := io.WriteString(alice, "PING\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, aliceReader, `ERR 400 UNKNOWN_COMMAND Unknown command "PING"`)

	if _, err := io.WriteString(alice, "QUIT\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, aliceReader, "OK bye")
	if _, err := aliceReader.ReadByte(); err != io.EOF {
		t.Fatalf("QUIT should close only Alice's connection, got %v", err)
	}
	if _, err := io.WriteString(bob, "PING\n"); err != nil {
		t.Fatal(err)
	}
	expectLine(t, bobReader, `ERR 400 UNKNOWN_COMMAND Unknown command "PING"`)

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
	expectLine(t, reader, "ERR 400 INVALID_ARGUMENTS Line exceeds 4096 bytes")
	expectLine(t, reader, `ERR 400 UNKNOWN_COMMAND Unknown command "PING"`)
}

func sendCommand(t *testing.T, conn net.Conn, command string) {
	t.Helper()
	if _, err := io.WriteString(conn, command+"\n"); err != nil {
		t.Fatal(err)
	}
}

func expectLook(t *testing.T, reader *bufio.Reader, roomID string, players []string) {
	t.Helper()
	line, err := readTestLine(reader)
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

	sendCommand(t, alice, "LOOK DETAILS")
	expectLine(t, aliceReader, "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before LOOK")
	sendCommand(t, alice, "CONNECT")
	expectLine(t, aliceReader, "ERR 400 INVALID_ARGUMENTS Usage: CONNECT <username>")
	sendCommand(t, alice, "CONNECT N")
	expectLine(t, aliceReader, "ERR 400 INVALID_ARGUMENTS CONNECT username must start with a lowercase letter and contain 3-20 lowercase letters, digits or underscores")
	sendCommand(t, alice, "CONNECT ARIS")
	expectLine(t, aliceReader, "ERR 400 INVALID_ARGUMENTS CONNECT username must start with a lowercase letter and contain 3-20 lowercase letters, digits or underscores")
	sendCommand(t, alice, "CONNECT  alice")
	expectLine(t, aliceReader, "ERR 400 INVALID_ARGUMENTS CONNECT must use one space between fixed arguments")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, aliceReader, "OK connected")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, aliceReader, `ERR 409 ALREADY_AUTHENTICATED Connection already uses username "alice"`)
	sendCommand(t, bob, "MOVE north")
	expectLine(t, bobReader, "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before MOVE")
	sendCommand(t, bob, "CONNECT alice")
	expectLine(t, bobReader, `ERR 201 NAME_IN_USE Username "alice" is already connected`)
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, bobReader, "OK connected")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE ENTER bob")

	sendCommand(t, alice, "LOOK DETAILS")
	expectLook(t, aliceReader, "loc.town_square", []string{"alice", "bob"})
	sendCommand(t, alice, "LOOK extra")
	expectLine(t, aliceReader, "ERR 400 INVALID_ARGUMENTS LOOK takes no arguments")
	sendCommand(t, alice, "MOVE")
	expectLine(t, aliceReader, "ERR 400 INVALID_ARGUMENTS Usage: MOVE <direction>; available from loc.town_square: north")
	sendCommand(t, alice, "MOVE north")
	expectLine(t, aliceReader, "OK room=loc.garden")
	expectLine(t, bobReader, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, bob, "LOOK DETAILS")
	expectLook(t, bobReader, "loc.town_square", []string{"bob"})
	sendCommand(t, bob, "MOVE north")
	expectLine(t, bobReader, "OK room=loc.garden")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "LOOK DETAILS")
	expectLook(t, aliceReader, "loc.garden", []string{"alice", "bob"})
	sendCommand(t, bob, "MOVE west")
	expectLine(t, bobReader, "ERR 301 NO_EXIT No west exit from loc.garden; available directions: south")
	sendCommand(t, bob, "QUIT now")
	expectLine(t, bobReader, "ERR 400 INVALID_ARGUMENTS QUIT takes no arguments")
	sendCommand(t, bob, "QUIT")
	expectLine(t, bobReader, "OK bye")
	if _, err := bobReader.ReadByte(); err != io.EOF {
		t.Fatalf("QUIT should close Bob's socket, got %v", err)
	}
	expectLine(t, aliceReader, "EVT ROOM PRESENCE LEAVE bob")
	sendCommand(t, alice, "LOOK DETAILS")
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

	sendCommand(t, alice, "WHO DETAILS")
	expectLine(t, aliceReader, "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before WHO")
	sendCommand(t, alice, "STATUS")
	expectLine(t, aliceReader, "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before STATUS")
	sendCommand(t, alice, "CHAT GLOBAL Hello")
	expectLine(t, aliceReader, "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before CHAT")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, aliceReader, "OK connected")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, bobReader, "OK connected")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE ENTER bob")

	sendCommand(t, alice, "WHO DETAILS")
	expectLine(t, aliceReader, `OK {"room":["alice","bob"],"server":2}`)
	sendCommand(t, bob, "MOVE north")
	expectLine(t, bobReader, "OK room=loc.garden")
	expectLine(t, aliceReader, "EVT ROOM PRESENCE LEAVE bob")
	sendCommand(t, alice, "WHO DETAILS")
	expectLine(t, aliceReader, `OK {"room":["alice"],"server":2}`)

	sendCommand(t, alice, "CHAT ROOM Hello  from the square")
	expectLine(t, aliceReader, "OK")
	expectLine(t, aliceReader, "EVT ROOM CHAT alice Hello  from the square")
	sendCommand(t, bob, "WHO DETAILS")
	expectLine(t, bobReader, `OK {"room":["bob"],"server":2}`)
	sendCommand(t, bob, "CHAT GLOBAL Hello everyone")
	expectLine(t, bobReader, "OK")
	expectLine(t, bobReader, "EVT GLOBAL CHAT bob Hello everyone")
	expectLine(t, aliceReader, "EVT GLOBAL CHAT bob Hello everyone")

	sendCommand(t, bob, "STATUS")
	expectLine(t, bobReader, `OK {"status":"healthy","player":"bob","hp":100,"max_hp":100,"state":"HORS_COMBAT","combat":null}`)
	sendCommand(t, bob, "CHAT ROOM")
	expectLine(t, bobReader, "ERR 400 INVALID_ARGUMENTS Usage: CHAT <GLOBAL|ROOM|GROUP> <message>")
	sendCommand(t, bob, "CHAT PRIVATE Hello")
	expectLine(t, bobReader, `ERR 400 INVALID_ARGUMENTS CHAT channel "PRIVATE" is invalid; use GLOBAL, ROOM or GROUP`)
	sendCommand(t, bob, "CHAT GROUP Hello")
	expectLine(t, bobReader, "ERR 401 NOT_IN_GROUP CHAT GROUP requires group membership; use GROUP CREATE or GROUP JOIN")
}

func TestCommandErrorsAreSpecific(t *testing.T) {
	addr, cancel, done := startTestServer(t)
	defer func() { cancel(); <-done }()
	conn, reader := dialTestClient(t, addr)

	for _, step := range []struct{ command, reply string }{
		{"", "ERR 400 INVALID_ARGUMENTS Empty command"},
		{"LOOK\r", "ERR 400 INVALID_ARGUMENTS Use LF without CR"},
		{"CONNECT", "ERR 400 INVALID_ARGUMENTS Usage: CONNECT <username>"},
		{"CONNECT alice", "OK connected"},
		{"WHO extra", "ERR 400 INVALID_ARGUMENTS WHO takes no arguments"},
		{"STATUS extra", "ERR 400 INVALID_ARGUMENTS STATUS takes no arguments"},
		{"MOVE", "ERR 400 INVALID_ARGUMENTS Usage: MOVE <direction>; available from loc.town_square: north"},
		{"MOVE diagonal", `ERR 301 NO_EXIT Direction "diagonal" is unknown; available from loc.town_square: north`},
		{"MOVE west", "ERR 301 NO_EXIT No west exit from loc.town_square; available directions: north"},
		{"CHAT ROOM ", "ERR 400 INVALID_ARGUMENTS CHAT message cannot be empty"},
		{"ATTACK npc.guard", `ERR 404 NPC_NOT_FOUND ATTACK target "npc.guard" is not in loc.town_square; use LOOK`},
		{"HO", `ERR 400 UNKNOWN_COMMAND Unknown command "HO"`},
		{"QUIT extra", "ERR 400 INVALID_ARGUMENTS QUIT takes no arguments"},
		{"WHO", "OK players=1"},
	} {
		sendCommand(t, conn, step.command)
		expectLine(t, reader, step.reply)
	}
}

// Gameplay tests ignore count notifications; RFC event tests read the raw stream.
func readTestLine(r *bufio.Reader) (string, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil || !strings.HasPrefix(line, "EVT STATS ") {
			return line, err
		}
	}
}

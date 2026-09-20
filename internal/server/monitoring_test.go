package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestCommandRateLimiter(t *testing.T) {
	var b tokenBucket
	now := time.Unix(1, 0)
	for i := 0; i < commandBurst; i++ {
		if !b.allow(now) {
			t.Fatalf("burst rejected at %d", i)
		}
	}
	if b.allow(now) {
		t.Fatal("flood accepted")
	}
	now = now.Add(50 * time.Millisecond)
	if !b.allow(now) || b.allow(now) {
		t.Fatal("refill is not 20 commands per second")
	}
	now = now.Add(time.Hour)
	for i := 0; i < commandBurst; i++ {
		if !b.allow(now) {
			t.Fatal("burst not refilled")
		}
	}
	if b.allow(now) {
		t.Fatal("refill exceeded cap")
	}
}

func TestStructuredLogsAndRapidConnections(t *testing.T) {
	world, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	s := New(slog.New(slog.NewJSONHandler(&logs, nil)), world)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	// Never inspect the buffer until the server and all writers have stopped.
	for i := 0; i < 6; i++ {
		conn, r := dialTestClient(t, listener.Addr().String())
		if i == 0 {
			sendCommand(t, conn, "CONNECT alice")
			expectLine(t, r, "OK connected")
			sendCommand(t, conn, "UNKNOWN foo")
			expectLine(t, r, `ERR 400 UNKNOWN_COMMAND Unknown command "UNKNOWN"`)
			sendCommand(t, conn, "TALKJSON npc.guard")
			var reply map[string]any
			readWorldReply(t, r, &reply)
		}
		sendCommand(t, conn, "QUIT")
		expectLine(t, r, "OK bye")
		if _, err := readTestLine(r); err != io.EOF {
			t.Fatal(err)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&logs)
	events := map[string]bool{}
	commands, responses, errorsSeen, interactions := 0, 0, 0, 0
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if entry["time"] == nil || entry["level"] == nil {
			t.Fatal("missing timestamps/levels")
		}
		if event, ok := entry["event"].(string); ok {
			events[event] = true
		}
		if entry["msg"] == "command received" {
			commands++
			if entry["remote_addr"] == nil || entry["params"] == nil {
				t.Fatal("missing command audit")
			}
		}
		if entry["msg"] == "server response" {
			responses++
			if entry["response"] == nil {
				t.Fatal("missing response body")
			}
		}
		if entry["error_code"] == "400" {
			errorsSeen++
		}
		if entry["msg"] == "NPC interaction" {
			interactions++
		}
	}
	if commands != 9 || responses < 9 || errorsSeen != 1 || interactions != 1 || !events["RAPID_RECONNECT"] {
		t.Fatalf("incomplete logs: commands=%d responses=%d errors=%d interactions=%d events=%v", commands, responses, errorsSeen, interactions, events)
	}
}

func TestCommandFloodClosesOnlyAbusiveSession(t *testing.T) {
	_, addr, stop := lifecycleServer(t)
	defer stop()
	abusive, r := dialTestClient(t, addr)
	healthy, hr := dialTestClient(t, addr)
	limited := false
	for i := 0; i < 512; i++ {
		sendCommand(t, abusive, "PING")
		line, err := readTestLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if line == "ERR 429 RATE_LIMITED Command flood detected; connection closing\n" {
			limited = true
			break
		}
		if line != "ERR 400 UNKNOWN_COMMAND Unknown command \"PING\"\n" {
			t.Fatal(line)
		}
	}
	if !limited {
		t.Fatal("sustained command burst was not limited")
	}
	if _, err := readTestLine(r); err != io.EOF {
		t.Fatalf("flooded session not closed: %v", err)
	}
	sendCommand(t, healthy, "CONNECT healthy")
	expectLine(t, hr, "OK connected")
	sendCommand(t, healthy, "WHO DETAILS")
	expectLine(t, hr, `OK {"room":["healthy"],"server":1}`)
}

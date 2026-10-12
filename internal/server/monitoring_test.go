package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestRejectedConnectionsAreLoggedTCP(t *testing.T) {
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
	defer cancel()
	for i := 0; i < maxConnections; i++ {
		dialTestClient(t, listener.Addr().String())
	}
	// Isolate abuse monitoring for the rejected attempts from initial admission.
	s.mu.Lock()
	s.connectionHistory = make(map[string]connectionWindow)
	s.mu.Unlock()
	rejected := make(map[string]map[string]bool)
	for i := 0; i < 6; i++ {
		conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		rejected[conn.LocalAddr().String()] = make(map[string]bool)
		r := bufio.NewReader(conn)
		rfcLine(t, r, "OK hello proto=1")
		rfcLine(t, r, "ERR 503 CONNECTION_LIMIT Server connection limit reached")
		if _, err := r.ReadByte(); err != io.EOF {
			t.Fatalf("rejected connection not closed: %v", err)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// Inspect logs only after all session writers have stopped.
	rapid := false
	decoder := json.NewDecoder(&logs)
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if entry["event"] == "RAPID_RECONNECT" && entry["client_ip"] == "127.0.0.1" {
			rapid = true
		}
		remote, _ := entry["remote_addr"].(string)
		seen, ok := rejected[remote]
		if !ok {
			continue
		}
		if entry["time"] == nil || entry["level"] == nil {
			t.Fatal("rejected connection audit lacks timestamp or level")
		}
		switch entry["msg"] {
		case "client connected":
			seen["connected"] = true
		case "connection rejected":
			if entry["error_code"] != "503" || entry["level"] != "WARN" {
				t.Fatalf("invalid rejection audit: %v", entry)
			}
			seen["rejected"] = true
		case "server response":
			if entry["response"] == "OK hello proto=1" {
				seen["greeting"] = true
			}
			if entry["response"] == "ERR 503 CONNECTION_LIMIT Server connection limit reached" && entry["error_code"] == "503" && entry["level"] == "WARN" {
				seen["error"] = true
			}
		case "client disconnected":
			if entry["reason"] != "connection_limit" || entry["duration_ms"] == nil {
				t.Fatalf("invalid rejected disconnection audit: %v", entry)
			}
			seen["disconnected"] = true
		}
	}
	for remote, seen := range rejected {
		for _, event := range []string{"connected", "rejected", "greeting", "error", "disconnected"} {
			if !seen[event] {
				t.Errorf("connection %s lacks %s audit", remote, event)
			}
		}
	}
	if !rapid {
		t.Fatal("rapid rejected connections were not detected")
	}
}

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

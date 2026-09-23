package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)
type eventItem struct {
	evtType string
	payload string
}

func ensureServer(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:4242", 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return
	}
	repoRoot, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	goBin := filepath.Join(repoRoot, "go")
	cmd := exec.Command(goBin, "run", "./cmd/server", "-addr", "127.0.0.1:4242")
	cmd.Dir = repoRoot
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to auto-start server: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:4242", 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Server did not become ready within 5 seconds")
}

// waitForEvent waits for a specific event type and condition
func waitForEvent(t *testing.T, ch <-chan eventItem, timeout time.Duration, match func(evtType, payload string) bool) (string, string) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if match(ev.evtType, ev.payload) {
				return ev.evtType, ev.payload
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event matching criteria")
			return "", ""
		}
	}
}

func TestAppLiveServerIntegration(t *testing.T) {
	ensureServer(t)

	app := NewApp()

	// Channel to collect events via OnEvent hook
	eventsChan := make(chan eventItem, 100)
	var mu sync.Mutex
	allEvents := make([]eventItem, 0)

	app.OnEvent = func(evtType, payload string) {
		mu.Lock()
		allEvents = append(allEvents, eventItem{evtType: evtType, payload: payload})
		mu.Unlock()
		select {
		case eventsChan <- eventItem{evtType: evtType, payload: payload}:
		default:
		}
	}

	// 1. Connect
	testUser := fmt.Sprintf("test_gui_%d", time.Now().Unix()%10000)
	if err := app.Connect("127.0.0.1", 4242, testUser); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		_ = app.Disconnect()
	}()

	if !app.IsConnected() {
		t.Fatalf("Expected app to be connected")
	}
	if app.GetUsername() != testUser {
		t.Fatalf("Expected username %s, got %s", testUser, app.GetUsername())
	}

	// Wait for "OK connected"
	_, payload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && pl == "connected"
	})
	if payload != "connected" {
		t.Fatalf("Expected 'connected' payload, got %q", payload)
	}

	// 2. Initial LOOK
	if err := app.Look(); err != nil {
		t.Fatalf("Look failed: %v", err)
	}
	_, lookPayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.HasPrefix(pl, "{")
	})
	var lookData struct {
		Room struct {
			ID    string            `json:"id"`
			Name  string            `json:"name"`
			Exits map[string]string `json:"exits"`
		} `json:"room"`
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Players []string `json:"players"`
	}
	if err := json.Unmarshal([]byte(lookPayload), &lookData); err != nil {
		t.Fatalf("Failed to parse LOOK payload: %v", err)
	}
	if lookData.Room.ID != "loc.town_square" {
		t.Fatalf("Expected start room loc.town_square, got %s", lookData.Room.ID)
	}

	// 3. Move North to Garden
	if err := app.Move("north"); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	_, movePayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.HasPrefix(pl, "room=")
	})
	if movePayload != "room=loc.garden" {
		t.Fatalf("Expected room=loc.garden, got %s", movePayload)
	}

	// 4. LOOK in garden
	if err := app.Look(); err != nil {
		t.Fatalf("Look failed: %v", err)
	}
	_, gardenLook := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.Contains(pl, "loc.garden")
	})
	if !strings.Contains(gardenLook, "loc.garden") {
		t.Fatalf("Expected garden room in payload, got %s", gardenLook)
	}

	// 5. Take item in garden (rare_herbs)
	if err := app.Take("item.rare_herbs"); err != nil {
		t.Fatalf("Take failed: %v", err)
	}
	_, takePayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.HasPrefix(pl, "taken=")
	})
	if takePayload != "taken=item.rare_herbs" {
		t.Fatalf("Expected taken=item.rare_herbs, got %s", takePayload)
	}

	// Verify EVT was received
	waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_evt" && strings.Contains(pl, "ITEM TAKE") && strings.Contains(pl, "item.rare_herbs")
	})

	// 6. Check Inventory
	if err := app.Inventory(); err != nil {
		t.Fatalf("Inventory failed: %v", err)
	}
	_, invPayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.HasPrefix(pl, "[")
	})
	if !strings.Contains(invPayload, "item.rare_herbs") {
		t.Fatalf("Expected inventory to contain rare herbs, got %s", invPayload)
	}

	// 7. Drop item
	if err := app.Drop("item.rare_herbs"); err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	_, dropPayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.HasPrefix(pl, "dropped=")
	})
	if dropPayload != "dropped=item.rare_herbs" {
		t.Fatalf("Expected dropped=item.rare_herbs, got %s", dropPayload)
	}

	// 8. Chat
	if err := app.Chat("GLOBAL", "Integration test message"); err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_evt" && strings.Contains(pl, "GLOBAL CHAT") && strings.Contains(pl, "Integration test message")
	})

	// 9. WHO
	if err := app.Who(); err != nil {
		t.Fatalf("Who failed: %v", err)
	}
	_, whoPayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.HasPrefix(pl, "players=")
	})
	if !strings.HasPrefix(whoPayload, "players=") {
		t.Fatalf("Expected WHO payload to start with players=, got %s", whoPayload)
	}

	// 10. STATUS
	if err := app.Status(); err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	_, statusPayload := waitForEvent(t, eventsChan, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && strings.Contains(pl, "hp")
	})
	if !strings.Contains(statusPayload, "100") {
		t.Fatalf("Expected STATUS payload to show 100 HP, got %s", statusPayload)
	}

	// 11. Disconnect
	if err := app.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if app.IsConnected() {
		t.Fatalf("Expected app to not be connected after Disconnect")
	}
}

func TestAppMultiClientPresenceAndChat(t *testing.T) {
	ensureServer(t)

	// Client 1: novanns
	app1 := NewApp()
	events1 := make(chan eventItem, 100)
	app1.OnEvent = func(evtType, payload string) {
		select {
		case events1 <- eventItem{evtType: evtType, payload: payload}:
		default:
		}
	}
	user1 := fmt.Sprintf("u1_%d", time.Now().Unix()%10000)
	if err := app1.Connect("127.0.0.1", 4242, user1); err != nil {
		t.Fatalf("Client 1 connect failed: %v", err)
	}
	defer func() { _ = app1.Disconnect() }()
	waitForEvent(t, events1, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && pl == "connected"
	})

	// Client 2: aris
	app2 := NewApp()
	events2 := make(chan eventItem, 100)
	app2.OnEvent = func(evtType, payload string) {
		select {
		case events2 <- eventItem{evtType: evtType, payload: payload}:
		default:
		}
	}
	user2 := fmt.Sprintf("u2_%d", time.Now().Unix()%10000)
	if err := app2.Connect("127.0.0.1", 4242, user2); err != nil {
		t.Fatalf("Client 2 connect failed: %v", err)
	}
	defer func() { _ = app2.Disconnect() }()
	waitForEvent(t, events2, 2*time.Second, func(et, pl string) bool {
		return et == "server_ok" && pl == "connected"
	})

	// Client 1 should receive presence ENTER for Client 2
	waitForEvent(t, events1, 2*time.Second, func(et, pl string) bool {
		return et == "server_evt" && strings.Contains(pl, "PRESENCE ENTER") && strings.Contains(pl, user2)
	})

	// Client 2 sends ROOM chat
	chatMsg := "Salutations dans la place du village"
	if err := app2.Chat("ROOM", chatMsg); err != nil {
		t.Fatalf("Client 2 chat failed: %v", err)
	}

	// Client 1 should receive ROOM chat from Client 2
	waitForEvent(t, events1, 2*time.Second, func(et, pl string) bool {
		return et == "server_evt" && strings.Contains(pl, "ROOM CHAT") && strings.Contains(pl, user2) && strings.Contains(pl, chatMsg)
	})

	// Client 2 moves north
	if err := app2.Move("north"); err != nil {
		t.Fatalf("Client 2 move failed: %v", err)
	}

	// Client 1 should receive presence LEAVE for Client 2
	waitForEvent(t, events1, 2*time.Second, func(et, pl string) bool {
		return et == "server_evt" && strings.Contains(pl, "PRESENCE LEAVE") && strings.Contains(pl, user2)
	})
}

func TestGUIProtocolCommandsAndDisconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan string, 16)
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		fmt.Fprintln(conn, "OK hello proto=1")
		r := bufio.NewScanner(conn)
		for r.Scan() {
			line := r.Text()
			commands <- line
			if line == "QUIT" {
				fmt.Fprintln(conn, "OK bye")
				done <- nil
				return
			}
			fmt.Fprintln(conn, "OK")
			fmt.Fprintln(conn, "EVT STATS players=1")
		}
		done <- r.Err()
	}()
	app := NewApp()
	addr := listener.Addr().(*net.TCPAddr)
	if err := app.Connect("127.0.0.1", addr.Port, "élise"); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		action func() error
		want   string
	}{
		{nil, "CONNECT élise"}, {app.Look, "LOOK DETAILS"}, {app.Inventory, "INVENTORY DETAILS"},
		{func() error { return app.Talk("Village Guard") }, "TALKJSON Village Guard"},
		{app.Who, "WHO"}, {app.Quests, "QUESTS"}, {func() error { return app.Quest("npc.herbalist") }, "QUEST npc.herbalist"},
		{func() error { return app.Group("CREATE", "") }, "GROUP CREATE"},
		{func() error { return app.Group("INVITE", "bob") }, "GROUP INVITE bob"},
		{func() error { return app.Group("JOIN", "bob") }, "GROUP JOIN bob"},
	}
	for _, step := range steps {
		if step.action != nil {
			if err := step.action(); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case got := <-commands:
			if got != step.want {
				t.Fatalf("got %s, want %s", got, step.want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("GUI send blocked")
		}
	}
	for _, cmd := range []string{"LOOK\nQUIT", "CHAT GLOBAL \xff", strings.Repeat("x", 4096)} {
		if app.SendCommand(cmd) == nil {
			t.Fatalf("invalid frame accepted: %q", cmd)
		}
	}
	disconnected := make(chan error, 1)
	go func() { disconnected <- app.Disconnect() }()
	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Fatal("GUI Disconnect deadlocked")
	}
	if app.IsConnected() {
		t.Fatal("still connected")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive QUIT")
	}
}

func TestGUIConcurrentConnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprintln(conn, "OK hello proto=1")
		r := bufio.NewScanner(conn)
		for r.Scan() {
			if r.Text() == "QUIT" {
				return
			}
			fmt.Fprintln(conn, "OK connected")
		}
	}()
	app := NewApp()
	port := listener.Addr().(*net.TCPAddr).Port
	errs := make(chan error, 2)
	gate := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() { <-gate; errs <- app.Connect("127.0.0.1", port, "alice") }()
	}
	close(gate)
	success := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err == nil {
				success++
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent connect blocked")
		}
	}
	if success != 1 {
		t.Fatalf("%d simultaneous connections accepted", success)
	}
	app.Disconnect()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connection leaked")
	}
}

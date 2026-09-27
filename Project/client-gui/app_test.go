package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type eventItem struct {
	evtType string
	payload string
}

func ensureServer(t *testing.T) *net.TCPAddr {
	t.Helper()
	repoRoot, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	serverBin := filepath.Join(t.TempDir(), "tap-server")
	goBin := filepath.Join(repoRoot, "go")
	buildCmd := exec.Command(goBin, "build", "-o", serverBin, "./cmd/server")
	buildCmd.Dir = repoRoot
	buildCmd.Env = append(os.Environ(), "GOPATH="+filepath.Join(repoRoot, ".go-work"), "GOCACHE="+filepath.Join(repoRoot, ".go-cache"))
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build server: %v (%s)", err, string(out))
	}
	cmd := exec.Command(serverBin, "-addr", "127.0.0.1:0")
	cmd.Dir = repoRoot
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to auto-start server: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	ready := make(chan string, 1)
	finished := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var entry struct {
				Message string `json:"msg"`
				Address string `json:"addr"`
			}
			if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.Message == "server listening" {
				ready <- entry.Address
			}
		}
		finished <- scanner.Err()
	}()
	select {
	case address := <-ready:
		addr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		return addr
	case err := <-finished:
		t.Fatalf("Server exited before readiness: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not become ready within 5 seconds")
	}
	return nil
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
	addr := ensureServer(t)

	app := NewApp()

	// Channel to collect events via OnEvent hook
	eventsChan := make(chan eventItem, 100)
	app.OnEvent = func(evtType, payload string) {
		select {
		case eventsChan <- eventItem{evtType: evtType, payload: payload}:
		default:
		}
	}

	// 1. Connect
	testUser := fmt.Sprintf("test_gui_%d", time.Now().Unix()%10000)
	if err := app.Connect("127.0.0.1", addr.Port, testUser); err != nil {
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
	addr := ensureServer(t)

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
	if err := app1.Connect("127.0.0.1", addr.Port, user1); err != nil {
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
	if err := app2.Connect("127.0.0.1", addr.Port, user2); err != nil {
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
			if strings.HasPrefix(line, "CONNECT ") {
				fmt.Fprintln(conn, "OK connected")
			} else {
				fmt.Fprintln(conn, "OK")
			}
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

func TestGUIConnectionValidation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var accepted atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			fmt.Fprintln(conn, "OK hello proto=1")
			bufio.NewReader(conn).ReadString('\n')
			fmt.Fprintln(conn, "OK connected")
			conn.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	cases := []struct {
		name, host, username string
		port                 int
	}{
		{"empty username", "127.0.0.1", "", port},
		{"uppercase unicode username", "127.0.0.1", "Élise", port},
		{"uppercase username", "127.0.0.1", "Alice", port},
		{"short username", "127.0.0.1", "ab", port},
		{"digit prefix", "127.0.0.1", "1alice", port},
		{"multiple words", "127.0.0.1", "alice bob", port},
		{"newline injection", "127.0.0.1", "alice\nQUIT", port},
		{"carriage return injection", "127.0.0.1", "alice\rQUIT", port},
		{"long username", "127.0.0.1", strings.Repeat("a", 21), port},
		{"empty host", "", "alice", port},
		{"blank host", "   ", "alice", port},
		{"zero port", "127.0.0.1", "alice", 0},
		{"negative port", "127.0.0.1", "alice", -1},
		{"large port", "127.0.0.1", "alice", 65536},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp()
			if err := app.Connect(tc.host, tc.port, tc.username); err == nil {
				app.Disconnect()
				t.Fatal("invalid connection arguments accepted")
			}
			if app.IsConnected() || app.GetUsername() != "" {
				t.Fatal("invalid arguments left an active session")
			}
		})
	}
	listener.Close()
	<-done
	if got := accepted.Load(); got != 0 {
		t.Fatalf("invalid arguments opened %d TCP connections", got)
	}
}

func TestGUIHandshakeTimeout(t *testing.T) {
	for _, stage := range []string{"greeting", "registration"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				if stage == "registration" {
					fmt.Fprintln(conn, "OK hello proto=1")
				}
				<-stop
			}()
			app := NewApp()
			started := time.Now()
			err = app.Connect("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "alice")
			elapsed := time.Since(started)
			if err == nil {
				app.Disconnect()
				t.Fatal("silent server accepted the connection")
			}
			if elapsed < 4*time.Second || elapsed > 8*time.Second {
				t.Fatalf("handshake timeout took %s; expected about five seconds", elapsed)
			}
			if app.IsConnected() || app.GetUsername() != "" {
				t.Fatal("failed handshake left an active session")
			}
		})
	}
}

func TestGUIRejectedHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprintln(conn, "OK hello proto=1")
		bufio.NewReader(conn).ReadString('\n')
		fmt.Fprintln(conn, "ERR 409 USERNAME_TAKEN Username already connected")
	}()
	app := NewApp()
	err = app.Connect("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "alice")
	if err == nil || !strings.Contains(err.Error(), "USERNAME_TAKEN") {
		t.Fatalf("server rejection was not returned: %v", err)
	}
	if app.IsConnected() || app.GetUsername() != "" {
		t.Fatal("rejected handshake left an active session")
	}
}

func TestGUIInvalidHandshakeFrames(t *testing.T) {
	cases := []struct{ name, frame string }{
		{"unknown prefix", "UNKNOWN greeting\n"},
		{"invalid UTF-8", "OK hello \xff\n"},
		{"CRLF", "OK hello proto=1\r\n"},
		{"oversized", "OK " + strings.Repeat("x", 4096) + "\n"},
		{"unterminated", "OK hello proto=1"},
		{"unexpected greeting", "OK something else\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				fmt.Fprint(conn, tc.frame)
			}()
			app := NewApp()
			if err := app.Connect("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "alice"); err == nil {
				app.Disconnect()
				t.Fatal("invalid server handshake accepted")
			}
			if app.IsConnected() || app.GetUsername() != "" {
				t.Fatal("invalid handshake retained session state")
			}
		})
	}
}

func TestGUIDisconnectClearsSessionOnce(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote=%t", remote), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			release := make(chan struct{})
			defer close(release)
			peerDone := make(chan struct{})
			go func() {
				defer close(peerDone)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				reader := bufio.NewReader(conn)
				fmt.Fprintln(conn, "OK hello proto=1")
				reader.ReadString('\n')
				fmt.Fprintln(conn, "OK connected")
				if remote {
					<-release
				} else {
					reader.ReadString('\n')
				}
			}()
			var count atomic.Int32
			events := make(chan eventItem, 8)
			app := NewApp()
			app.OnEvent = func(kind, payload string) {
				if kind == "disconnected" {
					count.Add(1)
					events <- eventItem{kind, payload}
				}
			}
			if err := app.Connect("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "alice"); err != nil {
				t.Fatal(err)
			}
			if remote {
				release <- struct{}{}
			} else if err := app.Disconnect(); err != nil {
				t.Fatal(err)
			}
			waitForEvent(t, events, 2*time.Second, func(kind, payload string) bool { return kind == "disconnected" })
			if app.IsConnected() || app.GetUsername() != "" {
				t.Fatal("disconnection retained session state")
			}
			if err := app.Disconnect(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-peerDone:
			case <-time.After(2 * time.Second):
				t.Fatal("peer stayed connected")
			}
			select {
			case <-events:
				t.Fatal("duplicate disconnection event")
			case <-time.After(50 * time.Millisecond):
			}
			if count.Load() != 1 {
				t.Fatalf("got %d disconnection events", count.Load())
			}
		})
	}
}

func TestGUIDisconnectCallbackCanDisconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		fmt.Fprintln(conn, "OK hello proto=1")
		reader.ReadString('\n')
		fmt.Fprintln(conn, "OK connected")
		reader.ReadString('\n')
	}()
	app := NewApp()
	reentered := make(chan error, 1)
	app.OnEvent = func(kind, payload string) {
		if kind == "disconnected" {
			reentered <- app.Disconnect()
		}
	}
	if err := app.Connect("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "alice"); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- app.Disconnect() }()
	for _, result := range []<-chan error{reentered, finished} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("disconnected callback deadlocked when calling Disconnect")
		}
	}
	if app.IsConnected() || app.GetUsername() != "" {
		t.Fatal("reentrant disconnection retained session state")
	}
}

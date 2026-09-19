package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"strings"
	"sync"
	"testing"
)

func lifecycleServer(t *testing.T) (*Server, string, func()) {
	t.Helper()
	w, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), w)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	return s, listener.Addr().String(), func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestConsumablesTCP(t *testing.T) {
	s, addr, stop := lifecycleServer(t)
	defer stop()
	conn, r := dialTestClient(t, addr)
	sendCommand(t, conn, "USE item.apple")
	expectLine(t, r, "ERR not_authenticated Send CONNECT <username> before USE")
	sendCommand(t, conn, "CONNECT alice")
	expectLine(t, r, "OK connected")
	for _, tc := range []struct{ command, want string }{
		{"USE", "ERR invalid_arguments Usage: USE <item_id or full display name>"},
		{"USE   ", "ERR invalid_arguments Usage: USE <item_id or full display name>"},
		{"USE item.apple ", "ERR invalid_arguments USE target must not contain tabs or surrounding spaces"},
		{"USE Fresh\tApple", "ERR invalid_arguments USE target must not contain tabs or surrounding spaces"},
		{"USE item.apple", `ERR not_in_inventory USE target "item.apple" is not in your inventory; use INVENTORY`},
		{"USE item.vigor_potion", `ERR not_in_inventory USE target "item.vigor_potion" is not in your inventory; use INVENTORY`},
	} {
		sendCommand(t, conn, tc.command)
		expectLine(t, r, tc.want)
	}
	sendCommand(t, conn, "MOVE east")
	expectLine(t, r, "OK room=loc.market")
	sendCommand(t, conn, "TAKE item.apple")
	expectLine(t, r, "OK taken=item.apple")
	expectLine(t, r, "EVT ROOM ITEM TAKE alice item.apple")
	sendCommand(t, conn, "USE Fresh Apple")
	expectLine(t, r, "ERR health_full USE cannot restore HP: health is already full; item kept")
	sendCommand(t, conn, "INVENTORY")
	expectLine(t, r, `OK [{"id":"item.apple","name":"Fresh Apple"}]`)
	s.mu.Lock()
	s.players["alice"].hp = 95
	s.mu.Unlock()
	sendCommand(t, conn, "USE FRESH APPLE")
	expectLine(t, r, "OK used=item.apple hp=100/100")
	expectLine(t, r, "EVT ROOM ITEM USE alice item.apple")
	sendCommand(t, conn, "STATUS")
	expectLine(t, r, `OK {"player":"alice","hp":100,"max_hp":100,"state":"HORS_COMBAT","combat":null}`)
	sendCommand(t, conn, "INVENTORY")
	expectLine(t, r, "OK []")
	sendCommand(t, conn, "LOOK")
	expectRoomItems(t, r, []string{})
	for _, command := range []string{"USE", "DROP", "TAKE"} {
		sendCommand(t, conn, command+" item.apple")
		code, message := "not_in_inventory", "is not in your inventory; use INVENTORY"
		if command == "TAKE" {
			code, message = "item_not_found", "is not in loc.market; use LOOK"
		}
		expectLine(t, r, fmt.Sprintf("ERR %s %s target %q %s", code, command, "item.apple", message))
	}
	sendCommand(t, conn, "MOVE north")
	expectLine(t, r, "OK room=loc.tavern")
	sendCommand(t, conn, "TAKE Frothy Ale")
	expectLine(t, r, "OK taken=item.ale")
	expectLine(t, r, "EVT ROOM ITEM TAKE alice item.ale")
	s.mu.Lock()
	s.players["alice"].hp = 20
	s.mu.Unlock()
	sendCommand(t, conn, "USE item.ale")
	expectLine(t, r, "OK used=item.ale hp=35/100")
	expectLine(t, r, "EVT ROOM ITEM USE alice item.ale")
	sendCommand(t, conn, "MOVE west")
	expectLine(t, r, "OK room=loc.dark_alley")
	sendCommand(t, conn, "TAKE item.rusty_sword")
	expectLine(t, r, "OK taken=item.rusty_sword")
	expectLine(t, r, "EVT ROOM ITEM TAKE alice item.rusty_sword")
	sendCommand(t, conn, "USE Rusty Sword")
	expectLine(t, r, "ERR item_not_usable Item item.rusty_sword cannot be consumed with USE")
	sendCommand(t, conn, "INVENTORY")
	expectLine(t, r, `OK [{"id":"item.rusty_sword","name":"Rusty Sword"}]`)
}

func TestConcurrentUseAndDrop(t *testing.T) {
	for _, competing := range []string{"USE", "DROP"} {
		t.Run(competing, func(t *testing.T) {
			w, err := LoadWorld("../../data/world.json")
			if err != nil {
				t.Fatal(err)
			}
			s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), w)
			c := &client{username: "alice", roomID: w.Start, hp: 10, outbox: make(chan string, 16)}
			s.itemLocations["item.apple"] = itemLocation{owner: c}
			start := make(chan struct{})
			var wg sync.WaitGroup
			for _, verb := range []string{"USE", competing} {
				wg.Add(1)
				go func(verb string) { defer wg.Done(); <-start; s.handleCommand(c, verb+" item.apple") }(verb)
			}
			close(start)
			wg.Wait()
			ok, fail := 0, 0
			for len(c.outbox) > 0 {
				line := <-c.outbox
				if strings.HasPrefix(line, "OK ") {
					ok++
				}
				if strings.HasPrefix(line, "ERR not_in_inventory ") {
					fail++
				}
			}
			if ok != 1 || fail != 1 {
				t.Fatalf("ok=%d fail=%d", ok, fail)
			}
			loc := s.itemLocations["item.apple"]
			if loc.consumed {
				if c.hp != 20 || loc.owner != nil || loc.roomID != "" {
					t.Fatalf("invalid consumed state: %+v hp=%d", loc, c.hp)
				}
			} else if competing != "DROP" || loc.roomID != w.Start || c.hp != 10 {
				t.Fatalf("invalid dropped state: %+v hp=%d", loc, c.hp)
			}
			s.dropInventoryLocked(c)
			if s.itemLocations["item.apple"] != loc {
				t.Fatal("disconnect recreated consumed/dropped item")
			}
		})
	}
}

func TestUseAmbiguityFullQueueAndOverflow(t *testing.T) {
	w := &World{Items: map[string]Item{"item.a": {ID: "item.a", Name: "Apple", Type: "consumable", HealValue: 10}, "item.b": {ID: "item.b", Name: "Apple", Type: "consumable", HealValue: 10}}}
	s := New(nil, w)
	c := &client{username: "alice", roomID: "loc.square", hp: 30, outbox: make(chan string, 1)}
	s.itemLocations["item.a"], s.itemLocations["item.b"] = itemLocation{owner: c}, itemLocation{owner: c}
	s.useItem(c, "Apple", true)
	if got := <-c.outbox; got != "ERR invalid_arguments USE name matches multiple items; use an item ID" {
		t.Fatal(got)
	}
	c.outbox <- "blocked"
	if s.useItem(c, "item.a", true) {
		t.Fatal("full queue accepted USE")
	}
	if c.hp != 30 || s.itemLocations["item.a"].owner != c {
		t.Fatal("failed reply mutated state")
	}
	<-c.outbox
	item := w.Items["item.a"]
	item.HealValue = int(^uint(0) >> 1)
	w.Items[item.ID] = item
	s.useItem(c, "item.a", true)
	if got := <-c.outbox; got != "OK used=item.a hp=100/100" || c.hp != 100 {
		t.Fatalf("overflow: %s hp=%d", got, c.hp)
	}
}

// Exercise mixed actions and terminal states rather than only isolated handlers.
func TestItemLifecycleMixedActions(t *testing.T) {
	w, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), w)
	clients := []*client{
		{username: "alice", roomID: "loc.garden", hp: 10, outbox: make(chan string, 32)},
		{username: "bob", roomID: "loc.market", hp: 25, outbox: make(chan string, 32)},
	}
	rooms := []string{"loc.garden", "loc.market", "loc.tavern", "loc.dark_alley", "loc.town_square"}
	ids := []string{"item.apple", "item.ale", "item.rare_herbs", "item.vigor_potion", "item.ancient_key", "item.rusty_sword"}
	// Fixed seed makes regressions reproducible.
	rng := rand.New(rand.NewSource(42))
	consumed := map[string]bool{}
	for step := 0; step < 2000; step++ {
		c := clients[rng.Intn(len(clients))]
		id := ids[rng.Intn(len(ids))]
		switch rng.Intn(7) {
		case 0:
			s.transferItem(c, "TAKE", id, true)
		case 1:
			s.transferItem(c, "DROP", id, true)
		case 2:
			s.useItem(c, id, true)
		case 3:
			s.talk(c, "npc.herbalist", true)
		case 4:
			s.dropInventoryLocked(c) // Disconnect never recreates terminal instances.
		case 5:
			c.roomID = rooms[rng.Intn(len(rooms))]
		case 6:
			c.hp = 1 + rng.Intn(99) // Future combat damage simulated inside the test only.
		}
		for _, client := range clients {
			for len(client.outbox) > 0 {
				<-client.outbox
			}
			if client.hp < 1 || client.hp > 100 {
				t.Fatalf("step %d invalid HP %d", step, client.hp)
			}
		}
		if len(s.itemLocations) != len(w.Items) {
			t.Fatalf("step %d changed instance count", step)
		}
		for id, loc := range s.itemLocations {
			if consumed[id] && !loc.consumed {
				t.Fatalf("step %d resurrected %s", step, id)
			}
			if loc.consumed {
				consumed[id] = true
			}
			states := 0
			if loc.owner != nil {
				states++
			}
			if loc.roomID != "" {
				states++
			}
			if loc.reserved {
				states++
			}
			if loc.consumed {
				states++
			}
			if states != 1 {
				t.Fatalf("step %d invalid item %s: %+v", step, id, loc)
			}
			if loc.roomID != "" {
				if _, exists := w.Locations[loc.roomID]; !exists {
					t.Fatalf("step %d unknown placement", step)
				}
			}
		}
		if winner := s.rewardClaims["quest.herbal_cure"]; winner != "" {
			if !s.itemLocations["item.rare_herbs"].consumed || s.itemLocations["item.vigor_potion"].reserved {
				t.Fatalf("step %d broke delivery invariant", step)
			}
		}
	}
}

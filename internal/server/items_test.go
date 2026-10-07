package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func expectRoomItems(t *testing.T, reader *bufio.Reader, ids []string) {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "OK ") {
		t.Fatalf("expected LOOK JSON, got %q", line)
	}
	var view struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "OK ")), &view); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		got = append(got, item.ID)
	}
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("room items=%v, want %v", got, ids)
	}
}

func TestUniqueItemsAcrossClientsAndDisconnect(t *testing.T) {
	world, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	addr, cancel, done := startTestServer(t, world)
	defer func() { cancel(); <-done }()
	alice, ar := dialTestClient(t, addr)
	bob, br := dialTestClient(t, addr)
	sendCommand(t, alice, "INVENTORY")
	expectLine(t, ar, "ERR not_authenticated Send CONNECT <username> before INVENTORY")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, ar, "OK connected")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, br, "OK connected")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "LOOK")
	expectRoomItems(t, ar, []string{"item.apple", "item.fountain"})
	sendCommand(t, alice, "INVENTORY")
	expectLine(t, ar, "OK []")
	sendCommand(t, alice, "TAKE")
	expectLine(t, ar, "ERR invalid_arguments Usage: TAKE <item_id or full display name>")
	sendCommand(t, alice, "TAKE pomme fraîche")
	expectLine(t, ar, "OK taken=item.apple")
	expectLine(t, ar, "EVT ROOM ITEM TAKE alice item.apple")
	expectLine(t, br, "EVT ROOM ITEM TAKE alice item.apple")
	sendCommand(t, alice, "INVENTORY")
	expectLine(t, ar, `OK [{"id":"item.apple","name":"Pomme fraîche"}]`)
	sendCommand(t, bob, "TAKE item.apple")
	expectLine(t, br, `ERR item_not_found TAKE target "item.apple" is not in loc.town_square; use LOOK`)
	sendCommand(t, alice, "TAKE item.fountain")
	expectLine(t, ar, "ERR item_not_obtainable Item item.fountain cannot be picked up with TAKE")
	sendCommand(t, bob, "DROP item.apple")
	expectLine(t, br, `ERR not_in_inventory DROP target "item.apple" is not in your inventory; use INVENTORY`)
	sendCommand(t, alice, "DROP POMME FRAÎCHE")
	expectLine(t, ar, "OK dropped=item.apple")
	expectLine(t, ar, "EVT ROOM ITEM DROP alice item.apple")
	expectLine(t, br, "EVT ROOM ITEM DROP alice item.apple")
	sendCommand(t, alice, "INVENTORY")
	expectLine(t, ar, "OK []")
	sendCommand(t, bob, "TAKE item.apple")
	expectLine(t, br, "OK taken=item.apple")
	expectLine(t, br, "EVT ROOM ITEM TAKE bob item.apple")
	expectLine(t, ar, "EVT ROOM ITEM TAKE bob item.apple")
	_ = bob.Close()
	expectLine(t, ar, "EVT ROOM ITEM DROP bob item.apple")
	expectLine(t, ar, "EVT ROOM PRESENCE LEAVE bob")
	sendCommand(t, alice, "LOOK")
	expectRoomItems(t, ar, []string{"item.apple", "item.fountain"})
	sendCommand(t, alice, "TAKE item.apple")
	expectLine(t, ar, "OK taken=item.apple")
	expectLine(t, ar, "EVT ROOM ITEM TAKE alice item.apple")
	sendCommand(t, alice, "MOVE north")
	expectLine(t, ar, "OK room=loc.garden")
	sendCommand(t, alice, "DROP item.apple")
	expectLine(t, ar, "OK dropped=item.apple")
	expectLine(t, ar, "EVT ROOM ITEM DROP alice item.apple")
	sendCommand(t, alice, "LOOK")
	expectRoomItems(t, ar, []string{"item.ancient_key", "item.apple", "item.rusty_sword"})
	sendCommand(t, alice, "DROP item.apple")
	expectLine(t, ar, `ERR not_in_inventory DROP target "item.apple" is not in your inventory; use INVENTORY`)
}

func TestConcurrentTakeHasExactlyOneOwner(t *testing.T) {
	world := &World{Start: "loc.square", Locations: map[string]Room{
		"loc.square": {Items: []string{"item.apple_1", "item.apple_2"}},
	}, Items: map[string]Item{
		"item.apple_1": {ID: "item.apple_1", Name: "Pomme", Obtainable: true},
		"item.apple_2": {ID: "item.apple_2", Name: "Pomme", Obtainable: true},
	}}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), world)
	alice := &client{username: "alice", roomID: "loc.square", outbox: make(chan string, 16)}
	bob := &client{username: "bob", roomID: "loc.square", outbox: make(chan string, 16)}
	s.players["alice"], s.players["bob"] = alice, bob
	s.transferItem(alice, "TAKE", "pomme", true)
	if got := <-alice.outbox; got != "ERR invalid_arguments TAKE name matches multiple items; use an item ID" {
		t.Fatalf("ambiguous name: %s", got)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, c := range []*client{alice, bob} {
		wg.Add(1)
		go func(c *client) { defer wg.Done(); <-start; s.transferItem(c, "TAKE", "item.apple_1", true) }(c)
	}
	close(start)
	wg.Wait()
	successes, failures := 0, 0
	for _, c := range []*client{alice, bob} {
		for len(c.outbox) > 0 {
			message := <-c.outbox
			if strings.HasPrefix(message, "OK taken=") {
				successes++
			}
			if strings.HasPrefix(message, "ERR item_not_found ") {
				failures++
			}
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("successes=%d failures=%d", successes, failures)
	}
	location := s.itemLocations["item.apple_1"]
	if location.owner == nil || location.roomID != "" {
		t.Fatalf("invalid owner/location: %+v", location)
	}
	if s.itemLocations["item.apple_2"].roomID != "loc.square" {
		t.Fatal("other instance moved")
	}
}

func TestWorldRejectsInvalidItemPlacements(t *testing.T) {
	for _, tc := range []struct{ name, catalogue, placement, want string }{
		{"unknown", `[]`, `["item.missing"]`, "unknown item"},
		{"duplicate_placement", `[{"id":"item.apple","name":"Apple","obtainable":true}]`, `["item.apple","item.apple"]`, "placed more than once"},
		{"duplicate_id", `[{"id":"item.apple","name":"Apple"},{"id":"item.apple","name":"Apple"}]`, `["item.apple"]`, "duplicate item ID"},
		{"unplaced", `[{"id":"item.apple","name":"Apple"}]`, `[]`, "no initial room"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/world.json"
			data := fmt.Sprintf(`{"world":{"start":"loc.square","items":%s,"locations":{"loc.square":{"name":"Square","description":"Here","items":%s}}}}`, tc.catalogue, tc.placement)
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadWorld(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

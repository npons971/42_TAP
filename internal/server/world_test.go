package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func readWorldReply(t *testing.T, r *bufio.Reader, target any) {
	t.Helper()
	line, err := readTestLine(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(line) > maxLineBytes || !strings.HasPrefix(line, "OK ") {
		t.Fatalf("invalid reply: %q", line)
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "OK ")), target); err != nil {
		t.Fatal(err)
	}
}

func TestFullWorldExploration(t *testing.T) {
	world, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(world.Locations) != 9 || len(world.Items) != 10 || len(world.NPCs) != 8 || len(world.Quests) != 2 {
		t.Fatalf("missing world content: %+v", world)
	}
	if world.Respawn != world.Start || world.Metadata.Name != "The Answer Protocol Realm" {
		t.Fatalf("lost metadata: %+v", world)
	}
	if world.Items["item.rusty_sword"].DamageBonus != 10 || world.Items["item.ale"].HealValue != 15 || world.NPCs["npc.bandit_leader"].HP != 70 {
		t.Fatal("lost future gameplay attributes")
	}
	var quest struct {
		Reward string `json:"reward_item"`
	}
	if err := json.Unmarshal(world.Quests["quest.herbal_cure"], &quest); err != nil || quest.Reward != "item.vigor_potion" {
		t.Fatalf("lost quest definition: %v %+v", err, quest)
	}
	addr, cancel, done := startTestServer(t, world)
	defer func() { cancel(); <-done }()
	conn, r := dialTestClient(t, addr)
	sendCommand(t, conn, "CONNECT explorer")
	expectLine(t, r, "OK connected")
	for _, id := range []string{"item.ancient_key", "item.vigor_potion"} {
		sendCommand(t, conn, "TAKE "+id)
		expectLine(t, r, fmt.Sprintf("ERR 404 ITEM_NOT_FOUND TAKE target %q is not in loc.town_square; use LOOK", id))
		sendCommand(t, conn, "DROP "+id)
		expectLine(t, r, fmt.Sprintf("ERR 404 ITEM_NOT_IN_INVENTORY DROP target %q is not in your inventory; use INVENTORY", id))
	}
	// Includes both loops, underground passages and the optional ruins branch.
	route := []string{"", "east", "north", "down", "south", "up", "south", "north", "south", "east", "east", "north", "south", "west", "west"}
	roomID := world.Start
	visited := map[string]bool{}
	for _, direction := range route {
		if direction != "" {
			roomID = world.Locations[roomID].Exits[direction]
			if roomID == "" {
				t.Fatalf("invalid test route direction %s", direction)
			}
			sendCommand(t, conn, "MOVE "+direction)
			expectLine(t, r, "OK room="+roomID)
		}
		visited[roomID] = true
		sendCommand(t, conn, "LOOK DETAILS")
		var view struct {
			Room  roomView         `json:"room"`
			Items []map[string]any `json:"items"`
			NPCs  []npcView        `json:"npcs"`
		}
		readWorldReply(t, r, &view)
		room := world.Locations[roomID]
		if view.Room.ID != roomID || view.Room.Description != room.Description || !reflect.DeepEqual(view.Room.Exits, room.Exits) {
			t.Fatalf("wrong room view: %+v", view)
		}
		if len(view.Items) != len(room.Items) || len(view.NPCs) != len(room.NPCs) {
			t.Fatalf("wrong placements in %s: %+v", roomID, view)
		}
		for _, item := range view.Items {
			if len(item) != 3 {
				t.Fatalf("LOOK exposed catalogue-only fields: %+v", item)
			}
			id, _ := item["id"].(string)
			if world.Items[id].InitialLocation != nil {
				t.Fatal("reserve exposed in LOOK")
			}
		}
		for _, npc := range view.NPCs {
			// Both exact IDs and case-insensitive complete English names work.
			for _, target := range []string{npc.ID, strings.ToUpper(npc.Name)} {
				sendCommand(t, conn, "TALKJSON "+target)
				var reply struct {
					NPC      string `json:"npc"`
					Dialogue string `json:"dialogue"`
				}
				readWorldReply(t, r, &reply)
				if reply.NPC != npc.ID || reply.Dialogue != world.NPCs[npc.ID].Dialogue {
					t.Fatalf("dialogue mismatch: %+v", reply)
				}
			}
		}
	}
	if len(visited) != 9 {
		t.Fatalf("only visited %v", visited)
	}
	sendCommand(t, conn, "MOVE east")
	expectLine(t, r, "OK room=loc.market")
	sendCommand(t, conn, "TAKE fresh apple")
	expectLine(t, r, "OK taken=item.apple")
	expectLine(t, r, "EVT ROOM ITEM TAKE explorer item.apple")
	sendCommand(t, conn, "INVENTORY DETAILS")
	expectLine(t, r, `OK [{"id":"item.apple","name":"Fresh Apple"}]`)
	sendCommand(t, conn, "DROP item.apple")
	expectLine(t, r, "OK dropped=item.apple")
	expectLine(t, r, "EVT ROOM ITEM DROP explorer item.apple")
	sendCommand(t, conn, "QUIT")
	expectLine(t, r, "OK bye")
	if _, err := readTestLine(r); err != io.EOF {
		t.Fatalf("QUIT must close the connection, got %v", err)
	}
}

func TestWorldReserveRespawnAndDialogues(t *testing.T) {
	for _, tc := range []struct{ name, extra, item, placement, dialogue, want string }{
		{"reserve", "", `"initial_location":{"kind":"reserve"}`, `[]`, `["First","Second"]`, ""},
		{"legacy_dialogue", "", `"initial_location":{"kind":"reserve"}`, `[]`, `"Hello"`, ""},
		{"reserve_and_room", "", `"initial_location":{"kind":"reserve"}`, `["item.reward"]`, `"Hello"`, "reserved and placed"},
		{"unsupported_reserve_kind", "", `"initial_location":{"kind":"room"}`, `[]`, `"Hello"`, "kind must be reserve"},
		{"unplaced_item", "", `"type":"quest_reward"`, `[]`, `"Hello"`, "no initial room"},
		{"unknown_respawn", `,"respawn":"loc.missing"`, `"initial_location":{"kind":"reserve"}`, `[]`, `"Hello"`, "respawn room"},
		{"empty_lines", "", `"initial_location":{"kind":"reserve"}`, `[]`, `[]`, "needs dialogue"},
		{"blank_line", "", `"initial_location":{"kind":"reserve"}`, `[]`, `["Hello"," "]`, "needs dialogue"},
		{"wrong_dialogue_type", "", `"initial_location":{"kind":"reserve"}`, `[]`, `[1]`, "dialogue must be"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/world.json"
			data := fmt.Sprintf(`{"world":{"start":"loc.square"%s,"items":[{"id":"item.reward","name":"Reward","obtainable":true,%s}],"npcs":[{"id":"npc.guard","name":"Guard","role":"dialogue","dialogue":%s}],"locations":{"loc.square":{"name":"Square","description":"Here","items":%s,"npcs":["npc.guard"]}}}}`, tc.extra, tc.item, tc.dialogue, tc.placement)
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			world, err := LoadWorld(path)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("want %q, got %v", tc.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if world.Respawn != world.Start {
				t.Fatal("missing respawn fallback")
			}
			if tc.name == "reserve" && world.NPCs["npc.guard"].Dialogue != "First\nSecond" {
				t.Fatal("lost dialogue order")
			}
			s := New(nil, world)
			if !s.itemLocations["item.reward"].reserved || len(s.roomItemsLocked("loc.square")) != 0 {
				t.Fatal("reserve not represented off-map")
			}
		})
	}
}

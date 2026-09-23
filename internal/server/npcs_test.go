package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTalkAndLookAcrossRooms(t *testing.T) {
	world, err := LoadWorld("testdata/two_rooms.json")
	if err != nil {
		t.Fatal(err)
	}
	addr, cancel, done := startTestServer(t, world)
	defer func() { cancel(); <-done }()
	alice, ar := dialTestClient(t, addr)
	bob, br := dialTestClient(t, addr)
	sendCommand(t, alice, "TALKJSON npc.guard")
	expectLine(t, ar, "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before TALK")
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, ar, "OK connected")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, br, "OK connected")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "LOOK DETAILS")
	line, err := readTestLine(ar)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		NPCs []npcView `json:"npcs"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "OK ")), &view); err != nil {
		t.Fatal(err)
	}
	if want := []npcView{{ID: "npc.guard", Name: "Garde du Village", Role: "dialogue"}}; !reflect.DeepEqual(view.NPCs, want) {
		t.Fatalf("LOOK NPCs=%v, want %v", view.NPCs, want)
	}
	if strings.Contains(line, "dialogue\":") {
		t.Fatal("LOOK exposed dialogue instead of descriptors only")
	}
	for _, target := range []string{"npc.guard", "GARDE DU VILLAGE"} {
		sendCommand(t, alice, "TALKJSON "+target)
		expectLine(t, ar, `OK {"npc":"npc.guard","dialogue":"Restez sur vos gardes, voyageur."}`)
	}
	// A subsequent reply must be first in Bob's stream: TALK has no broadcast.
	sendCommand(t, bob, "WHO DETAILS")
	expectLine(t, br, `OK {"room":["alice","bob"],"server":2}`)
	for _, tc := range []struct{ command, want string }{
		{"TALK", "ERR 400 INVALID_ARGUMENTS Usage: TALK <npc_id or full display name>"},
		{"TALK   ", "ERR 400 INVALID_ARGUMENTS Usage: TALK <npc_id or full display name>"},
		{"TALK npc.guard ", "ERR 400 INVALID_ARGUMENTS TALK target must not contain tabs or surrounding spaces"},
		{"TALK Garde\tdu Village", "ERR 400 INVALID_ARGUMENTS TALK target must not contain tabs or surrounding spaces"},
		{"TALK npc.missing", `ERR 404 NPC_NOT_FOUND TALK target "npc.missing" is not in loc.town_square; use LOOK`},
		{"TALK npc.herbalist", `ERR 404 NPC_NOT_FOUND TALK target "npc.herbalist" is not in loc.town_square; use LOOK`},
		{"TALK Garde", `ERR 404 NPC_NOT_FOUND TALK target "Garde" is not in loc.town_square; use LOOK`},
		{"TALK NPC.GUARD", `ERR 404 NPC_NOT_FOUND TALK target "NPC.GUARD" is not in loc.town_square; use LOOK`},
	} {
		sendCommand(t, alice, tc.command)
		expectLine(t, ar, tc.want)
	}
	sendCommand(t, alice, "MOVE north")
	expectLine(t, ar, "OK room=loc.garden")
	expectLine(t, br, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, alice, "TALKJSON npc.guard")
	expectLine(t, ar, `ERR 404 NPC_NOT_FOUND TALK target "npc.guard" is not in loc.garden; use LOOK`)
	sendCommand(t, alice, "TALKJSON rat géant")
	expectLine(t, ar, `OK {"npc":"npc.giant_rat","dialogue":"Le rat grince des dents en vous regardant."}`)
	sendCommand(t, alice, "TALKJSON Herboriste")
	expectLine(t, ar, `OK {"npc":"npc.herbalist","dialogue":"Bonjour aventurier. Auriez-vous un instant pour m’aider ?"}`)
}

func TestTalkAmbiguityAndLineLimit(t *testing.T) {
	world := &World{Start: "loc.square", Locations: map[string]Room{
		"loc.square": {NPCs: []string{"npc.guard_2", "npc.guard_1"}},
		"loc.empty":  {},
	}, NPCs: map[string]NPC{
		"npc.guard_1": {ID: "npc.guard_1", Name: "Garde", Role: "dialogue", Dialogue: "Bonjour"},
		"npc.guard_2": {ID: "npc.guard_2", Name: "Garde", Role: "dialogue", Dialogue: "Salut"},
	}}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), world)
	c := &client{username: "alice", roomID: "loc.square", outbox: make(chan string, 16)}
	s.talkReply(c, "garde", true, true)
	if got := <-c.outbox; got != "ERR 400 INVALID_ARGUMENTS TALK name matches multiple NPCs; use an NPC ID" {
		t.Fatalf("ambiguity: %s", got)
	}
	s.talkReply(c, "npc.guard_1", true, true)
	if got := <-c.outbox; got != `OK {"npc":"npc.guard_1","dialogue":"Bonjour"}` {
		t.Fatalf("ID resolution: %s", got)
	}
	views := s.roomNPCsLocked("loc.square")
	if len(views) != 2 || views[0].ID != "npc.guard_1" || views[1].ID != "npc.guard_2" {
		t.Fatalf("NPC ordering: %v", views)
	}
	if empty := s.roomNPCsLocked("loc.empty"); empty == nil || len(empty) != 0 {
		t.Fatalf("empty room NPCs must encode as []: %v", empty)
	}
	// Include UTF-8, quotes and newlines to measure the encoded wire payload.
	npc := world.NPCs["npc.guard_1"]
	npc.Dialogue = "é\"\n"
	world.NPCs[npc.ID] = npc
	s.talkReply(c, npc.ID, true, true)
	prefix := <-c.outbox
	padding := maxLineBytes - len(prefix) - 1
	npc.Dialogue += strings.Repeat("x", padding)
	world.NPCs[npc.ID] = npc
	s.talkReply(c, npc.ID, true, true)
	if got := <-c.outbox; !strings.HasPrefix(got, "OK ") || len(got)+1 != maxLineBytes {
		t.Fatalf("exact limit rejected: %d bytes, %s", len(got)+1, shortVerb(got))
	}
	npc.Dialogue += "x"
	world.NPCs[npc.ID] = npc
	s.talkReply(c, npc.ID, true, true)
	if got := <-c.outbox; got != "ERR 413 RESPONSE_TOO_LARGE TALK response exceeds 4096-byte line limit" {
		t.Fatalf("oversized dialogue: %s", got)
	}
}

func TestWorldRejectsInvalidNPCs(t *testing.T) {
	for _, tc := range []struct{ name, catalogue, placement, want string }{
		{"unknown", `[]`, `["npc.missing"]`, "unknown NPC"},
		{"duplicate_placement", `[{"id":"npc.guard","name":"Guard","role":"dialogue","dialogue":"Hi"}]`, `["npc.guard","npc.guard"]`, "placed more than once"},
		{"duplicate_id", `[{"id":"npc.guard","name":"Guard","role":"dialogue","dialogue":"Hi"},{"id":"npc.guard","name":"Guard","role":"dialogue","dialogue":"Hi"}]`, `["npc.guard"]`, "duplicate NPC ID"},
		{"unplaced", `[{"id":"npc.guard","name":"Guard","role":"dialogue","dialogue":"Hi"}]`, `[]`, "no initial room"},
		{"bad_id", `[{"id":"npc.Guard","name":"Guard","role":"dialogue","dialogue":"Hi"}]`, `[]`, "invalid NPC ID"},
		{"bad_name", `[{"id":"npc.guard","name":"Guard\n","role":"dialogue","dialogue":"Hi"}]`, `[]`, "display name"},
		{"bad_role", `[{"id":"npc.guard","name":"Guard","role":"merchant","dialogue":"Hi"}]`, `[]`, "role must be"},
		{"empty_dialogue", `[{"id":"npc.guard","name":"Guard","role":"dialogue","dialogue":"  "}]`, `[]`, "needs dialogue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/world.json"
			data := `{"world":{"start":"loc.square","npcs":` + tc.catalogue + `,"locations":{"loc.square":{"name":"Square","description":"Here","npcs":` + tc.placement + `}}}}`
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadWorld(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

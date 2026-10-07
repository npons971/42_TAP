package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// These assertions read every frame, including STATS, without legacy helpers.
func rfcLine(t *testing.T, r *bufio.Reader, want string) {
	t.Helper()
	got, err := r.ReadString('\n')
	if err != nil || got != want+"\n" {
		t.Fatalf("got %q (%v), want %q", got, err, want)
	}
}
func rfcJSON(t *testing.T, r *bufio.Reader, v any) {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "OK ") {
		t.Fatalf("JSON reply: %q %v", line, err)
	}
	if err := json.Unmarshal([]byte(line[3:]), v); err != nil {
		t.Fatal(err)
	}
}
func rfcError(t *testing.T, r *bufio.Reader, prefix string) {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, prefix) {
		t.Fatalf("got %q %v, expected %s", line, err, prefix)
	}
}
func TestRFCBasicFramesAndUnicodeTCP(t *testing.T) {
	_, addr, stop := lifecycleServer(t)
	defer stop()
	a, ar := dialTestClient(t, addr)
	b, br := dialTestClient(t, addr)
	// Fragment inside a multi-byte codepoint, and coalesce the following command.
	wire := []byte("cOnNeCt élise\nwho\n")
	if _, err := a.Write(wire[:9]); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Write(wire[9:]); err != nil {
		t.Fatal(err)
	}
	rfcLine(t, ar, "OK connected")
	rfcLine(t, ar, "EVT STATS players=1")
	rfcLine(t, ar, "OK players=1")
	sendCommand(t, b, "CONNECT élise")
	rfcError(t, br, "ERR 201 NAME_IN_USE ")
	sendCommand(t, b, "CONNECT bob")
	rfcLine(t, br, "OK connected")
	rfcLine(t, br, "EVT STATS players=2")
	rfcLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	rfcLine(t, ar, "EVT STATS players=2")
	sendCommand(t, a, "look")
	var room struct {
		Room    roomView `json:"room"`
		Players []string `json:"players"`
		Items   []string `json:"items"`
		NPCs    []string `json:"npcs"`
	}
	rfcJSON(t, ar, &room)
	if room.Room.ID != "loc.town_square" || len(room.Items) != 1 || room.Items[0] != "item.fountain" || len(room.NPCs) != 1 || room.NPCs[0] != "npc.guard" {
		t.Fatal(room)
	}
	sendCommand(t, a, "inventory")
	rfcLine(t, ar, "OK []")
	sendCommand(t, a, "TaLk Village Guard")
	rfcLine(t, ar, "OK Stay on your guard, traveler. The outer ruins have grown lawless. The herbalist in the northern garden might need help gathering supplies. Don't cause any trouble within the village gates.")
	sendCommand(t, a, "move down")
	rfcError(t, ar, "ERR 301 NO_EXIT ")
	sendCommand(t, a, "TAKE item.missing")
	rfcError(t, ar, "ERR 404 ITEM_NOT_FOUND ")
	sendCommand(t, a, "DROP item.missing")
	rfcError(t, ar, "ERR 404 ITEM_NOT_IN_INVENTORY ")
	sendCommand(t, a, "TALK npc.missing")
	rfcError(t, ar, "ERR 404 NPC_NOT_FOUND ")
	sendCommand(t, a, "ATTACK npc.guard")
	rfcError(t, ar, "ERR 405 NPC_NOT_HOSTILE ")
	sendCommand(t, a, "QUEST npc.guard")
	rfcError(t, ar, "ERR 406 NO_QUEST_AVAILABLE ")
	sendCommand(t, a, "QUEST npc.missing")
	rfcError(t, ar, "ERR 404 NPC_NOT_FOUND ")
	sendCommand(t, a, "STATUS")
	var status struct {
		HP     int `json:"hp"`
		MaxHP  int `json:"max_hp"`
		Status string
	}
	rfcJSON(t, ar, &status)
	if status.HP != 100 || status.MaxHP != 100 || status.Status != "healthy" {
		t.Fatal(status)
	}
	// Unicode chat works; control characters are rejected and the next frame survives.
	sendCommand(t, a, "chat global bonjour 世界")
	rfcLine(t, ar, "OK")
	rfcLine(t, ar, "EVT GLOBAL CHAT élise bonjour 世界")
	rfcLine(t, br, "EVT GLOBAL CHAT élise bonjour 世界")
	sendCommand(t, a, "CHAT GLOBAL \x1b[2J")
	rfcError(t, ar, "ERR 400 INVALID_ARGUMENTS ")
	sendCommand(t, a, "quit")
	rfcLine(t, ar, "OK bye")
	if _, err := ar.ReadByte(); err != io.EOF {
		t.Fatal(err)
	}
	rfcLine(t, br, "EVT ROOM PRESENCE LEAVE élise")
	rfcLine(t, br, "EVT STATS players=1")
	sendCommand(t, b, "QUIT")
	rfcLine(t, br, "OK bye")
	if _, err := br.ReadByte(); err != io.EOF {
		t.Fatal(err)
	}
}
func TestRFCQuestAcceptanceAndInventoryTCP(t *testing.T) {
	_, addr, stop := lifecycleServer(t)
	defer stop()
	c, r := dialTestClient(t, addr)
	sendCommand(t, c, "CONNECT alice")
	rfcLine(t, r, "OK connected")
	rfcLine(t, r, "EVT STATS players=1")
	sendCommand(t, c, "QUESTS")
	rfcLine(t, r, "OK []")
	sendCommand(t, c, "MOVE north")
	rfcLine(t, r, "OK room=loc.garden")
	sendCommand(t, c, "quest Village Herbalist")
	var q struct {
		ID                          string `json:"quest_id"`
		Description, Reward, Status string
	}
	rfcJSON(t, r, &q)
	if q.ID != "quest.herbal_cure" || q.Status != "available" || q.Reward != "item.vigor_potion" {
		t.Fatal(q)
	}
	sendCommand(t, c, "QUEST npc.herbalist")
	rfcJSON(t, r, &q)
	if q.Status != "active" {
		t.Fatal(q)
	}
	sendCommand(t, c, "QUESTS")
	rfcLine(t, r, `OK [{"quest_id":"quest.herbal_cure","status":"active","progress":"0/1"}]`)
	sendCommand(t, c, "TAKE Rare Herbs")
	rfcLine(t, r, "OK taken=item.rare_herbs")
	rfcLine(t, r, "EVT ROOM ITEM TAKE alice item.rare_herbs")
	sendCommand(t, c, "INVENTORY")
	rfcLine(t, r, `OK ["item.rare_herbs"]`)
	sendCommand(t, c, "QUESTS")
	rfcLine(t, r, `OK [{"quest_id":"quest.herbal_cure","status":"active","progress":"1/1"}]`)
	sendCommand(t, c, "TALK npc.herbalist")
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "OK ") || strings.HasPrefix(line, "OK {") {
		t.Fatal(line, err)
	}
	rfcLine(t, r, "EVT ROOM ITEM DELIVER alice item.rare_herbs")
	rfcLine(t, r, "EVT ROOM ITEM REWARD alice item.vigor_potion")
	sendCommand(t, c, "QUESTS")
	rfcLine(t, r, `OK [{"quest_id":"quest.herbal_cure","status":"completed"}]`)
	sendCommand(t, c, "QUEST npc.herbalist")
	rfcError(t, r, "ERR 406 NO_QUEST_AVAILABLE ")
	sendCommand(t, c, "INVENTORY")
	rfcLine(t, r, `OK ["item.vigor_potion"]`)
}
func TestRFCQuestFailureDoesNotAccept(t *testing.T) {
	s, c, def := rewardTestState(t)
	s.itemLocations[def.TargetItem] = itemLocation{roomID: c.roomID}
	def.Description = strings.Repeat("x", maxLineBytes)
	s.world.Rewards[def.ID] = def
	s.requestQuest(c, def.Giver, true)
	if got := <-c.outbox; !strings.HasPrefix(got, "ERR 413 RESPONSE_TOO_LARGE ") {
		t.Fatal(got)
	}
	if len(s.questProgress) != 0 {
		t.Fatal("oversized quest was accepted")
	}
	def.Description = "short"
	s.world.Rewards[def.ID] = def
	c.outbox = make(chan string, 1)
	c.outbox <- "blocked"
	if s.requestQuest(c, def.Giver, true) || len(s.questProgress) != 0 {
		t.Fatal("full queue accepted a quest")
	}
}
func TestRFCGroupEventsTCP(t *testing.T) {
	_, addr, stop := lifecycleServer(t)
	defer stop()
	a, ar := dialTestClient(t, addr)
	b, br := dialTestClient(t, addr)
	sendCommand(t, a, "CONNECT alice")
	rfcLine(t, ar, "OK connected")
	rfcLine(t, ar, "EVT STATS players=1")
	sendCommand(t, b, "CONNECT bob")
	rfcLine(t, br, "OK connected")
	rfcLine(t, br, "EVT STATS players=2")
	rfcLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	rfcLine(t, ar, "EVT STATS players=2")
	sendCommand(t, b, "GROUP INVITE alice")
	rfcError(t, br, "ERR 401 NOT_IN_GROUP ")
	sendCommand(t, a, "group create")
	rfcLine(t, ar, "OK group=group.1")
	sendCommand(t, a, "GROUP CREATE")
	rfcError(t, ar, "ERR 402 ALREADY_IN_GROUP ")
	sendCommand(t, a, "GROUP INVITE bob")
	rfcLine(t, ar, "OK")
	rfcLine(t, br, "EVT GROUP INVITE alice")
	sendCommand(t, b, "GROUP JOIN alice")
	rfcLine(t, br, "OK group=group.1")
	rfcLine(t, br, "EVT GROUP JOIN bob")
	rfcLine(t, ar, "EVT GROUP JOIN bob")
	sendCommand(t, b, "CHAT GROUP salut")
	rfcLine(t, br, "OK")
	rfcLine(t, br, "EVT GROUP CHAT bob salut")
	rfcLine(t, ar, "EVT GROUP CHAT bob salut")
	sendCommand(t, b, "GROUP LEAVE")
	rfcLine(t, br, "OK")
	rfcLine(t, br, "EVT GROUP LEAVE bob")
	rfcLine(t, ar, "EVT GROUP LEAVE bob")
}
func TestRFCCombatCoreFields(t *testing.T) {
	s, c := combatClient(t)
	s.attack(c, "npc.bandit_leader", true)
	r := turnReply(t, c)
	if r.AttackerHP != 80 || r.TargetHP != 58 || r.Damage != 12 || r.Status != "combat" {
		t.Fatalf("RFC combat fields: %+v", r)
	}
}

// Abrupt disconnect must produce a count update and free the name.
func TestRFCDisconnectStatisticsTCP(t *testing.T) {
	_, addr, stop := lifecycleServer(t)
	defer stop()
	a, ar := dialTestClient(t, addr)
	sendCommand(t, a, "CONNECT alice")
	rfcLine(t, ar, "OK connected")
	rfcLine(t, ar, "EVT STATS players=1")
	b, br := dialTestClient(t, addr)
	sendCommand(t, b, "CONNECT bob")
	rfcLine(t, br, "OK connected")
	rfcLine(t, br, "EVT STATS players=2")
	rfcLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	rfcLine(t, ar, "EVT STATS players=2")
	b.Close()
	rfcLine(t, ar, "EVT ROOM PRESENCE LEAVE bob")
	rfcLine(t, ar, "EVT STATS players=1")
	b, br = dialTestClient(t, addr)
	sendCommand(t, b, "CONNECT bob")
	rfcLine(t, br, "OK connected")
	rfcLine(t, br, "EVT STATS players=2")
}

func TestRFCConnectionLimitTCP(t *testing.T) {
	addr, cancel, done := startTestServer(t)
	defer func() { cancel(); <-done }()
	var first *bufio.Reader
	var firstConn net.Conn
	for i := 0; i < maxConnections; i++ {
		c, r := dialTestClient(t, addr)
		if i == 0 {
			firstConn, first = c, r
		}
	}
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	r := bufio.NewReader(c)
	rfcLine(t, r, "OK hello proto=1")
	rfcError(t, r, "ERR 503 CONNECTION_LIMIT ")
	sendCommand(t, firstConn, "WHO")
	rfcError(t, first, "ERR 403 NOT_AUTHENTICATED ")
}
func TestInventoryAndRewardCapacity(t *testing.T) {
	for _, fetch := range []bool{false, true} {
		t.Run(fmt.Sprint("fetch=", fetch), func(t *testing.T) {
			s, c, def := rewardTestState(t)
			if !fetch {
				def = s.world.Rewards["quest.bandit_bounty"]
				c.roomID = def.Giver
				for room, loc := range s.world.Locations {
					for _, id := range loc.NPCs {
						if id == def.Giver {
							c.roomID = room
						}
					}
				}
				s.progressLocked(c.username).Defeated[def.DefeatNPC] = true
			}
			if fetch {
				s.itemLocations[def.TargetItem] = itemLocation{owner: c}
			}
			count := maxInventoryItems - s.inventoryCountLocked(c)
			for i := 0; i < count; i++ {
				id := fmt.Sprintf("item.capacity_%d", i)
				s.itemLocations[id] = itemLocation{owner: c}
			}
			s.itemLocations["item.apple"] = itemLocation{roomID: c.roomID}
			s.transferItem(c, "TAKE", "item.apple", true)
			if got := <-c.outbox; !strings.HasPrefix(got, "ERR 409 INVENTORY_FULL ") {
				t.Fatal(got)
			}
			s.talk(c, def.Giver, true)
			got := <-c.outbox
			if fetch {
				if !strings.HasPrefix(got, "OK ") || s.rewardClaims[def.ID] != c.username || !s.itemLocations[def.TargetItem].consumed || s.inventoryCountLocked(c) != maxInventoryItems {
					t.Fatal(got, "fetch replacement failed")
				}
			} else {
				if !strings.HasPrefix(got, "ERR 409 INVENTORY_FULL ") || s.rewardClaims[def.ID] != "" || !s.itemLocations[def.RewardItem].reserved {
					t.Fatal(got, "full inventory claimed reward")
				}
				delete(s.itemLocations, "item.capacity_0")
				s.talk(c, def.Giver, true)
				if got := <-c.outbox; !strings.HasPrefix(got, "OK ") || s.rewardClaims[def.ID] != c.username {
					t.Fatal(got, "retry did not complete")
				}
			}
		})
	}
}

func TestQuestListHidesUnstartedUnavailableDefinitions(t *testing.T) {
	s, c, def := rewardTestState(t)
	other := &client{username: "bob", roomID: c.roomID, outbox: make(chan string, 16)}
	s.requestQuest(other, def.Giver, true)
	<-other.outbox
	s.talk(c, def.Giver, true)
	drain(c)
	s.quests(other, []string{"QUESTS"})
	if got := <-other.outbox; got != `OK [{"quest_id":"quest.herbal_cure","status":"unavailable","progress":"0/1"}]` {
		t.Fatal(got)
	}
	newcomer := &client{username: "newcomer", roomID: c.roomID, outbox: make(chan string, 16)}
	s.quests(newcomer, []string{"QUESTS"})
	if got := <-newcomer.outbox; got != "OK []" {
		t.Fatal("unstarted definitions leaked:", got)
	}
	s.requestQuest(newcomer, def.Giver, true)
	if got := <-newcomer.outbox; !strings.HasPrefix(got, "ERR 406 NO_QUEST_AVAILABLE ") {
		t.Fatal(got)
	}
}

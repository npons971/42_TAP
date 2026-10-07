package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestDeliveryRewardsTCPAndReconnect(t *testing.T) {
	s, addr, stop := lifecycleServer(t)
	defer stop()
	alice, ar := dialTestClient(t, addr)
	bob, br := dialTestClient(t, addr)
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, ar, "OK connected")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, br, "OK connected")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "MOVE north")
	expectLine(t, ar, "OK room=loc.garden")
	expectLine(t, br, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, bob, "MOVE north")
	expectLine(t, br, "OK room=loc.garden")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "TAKE Rare Herbs")
	expectLine(t, ar, "OK taken=item.rare_herbs")
	expectLine(t, ar, "EVT ROOM ITEM TAKE alice item.rare_herbs")
	expectLine(t, br, "EVT ROOM ITEM TAKE alice item.rare_herbs")
	sendCommand(t, bob, "TALK npc.herbalist")
	var reply struct {
		NPC      string `json:"npc"`
		Dialogue string `json:"dialogue"`
	}
	readWorldReply(t, br, &reply)
	if reply.Dialogue != s.world.NPCs["npc.herbalist"].Dialogue {
		t.Fatal("non-owner delivered another player's herbs")
	}
	sendCommand(t, alice, "MOVE south")
	expectLine(t, ar, "OK room=loc.town_square")
	expectLine(t, br, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, alice, "TALK npc.herbalist")
	expectLine(t, ar, `ERR target_not_found TALK target "npc.herbalist" is not in loc.town_square; use LOOK`)
	sendCommand(t, alice, "MOVE north")
	expectLine(t, ar, "OK room=loc.garden")
	expectLine(t, br, "EVT ROOM PRESENCE ENTER alice")
	s.mu.Lock()
	s.players["alice"].hp = 15
	s.mu.Unlock()
	sendCommand(t, alice, "TALK VILLAGE HERBALIST")
	readWorldReply(t, ar, &reply)
	if reply.Dialogue != s.world.Rewards["quest.herbal_cure"].Dialogue {
		t.Fatalf("wrong completion reply: %+v", reply)
	}
	for _, r := range []struct{ verb, id string }{{"DELIVER", "item.rare_herbs"}, {"REWARD", "item.vigor_potion"}} {
		want := fmt.Sprintf("EVT ROOM ITEM %s alice %s", r.verb, r.id)
		expectLine(t, ar, want)
		expectLine(t, br, want)
	}
	sendCommand(t, alice, "STATUS")
	expectLine(t, ar, `OK {"player":"alice","hp":100,"max_hp":100,"state":"HORS_COMBAT","combat":null}`)
	sendCommand(t, alice, "INVENTORY")
	expectLine(t, ar, `OK [{"id":"item.vigor_potion","name":"Vigor Potion"}]`)
	sendCommand(t, alice, "TALK npc.herbalist")
	readWorldReply(t, ar, &reply)
	if !strings.Contains(reply.Dialogue, "already complete") {
		t.Fatal("delivery replay not identified")
	}
	sendCommand(t, bob, "TALK npc.herbalist")
	expectLine(t, br, "ERR reward_unavailable TALK reward for quest.herbal_cure has already been awarded in this world")
	sendCommand(t, bob, "TAKE item.rare_herbs")
	expectLine(t, br, `ERR item_not_found TAKE target "item.rare_herbs" is not in loc.garden; use LOOK`)
	sendCommand(t, alice, "USE Vigor Potion")
	expectLine(t, ar, "ERR health_full USE cannot restore HP: health is already full; item kept")
	s.mu.Lock()
	s.players["alice"].hp = 1
	s.mu.Unlock()
	sendCommand(t, alice, "USE item.vigor_potion")
	expectLine(t, ar, "OK used=item.vigor_potion hp=100/100")
	expectLine(t, ar, "EVT ROOM ITEM USE alice item.vigor_potion")
	expectLine(t, br, "EVT ROOM ITEM USE alice item.vigor_potion")
	_ = alice.Close()
	expectLine(t, br, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, bob, "LOOK")
	expectRoomItems(t, br, []string{})
	again, rr := dialTestClient(t, addr)
	sendCommand(t, again, "CONNECT alice")
	expectLine(t, rr, "OK connected")
	sendCommand(t, again, "MOVE north")
	expectLine(t, rr, "OK room=loc.garden")
	expectLine(t, br, "EVT ROOM PRESENCE ENTER alice")
	sendCommand(t, again, "TALK npc.herbalist")
	readWorldReply(t, rr, &reply)
	if !strings.Contains(reply.Dialogue, "already complete") {
		t.Fatal("reconnect bypassed world claim")
	}
	sendCommand(t, again, "INVENTORY")
	expectLine(t, rr, "OK []")
	fresh := New(nil, s.world)
	if !fresh.itemLocations["item.vigor_potion"].reserved || fresh.itemLocations["item.rare_herbs"].roomID != "loc.garden" || len(fresh.rewardClaims) != 0 {
		t.Fatal("restart did not restore initial unique world")
	}
}

func rewardTestState(t *testing.T) (*Server, *client, RewardDefinition) {
	t.Helper()
	w, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), w)
	c := &client{username: "alice", roomID: "loc.garden", hp: 12, outbox: make(chan string, 16)}
	s.itemLocations["item.rare_herbs"] = itemLocation{owner: c}
	return s, c, w.Rewards["quest.herbal_cure"]
}

func TestDeliveryFailuresAreAtomic(t *testing.T) {
	for _, kind := range []string{"queue_full", "response_too_large", "reward_missing", "reward_owned", "reward_consumed", "reward_on_ground"} {
		t.Run(kind, func(t *testing.T) {
			s, c, def := rewardTestState(t)
			switch kind {
			case "queue_full":
				c.outbox = make(chan string, 1)
				c.outbox <- "blocked"
			case "response_too_large":
				def.Dialogue = strings.Repeat("x", maxLineBytes)
				s.world.Rewards[def.ID] = def
			case "reward_missing":
				delete(s.itemLocations, def.RewardItem)
			case "reward_owned":
				s.itemLocations[def.RewardItem] = itemLocation{owner: &client{username: "bob"}}
			case "reward_consumed":
				s.itemLocations[def.RewardItem] = itemLocation{consumed: true}
			case "reward_on_ground":
				s.itemLocations[def.RewardItem] = itemLocation{roomID: c.roomID}
			}
			before, exists := s.itemLocations[def.RewardItem]
			alive := s.talk(c, "npc.herbalist", true)
			if c.hp != 12 || s.itemLocations[def.TargetItem].owner != c || len(s.rewardClaims) != 0 {
				t.Fatal("failed delivery mutated player or herbs")
			}
			after, existsAfter := s.itemLocations[def.RewardItem]
			if before != after || exists != existsAfter {
				t.Fatal("failed delivery mutated reward")
			}
			got := <-c.outbox
			if kind == "queue_full" {
				if alive || got != "blocked" {
					t.Fatal("full queue accepted delivery")
				}
			} else if kind == "response_too_large" {
				if !strings.HasPrefix(got, "ERR response_too_large ") {
					t.Fatal(got)
				}
			} else if !strings.HasPrefix(got, "ERR reward_unavailable ") {
				t.Fatal(got)
			}
		})
	}
}

func TestConcurrentDeliveryAndRewardClaims(t *testing.T) {
	s, c, def := rewardTestState(t)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; s.talk(c, "npc.herbalist", true) }()
	}
	close(start)
	wg.Wait()
	if !s.itemLocations[def.TargetItem].consumed || s.itemLocations[def.RewardItem].owner != c || s.rewardClaims[def.ID] != "alice" || c.hp != 100 {
		t.Fatal("invalid concurrent delivery")
	}
	completed := 0
	for len(c.outbox) > 0 {
		if strings.Contains(<-c.outbox, def.Dialogue) {
			completed++
		}
	}
	if completed != 1 {
		t.Fatalf("completion count=%d", completed)
	}
	// Allocation engine is also tested for the future combat reward.
	def = s.world.Rewards["quest.bandit_bounty"]
	start = make(chan struct{})
	results := make(chan bool, 2)
	for _, username := range []string{"alice", "bob"} {
		wg.Add(1)
		go func(username string) {
			defer wg.Done()
			<-start
			s.mu.Lock()
			defer s.mu.Unlock()
			results <- s.grantRewardLocked(&client{username: username, roomID: s.world.Start, hp: 40}, def)
		}(username)
	}
	close(start)
	wg.Wait()
	a, b := <-results, <-results
	if a == b {
		t.Fatalf("unique reward claim results: %v %v", a, b)
	}
	owner := s.itemLocations[def.RewardItem].owner
	if owner == nil || s.rewardClaims[def.ID] != owner.username {
		t.Fatal("reward has no unique winner")
	}
	s.mu.Lock()
	s.dropInventoryLocked(owner)
	if s.grantRewardLocked(c, def) {
		t.Fatal("dropping reward allowed another allocation")
	}
	s.mu.Unlock()
}

func TestDeliveredRewardCanBeDroppedAndTransferred(t *testing.T) {
	s, c, def := rewardTestState(t)
	s.talk(c, "npc.herbalist", true)
	<-c.outbox
	s.transferItem(c, "DROP", def.RewardItem, true)
	<-c.outbox
	bob := &client{username: "bob", roomID: c.roomID, hp: 9, outbox: make(chan string, 16)}
	s.transferItem(bob, "TAKE", def.RewardItem, true)
	<-bob.outbox
	s.useItem(bob, def.RewardItem, true)
	if got := <-bob.outbox; got != "OK used=item.vigor_potion hp=100/100" {
		t.Fatal(got)
	}
	if !s.itemLocations[def.RewardItem].consumed || s.rewardClaims[def.ID] != "alice" {
		t.Fatal("transfer lost unique claim")
	}
	s.dropInventoryLocked(bob)
	if !s.itemLocations[def.RewardItem].consumed {
		t.Fatal("disconnect resurrected potion")
	}
}

func TestWorldRejectsInvalidRewardDefinitions(t *testing.T) {
	data, err := os.ReadFile("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, key string
		value     any
		want      string
	}{
		{"unknown_giver", "giver", "npc.missing", "known quest_giver"},
		{"unknown_reward", "reward_item", "item.missing", "reserved item"},
		{"room_reward", "reward_item", "item.apple", "reserved item"},
		{"unknown_target", "target_item", "item.missing", "room quest item"},
		{"consumable_target", "target_item", "item.apple", "room quest item"},
		{"wrong_npc", "target_npc", "npc.guard", "delivery target"},
		{"negative_heal", "reward_hp_heal", -1, "nonnegative healing"},
		{"blank_completion", "completion_dialogue", " ", "completion dialogue"},
		{"unsupported_type", "type", "other", "unsupported type"},
		{"shared_reward", "reward_item", "item.ancient_key", "share a unique reward"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var file map[string]any
			if err := json.Unmarshal(data, &file); err != nil {
				t.Fatal(err)
			}
			world := file["world"].(map[string]any)
			quests := world["quests"].(map[string]any)
			quests["quest.herbal_cure"].(map[string]any)[tc.key] = tc.value
			encoded, _ := json.Marshal(file)
			path := t.TempDir() + "/world.json"
			if err := os.WriteFile(path, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadWorld(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q got %v", tc.want, err)
			}
		})
	}
}

func TestRewardDisconnectReturnsOnlyUnconsumedReward(t *testing.T) {
	s, addr, stop := lifecycleServer(t)
	defer stop()
	alice, ar := dialTestClient(t, addr)
	bob, br := dialTestClient(t, addr)
	sendCommand(t, alice, "CONNECT alice")
	expectLine(t, ar, "OK connected")
	sendCommand(t, alice, "MOVE north")
	expectLine(t, ar, "OK room=loc.garden")
	sendCommand(t, bob, "CONNECT bob")
	expectLine(t, br, "OK connected")
	sendCommand(t, bob, "MOVE north")
	expectLine(t, br, "OK room=loc.garden")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, alice, "TAKE item.rare_herbs")
	expectLine(t, ar, "OK taken=item.rare_herbs")
	expectLine(t, ar, "EVT ROOM ITEM TAKE alice item.rare_herbs")
	expectLine(t, br, "EVT ROOM ITEM TAKE alice item.rare_herbs")
	sendCommand(t, alice, "TALK npc.herbalist")
	var response map[string]any
	readWorldReply(t, ar, &response)
	for _, action := range []struct{ verb, id string }{{"DELIVER", "item.rare_herbs"}, {"REWARD", "item.vigor_potion"}} {
		event := fmt.Sprintf("EVT ROOM ITEM %s alice %s", action.verb, action.id)
		expectLine(t, ar, event)
		expectLine(t, br, event)
	}
	_ = alice.Close()
	expectLine(t, br, "EVT ROOM ITEM DROP alice item.vigor_potion")
	expectLine(t, br, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, bob, "LOOK")
	expectRoomItems(t, br, []string{"item.vigor_potion"})
	sendCommand(t, bob, "TAKE item.vigor_potion")
	expectLine(t, br, "OK taken=item.vigor_potion")
	expectLine(t, br, "EVT ROOM ITEM TAKE bob item.vigor_potion")
	sendCommand(t, bob, "TALK npc.herbalist")
	expectLine(t, br, "ERR reward_unavailable TALK reward for quest.herbal_cure has already been awarded in this world")
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.itemLocations["item.rare_herbs"].consumed || s.itemLocations["item.vigor_potion"].owner != s.players["bob"] {
		t.Fatal("disconnect recreated ingredient or lost transferred reward")
	}
}

func TestRewardAllocatorRejectsForgedDefinition(t *testing.T) {
	s, c, def := rewardTestState(t)
	forged := def
	forged.Heal++
	if s.grantRewardLocked(c, forged) {
		t.Fatal("forged reward definition accepted")
	}
	forged = def
	forged.ID = "quest.fake"
	if s.grantRewardLocked(c, forged) {
		t.Fatal("unknown quest reward accepted")
	}
	anonymous := &client{hp: 10}
	if s.grantRewardLocked(anonymous, def) {
		t.Fatal("anonymous reward recipient accepted")
	}
	if len(s.rewardClaims) != 0 || !s.itemLocations[def.RewardItem].reserved {
		t.Fatal("failed grant mutated reward")
	}
}

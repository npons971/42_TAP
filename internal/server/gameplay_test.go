package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func combatClient(t *testing.T) (*Server, *client) {
	t.Helper()
	s, c, _ := rewardTestState(t)
	c.roomID = "loc.ruins_den"
	c.hp = 100
	s.players[c.username] = c
	return s, c
}
func turnReply(t *testing.T, c *client) combatResult {
	t.Helper()
	line := <-c.outbox
	if !strings.HasPrefix(line, "OK ") {
		t.Fatal(line)
	}
	var result combatResult
	if err := json.Unmarshal([]byte(line[3:]), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func drain(c *client) {
	for len(c.outbox) > 0 {
		<-c.outbox
	}
}

func TestCombatVictoryAndQuestTCP(t *testing.T) {
	s, addr, stop := lifecycleServer(t)
	defer stop()
	conn, r := dialTestClient(t, addr)
	sendCommand(t, conn, "CONNECT alice")
	expectLine(t, r, "OK connected")
	sendCommand(t, conn, "QUESTINFO quest.bandit_bounty")
	var details struct {
		Status    string `json:"status"`
		Objective struct {
			Completed bool `json:"completed"`
		} `json:"objective"`
	}
	readWorldReply(t, r, &details)
	if details.Status != "NOT_STARTED" {
		t.Fatal(details)
	}
	// Acquire the weapon, then accept the bounty at its giver.
	for _, step := range []struct{ command, reply string }{
		{"MOVE west", "OK room=loc.dark_alley"},
		{"TAKE Rusty Sword", "OK taken=item.rusty_sword"},
	} {
		sendCommand(t, conn, step.command)
		expectLine(t, r, step.reply)
	}
	expectLine(t, r, "EVT ROOM ITEM TAKE alice item.rusty_sword")
	for _, step := range []struct{ dir, id string }{{"south", "loc.town_square"}, {"east", "loc.market"}, {"east", "loc.ruins_gate"}} {
		sendCommand(t, conn, "MOVE "+step.dir)
		expectLine(t, r, "OK room="+step.id)
	}
	sendCommand(t, conn, "TALKJSON npc.guard_captain")
	var dialogue map[string]any
	readWorldReply(t, r, &dialogue)
	sendCommand(t, conn, "QUESTINFO quest.bandit_bounty")
	readWorldReply(t, r, &details)
	if details.Status != "IN_PROGRESS" {
		t.Fatal(details)
	}
	sendCommand(t, conn, "MOVE north")
	expectLine(t, r, "OK room=loc.ruins_den")
	for i := 0; i < 4; i++ {
		sendCommand(t, conn, "ATTACK BANDIT LEADER")
		var result combatResult
		readWorldReply(t, r, &result)
		line, err := readTestLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(line, "EVT ROOM COMBAT ") {
			t.Fatal(line)
		}
		if result.PlayerHP != 100-min(i+1, 3)*20 || result.TargetHP != max(0, 70-(i+1)*22) {
			t.Fatalf("turn %d: %+v", i, result)
		}
		if i == 0 {
			sendCommand(t, conn, "STATUS")
			var status struct {
				State  string      `json:"state"`
				Combat *combatView `json:"combat"`
			}
			readWorldReply(t, r, &status)
			if status.State != "EN_COMBAT" || status.Combat.TargetHP != 48 {
				t.Fatal(status)
			}
			sendCommand(t, conn, "MOVE south")
			expectLine(t, r, "ERR 409 IN_COMBAT MOVE is unavailable in combat; use FLEE")
			sendCommand(t, conn, "USE item.rusty_sword")
			expectLine(t, r, "ERR 409 IN_COMBAT USE is unavailable in combat; use FLEE")
		}
		if i == 3 && result.Outcome != "victory" {
			t.Fatal(result)
		}
	}
	sendCommand(t, conn, "QUESTINFO quest.bandit_bounty")
	readWorldReply(t, r, &details)
	if details.Status != "OBJECTIVES_MET" || !details.Objective.Completed {
		t.Fatal(details)
	}
	sendCommand(t, conn, "LOOK DETAILS")
	var room struct {
		NPCs []npcView `json:"npcs"`
	}
	readWorldReply(t, r, &room)
	if len(room.NPCs) != 0 {
		t.Fatal("defeated enemy still visible")
	}
	sendCommand(t, conn, "ATTACK npc.bandit_leader")
	expectLine(t, r, `ERR 404 NPC_NOT_FOUND ATTACK target "npc.bandit_leader" is not in loc.ruins_den; use LOOK`)
	sendCommand(t, conn, "MOVE south")
	expectLine(t, r, "OK room=loc.ruins_gate")
	sendCommand(t, conn, "TALKJSON Guard Captain")
	readWorldReply(t, r, &dialogue)
	if dialogue["dialogue"] != s.world.Rewards["quest.bandit_bounty"].Dialogue {
		t.Fatal(dialogue)
	}
	expectLine(t, r, "EVT ROOM ITEM REWARD alice item.ancient_key")
	sendCommand(t, conn, "QUESTINFO quest.bandit_bounty")
	readWorldReply(t, r, &details)
	if details.Status != "COMPLETED" {
		t.Fatal(details)
	}
	sendCommand(t, conn, "INVENTORY DETAILS")
	var inventory []inventoryItem
	readWorldReply(t, r, &inventory)
	if len(inventory) != 2 || inventory[0].ID != "item.ancient_key" {
		t.Fatal(inventory)
	}
	sendCommand(t, conn, "QUESTS DETAILS")
	var list []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	readWorldReply(t, r, &list)
	if len(list) != 2 || list[0].Status != "COMPLETED" {
		t.Fatal(list)
	}
}

func TestCombatDefeatDefendAndFlee(t *testing.T) {
	t.Run("respawn", func(t *testing.T) {
		s, c := combatClient(t)
		c.hp = 10
		s.attack(c, "npc.bandit_leader", true)
		result := turnReply(t, c)
		if result.Outcome != "respawn" || result.CounterDamage != 10 || c.hp != 30 || c.roomID != s.world.Respawn || c.target != "" || s.npcStates["npc.bandit_leader"].Opponent != nil {
			t.Fatalf("bad respawn: %+v %+v", result, c)
		}
		if s.progressLocked(c.username).Defeated["npc.bandit_leader"] {
			t.Fatal("defeat credited as victory")
		}
	})
	t.Run("defend", func(t *testing.T) {
		s, c := combatClient(t)
		s.attack(c, "npc.bandit_leader", true)
		turnReply(t, c)
		drain(c)
		s.itemLocations["item.wooden_shield"] = itemLocation{owner: c}
		s.combatAction(c, []string{"DEFEND"})
		result := turnReply(t, c)
		if result.CounterDamage != 7 || c.hp != 73 || result.Damage != 0 || c.target == "" {
			t.Fatal(result)
		}
	})
	for _, roll := range []int{0, 99} {
		t.Run(fmt.Sprintf("flee_%d", roll), func(t *testing.T) {
			s, c := combatClient(t)
			s.attack(c, "npc.bandit_leader", true)
			turnReply(t, c)
			drain(c)
			s.fleeRoll = func() int { return roll }
			s.combatAction(c, []string{"FLEE", "south"})
			result := turnReply(t, c)
			if roll == 0 {
				if result.Outcome != "fled" || c.roomID != "loc.ruins_gate" || c.target != "" || c.hp != 80 {
					t.Fatal(result)
				}
			} else if result.Outcome != "flee_failed" || c.hp != 60 || c.target == "" {
				t.Fatal(result)
			}
		})
	}
}

func TestCombatConcurrencyAndAtomicFailure(t *testing.T) {
	s, alice := combatClient(t)
	bob := &client{username: "bob", roomID: alice.roomID, hp: 100, outbox: make(chan string, 16)}
	s.players[bob.username] = bob
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for _, c := range []*client{alice, bob} {
		wg.Add(1)
		go func(c *client) { defer wg.Done(); <-gate; s.attack(c, "npc.bandit_leader", true) }(c)
	}
	close(gate)
	wg.Wait()
	successes, busy := 0, 0
	for _, c := range []*client{alice, bob} {
		for len(c.outbox) > 0 {
			line := <-c.outbox
			if strings.HasPrefix(line, "OK ") {
				successes++
			}
			if strings.HasPrefix(line, "ERR 409 TARGET_BUSY") {
				busy++
			}
		}
	}
	if successes != 1 || busy != 1 || s.npcStates["npc.bandit_leader"].HP != 58 {
		t.Fatalf("success=%d busy=%d", successes, busy)
	}
	owner := s.npcStates["npc.bandit_leader"].Opponent
	hp := owner.hp
	owner.outbox = make(chan string, 1)
	owner.outbox <- "blocked"
	if s.attack(owner, "npc.bandit_leader", true) {
		t.Fatal("full queue accepted combat")
	}
	if owner.hp != hp || s.npcStates["npc.bandit_leader"].HP != 58 {
		t.Fatal("failed reply mutated combat")
	}
	s.releaseCombatLocked(owner)
	if s.npcStates["npc.bandit_leader"].Opponent != nil {
		t.Fatal("disconnect kept enemy locked")
	}
}

func TestQuestPossessionProgressAndReconnect(t *testing.T) {
	s, c, def := rewardTestState(t)
	if got := s.questStatusLocked(c, def); got != "NOT_STARTED" {
		t.Fatal(got)
	}
	// TALK to the giver accepts and may complete immediately if possession proves it.
	s.itemLocations[def.TargetItem] = itemLocation{roomID: c.roomID}
	s.talkReply(c, "npc.herbalist", true, true)
	drain(c)
	if got := s.questStatusLocked(c, def); got != "IN_PROGRESS" {
		t.Fatal(got)
	}
	s.transferItem(c, "TAKE", def.TargetItem, true)
	drain(c)
	if got := s.questStatusLocked(c, def); got != "OBJECTIVES_MET" {
		t.Fatal(got)
	}
	s.transferItem(c, "DROP", def.TargetItem, true)
	drain(c)
	if got := s.questStatusLocked(c, def); got != "IN_PROGRESS" {
		t.Fatal(got)
	}
	again := &client{username: c.username, roomID: c.roomID, hp: 100, outbox: make(chan string, 16)}
	if got := s.questStatusLocked(again, def); got != "IN_PROGRESS" {
		t.Fatal("reconnect lost progress")
	}
	s.transferItem(again, "TAKE", def.TargetItem, true)
	drain(again)
	s.talkReply(again, "npc.herbalist", true, true)
	drain(again)
	if got := s.questStatusLocked(c, def); got != "COMPLETED" {
		t.Fatal(got)
	}
	other := &client{username: "bob"}
	if got := s.questStatusLocked(other, def); got != "UNAVAILABLE" {
		t.Fatal(got)
	}
}

func TestNewCommandArgumentErrors(t *testing.T) {
	s, c := combatClient(t)
	for _, tc := range []struct{ command, code string }{
		{"ATTACK", "invalid_arguments"}, {"ATTACK npc.bandit_leader ", "invalid_arguments"}, {"ATTACK npc.guard", "target_not_found"},
		{"QUEST", "invalid_arguments"}, {"QUESTS extra", "invalid_arguments"}, {"QUESTINFO quest.missing", "quest_not_found"},
		{"GROUP", "invalid_arguments"}, {"GROUP CREATE ARIS", "invalid_arguments"}, {"GROUP JOIN missing", "group_not_found"},
		{"GROUP LEAVE extra", "invalid_arguments"}, {"GROUP LEAVE", "group_required"}, {"GROUP DANCE foo", "invalid_arguments"},
		{"DEFEND extra", "invalid_arguments"}, {"DEFEND", "not_in_combat"}, {"FLEE", "not_in_combat"},
	} {
		s.handleCommand(c, tc.command)
		line := <-c.outbox
		if !strings.HasPrefix(line, "ERR "+errorSpecs[tc.code]+" ") {
			t.Fatalf("%s: %s", tc.command, line)
		}
	}
}

func TestCombatRestrictionsAndBoundedDamage(t *testing.T) {
	s, c := combatClient(t)
	c.roomID = "loc.town_square"
	s.attack(c, "npc.guard", true)
	if got := <-c.outbox; !strings.HasPrefix(got, "ERR 405 NPC_NOT_HOSTILE ") {
		t.Fatal(got)
	}
	c.roomID = "loc.ruins_den"
	s.attack(c, "npc.bandit_leader", true)
	turnReply(t, c)
	drain(c)
	for _, command := range []string{"TAKE item.missing", "DROP item.missing", "TALK npc.bandit_leader", "USE item.missing"} {
		s.handleCommand(c, command)
		if got := <-c.outbox; !strings.HasPrefix(got, "ERR 409 IN_COMBAT ") {
			t.Fatalf("%s: %s", command, got)
		}
	}
	s.combatAction(c, []string{"FLEE", "up"})
	if got := <-c.outbox; !strings.HasPrefix(got, "ERR 301 NO_EXIT ") {
		t.Fatal(got)
	}
	if c.hp != 80 || s.npcStates[c.target].HP != 58 {
		t.Fatal("invalid escape spent a turn")
	}
	// Overflow-sized bonuses still apply at most the remaining enemy HP.
	item := s.world.Items["item.rusty_sword"]
	item.DamageBonus = int(^uint(0) >> 1)
	s.world.Items[item.ID] = item
	s.itemLocations[item.ID] = itemLocation{owner: c}
	s.attack(c, "npc.bandit_leader", true)
	result := turnReply(t, c)
	if result.Damage != 58 || result.CounterDamage != 0 || result.Outcome != "victory" || c.hp != 80 {
		t.Fatal(result)
	}
}

func TestQuestSizeLimitAndEmptyWorld(t *testing.T) {
	s, c, def := rewardTestState(t)
	def.Description = strings.Repeat("x", maxLineBytes)
	s.world.Rewards[def.ID] = def
	s.quests(c, []string{"QUESTINFO", def.ID})
	if got := <-c.outbox; got != "ERR 413 RESPONSE_TOO_LARGE QUESTINFO response exceeds 4096-byte line limit" {
		t.Fatal(got)
	}
	if len(s.questProgress) != 0 {
		t.Fatal("read-only query changed progress")
	}
	s.world.Rewards = map[string]RewardDefinition{}
	s.quests(c, []string{"QUESTS"})
	if got := <-c.outbox; got != "OK []" {
		t.Fatal(got)
	}
}

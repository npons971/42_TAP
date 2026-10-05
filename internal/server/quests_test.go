package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type journalStep struct {
	Type         string `json:"type"`
	Label        string `json:"label"`
	TargetID     string `json:"target_id"`
	LocationID   string `json:"location_id"`
	LocationName string `json:"location_name"`
	Completed    bool   `json:"completed"`
	Hint         string `json:"hint"`
}

type journalEntry struct {
	ID                string        `json:"id"`
	Title             string        `json:"title"`
	Description       string        `json:"description"`
	Status            string        `json:"status"`
	Giver             string        `json:"giver"`
	GiverName         string        `json:"giver_name"`
	GiverLocationID   string        `json:"giver_location_id"`
	GiverLocationName string        `json:"giver_location_name"`
	Reward            string        `json:"reward"`
	RewardHPHeal      int           `json:"reward_hp_heal"`
	Steps             []journalStep `json:"steps"`
	Objective         struct {
		Type       string `json:"type"`
		TargetItem string `json:"target_item"`
		TargetNPC  string `json:"target_npc"`
		Completed  bool   `json:"completed"`
	} `json:"objective"`
}

func journalReply(t *testing.T, s *Server, c *client, args ...string) string {
	t.Helper()
	if !s.quests(c, args) {
		t.Fatal("journal query could not queue a reply")
	}
	line := <-c.outbox
	if !strings.HasPrefix(line, "OK ") {
		t.Fatal(line)
	}
	if len(line)+1 > maxLineBytes {
		t.Fatalf("journal response has %d bytes including LF", len(line)+1)
	}
	return line
}

func journalInfo(t *testing.T, s *Server, c *client, id string) journalEntry {
	t.Helper()
	line := journalReply(t, s, c, "QUESTINFO", id)
	var entry journalEntry
	if err := json.Unmarshal([]byte(line[3:]), &entry); err != nil {
		t.Fatal(err)
	}
	if len(entry.Steps) != 2 {
		t.Fatalf("quest needs objective and turn-in steps: %+v", entry)
	}
	return entry
}

func TestQuestJournalAvailableReadOnly(t *testing.T) {
	s, c, fetch := rewardTestState(t)
	s.itemLocations[fetch.TargetItem] = itemLocation{roomID: "loc.garden"}
	line := journalReply(t, s, c, "QUESTS", "DETAILS")
	var entries []journalEntry
	if err := json.Unmarshal([]byte(line[3:]), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ID != "quest.bandit_bounty" || entries[1].ID != fetch.ID {
		t.Fatalf("all definitions must be sorted: %+v", entries)
	}
	for _, entry := range entries {
		def := s.world.Rewards[entry.ID]
		if entry.Status != "NOT_STARTED" || entry.Title != def.Title || entry.Description != def.Description || entry.Giver != def.Giver || entry.GiverName != s.world.NPCs[def.Giver].Name || entry.Reward != s.world.Items[def.RewardItem].Name || entry.RewardHPHeal != def.Heal {
			t.Fatalf("incomplete readable definition: %+v", entry)
		}
		if len(entry.Steps) != 2 || entry.Steps[0].Completed || entry.Steps[1].Completed || entry.GiverLocationID == "" || entry.GiverLocationName != s.world.Locations[entry.GiverLocationID].Name || entry.Steps[1].TargetID != def.Giver || entry.Steps[1].LocationID != entry.GiverLocationID {
			t.Fatalf("missing turn-in guidance: %+v", entry)
		}
		journalInfo(t, s, c, entry.ID)
	}
	if len(s.questProgress) != 0 || len(s.rewardClaims) != 0 || !s.itemLocations[fetch.RewardItem].reserved || s.itemLocations[fetch.TargetItem].owner != nil {
		t.Fatal("read-only journal accepted a quest or changed world state")
	}
	if got := journalReply(t, s, c, "QUESTS"); got != "OK []" {
		t.Fatal("base QUESTS includes unaccepted quests:", got)
	}
}

func TestQuestJournalFetchPossessionDropAndCompletion(t *testing.T) {
	s, c, def := rewardTestState(t)
	s.itemLocations[def.TargetItem] = itemLocation{roomID: c.roomID}
	if !s.requestQuest(c, def.Giver, true) {
		t.Fatal("could not accept quest")
	}
	// The RFC reply remains exactly the original four fields.
	line := <-c.outbox
	want, _ := json.Marshal(struct {
		ID          string `json:"quest_id"`
		Description string `json:"description"`
		Reward      string `json:"reward"`
		Status      string `json:"status"`
	}{def.ID, def.Description, def.RewardItem, "available"})
	if line != "OK "+string(want) {
		t.Fatal("QUEST wire format changed:", line)
	}
	entry := journalInfo(t, s, c, def.ID)
	if entry.Status != "IN_PROGRESS" || entry.Steps[0].Type != "FETCH" || entry.Steps[0].TargetID != def.TargetItem || entry.Steps[1].Type != "DELIVER" || !strings.Contains(entry.Steps[0].Label, s.world.Items[def.TargetItem].Name) || !strings.Contains(entry.Steps[1].Label, entry.GiverName) || entry.Objective.Type != "FETCH" || entry.Objective.TargetItem != def.TargetItem || entry.Objective.TargetNPC != def.TargetNPC {
		t.Fatalf("incorrect fetch guidance/objective: %+v", entry)
	}
	s.transferItem(c, "TAKE", def.TargetItem, true)
	drain(c)
	entry = journalInfo(t, s, c, def.ID)
	if entry.Status != "OBJECTIVES_MET" || !entry.Steps[0].Completed || entry.Steps[1].Completed || !entry.Objective.Completed {
		t.Fatalf("possession must leave delivery pending: %+v", entry)
	}
	if got := journalReply(t, s, c, "QUESTS"); got != fmt.Sprintf(`OK [{"quest_id":%q,"status":"active","progress":"1/1"}]`, def.ID) {
		t.Fatal("base QUESTS wire format changed:", got)
	}
	c.roomID = "loc.town_square"
	s.transferItem(c, "DROP", def.TargetItem, true)
	drain(c)
	entry = journalInfo(t, s, c, def.ID)
	if entry.Status != "IN_PROGRESS" || entry.Steps[0].Completed || entry.Objective.Completed || entry.Steps[0].LocationID != c.roomID || entry.Steps[0].LocationName != s.world.Locations[c.roomID].Name {
		t.Fatalf("dropped ingredient must use its new room: %+v", entry)
	}
	bob := &client{username: "bob", roomID: c.roomID, outbox: make(chan string, 16)}
	s.transferItem(bob, "TAKE", def.TargetItem, true)
	drain(bob)
	entry = journalInfo(t, s, c, def.ID)
	if entry.Steps[0].Completed || entry.Steps[0].LocationID != "loc.garden" || !strings.Contains(entry.Steps[0].Hint, "Another player") || !strings.Contains(entry.Steps[0].Hint, "not currently on the ground") {
		t.Fatalf("another player's inventory must not look collectible: %+v", entry)
	}
	s.transferItem(bob, "DROP", def.TargetItem, true)
	drain(bob)
	s.transferItem(c, "TAKE", def.TargetItem, true)
	drain(c)
	c.roomID = "loc.garden"
	s.talkReply(c, def.Giver, true, true)
	drain(c)
	entry = journalInfo(t, s, c, def.ID)
	if entry.Status != "COMPLETED" || !entry.Steps[0].Completed || !entry.Steps[1].Completed || !entry.Objective.Completed || !s.itemLocations[def.TargetItem].consumed || entry.Steps[0].Hint != "" {
		t.Fatalf("consumption must preserve completed steps: %+v", entry)
	}
	if got := journalReply(t, s, c, "QUESTS"); got != fmt.Sprintf(`OK [{"quest_id":%q,"status":"completed"}]`, def.ID) {
		t.Fatal("completed RFC reply changed:", got)
	}
	entry = journalInfo(t, s, bob, def.ID)
	if entry.Status != "UNAVAILABLE" || entry.Steps[0].Completed || entry.Steps[1].Completed {
		t.Fatalf("another player cannot inherit a completed quest: %+v", entry)
	}
}

func TestQuestJournalConsumedSharedIngredient(t *testing.T) {
	s, alice, def := rewardTestState(t)
	shared := def
	shared.ID, shared.Giver, shared.TargetNPC, shared.RewardItem = "quest.shared_herbs", "npc.guard_captain", "npc.guard_captain", "item.captains_shield"
	raw, err := json.Marshal(shared)
	if err != nil {
		t.Fatal(err)
	}
	delete(s.world.Quests, "quest.bandit_bounty")
	s.world.Quests[shared.ID] = raw
	if err := s.world.validateRewards(); err != nil {
		t.Fatal("distinct quests may share an ingredient:", err)
	}
	s.talkReply(alice, def.Giver, true, true)
	drain(alice)
	bob := &client{username: "bob", outbox: make(chan string, 16)}
	entry := journalInfo(t, s, bob, shared.ID)
	if entry.Status != "NOT_STARTED" || entry.Steps[0].Completed || entry.Steps[1].Completed || entry.Steps[0].LocationID != "loc.garden" || !strings.Contains(entry.Steps[0].Hint, "already been delivered") || !strings.Contains(entry.Steps[0].Hint, "no longer available") {
		t.Fatalf("shared consumed ingredient must explain unavailability: %+v", entry)
	}
	if s.rewardClaims[shared.ID] != "" || !s.itemLocations[shared.RewardItem].reserved {
		t.Fatal("unclaimed shared quest must keep its own reward available")
	}
}

func TestQuestJournalDefeatPersonalVictoryAndReport(t *testing.T) {
	s, c := combatClient(t)
	def := s.world.Rewards["quest.bandit_bounty"]
	entry := journalInfo(t, s, c, def.ID)
	if entry.Steps[0].Type != "DEFEAT" || entry.Steps[0].TargetID != def.DefeatNPC || entry.Steps[0].LocationID != "loc.ruins_den" || entry.Steps[1].Type != "REPORT" || entry.Steps[1].LocationID != "loc.ruins_gate" || entry.Objective.Type != "DEFEAT" || entry.Objective.TargetNPC != def.DefeatNPC || entry.Objective.TargetItem != "" {
		t.Fatalf("incorrect defeat/report guidance: %+v", entry)
	}
	// A victory before acceptance still counts, but the report stays pending.
	s.itemLocations["item.rusty_sword"] = itemLocation{owner: c}
	for s.npcStates[def.DefeatNPC].HP > 0 {
		s.attack(c, def.DefeatNPC, true)
		turnReply(t, c)
		drain(c)
	}
	entry = journalInfo(t, s, c, def.ID)
	if entry.Status != "NOT_STARTED" || !entry.Steps[0].Completed || entry.Steps[1].Completed || entry.Steps[0].Hint != "" {
		t.Fatalf("personal victory before acceptance must remain valid: %+v", entry)
	}
	bob := &client{username: "bob", roomID: "loc.ruins_gate", hp: 100, outbox: make(chan string, 16)}
	s.requestQuest(bob, def.Giver, true)
	drain(bob)
	entry = journalInfo(t, s, bob, def.ID)
	if entry.Status != "IN_PROGRESS" || entry.Steps[0].Completed || entry.Objective.Completed || !strings.Contains(entry.Steps[0].Hint, "defeated by another player") || !strings.Contains(entry.Steps[0].Hint, "does not respawn") {
		t.Fatalf("someone else's final blow must not count: %+v", entry)
	}
	c.roomID = "loc.ruins_gate"
	s.requestQuest(c, def.Giver, true)
	drain(c)
	entry = journalInfo(t, s, c, def.ID)
	if entry.Status != "OBJECTIVES_MET" || !entry.Steps[0].Completed || entry.Steps[1].Completed {
		t.Fatalf("report must remain pending: %+v", entry)
	}
	s.talkReply(c, def.Giver, true, true)
	drain(c)
	entry = journalInfo(t, s, c, def.ID)
	if entry.Status != "COMPLETED" || !entry.Steps[0].Completed || !entry.Steps[1].Completed {
		t.Fatalf("reported victory did not complete both steps: %+v", entry)
	}
	entry = journalInfo(t, s, bob, def.ID)
	if entry.Status != "UNAVAILABLE" || entry.Steps[0].Completed || entry.Steps[1].Completed {
		t.Fatalf("global reward claim must block another player: %+v", entry)
	}
}

func TestQuestJournalUTF8LineBound(t *testing.T) {
	for _, args := range [][]string{{"QUESTS", "DETAILS"}, {"QUESTINFO", "quest.herbal_cure"}} {
		t.Run(args[0], func(t *testing.T) {
			s, c, def := rewardTestState(t)
			def.Description = ""
			s.world.Rewards[def.ID] = def
			line := journalReply(t, s, c, args...)
			remaining := maxLineBytes - len(line) - 1
			def.Description = strings.Repeat("é", remaining/2) + strings.Repeat("x", remaining%2)
			s.world.Rewards[def.ID] = def
			line = journalReply(t, s, c, args...)
			if len(line)+1 != maxLineBytes {
				t.Fatalf("expected exact line boundary, got %d", len(line)+1)
			}
			def.Description += "x"
			s.world.Rewards[def.ID] = def
			if !s.quests(c, args) {
				t.Fatal("could not queue size error")
			}
			if got := <-c.outbox; got != "ERR 413 RESPONSE_TOO_LARGE "+args[0]+" response exceeds 4096-byte line limit" {
				t.Fatal(got)
			}
			if len(s.questProgress) != 0 || len(s.rewardClaims) != 0 {
				t.Fatal("line overflow mutated quest state")
			}
		})
	}
}

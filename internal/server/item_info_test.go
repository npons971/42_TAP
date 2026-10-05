package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"reflect"
	"strings"
	"testing"
)

func itemInfoState(t *testing.T) (*Server, *client) {
	t.Helper()
	s, c, _ := rewardTestState(t)
	s.players[c.username] = c
	s.itemLocations["item.rusty_sword"] = itemLocation{owner: c}
	return s, c
}

func infoCommandReply(t *testing.T, s *Server, c *client, command string) string {
	t.Helper()
	beforeClient := *c
	beforeItems := maps.Clone(s.itemLocations)
	beforeClaims := maps.Clone(s.rewardClaims)
	beforeProgress, _ := json.Marshal(s.questProgress)
	if !s.handleCommand(c, command) {
		t.Fatalf("command rejected queue: %s", command)
	}
	afterProgress, _ := json.Marshal(s.questProgress)
	if !reflect.DeepEqual(beforeClient, *c) || !reflect.DeepEqual(beforeItems, s.itemLocations) || !reflect.DeepEqual(beforeClaims, s.rewardClaims) || string(beforeProgress) != string(afterProgress) {
		t.Fatalf("read command mutated state: %s", command)
	}
	select {
	case line := <-c.outbox:
		return line
	default:
		t.Fatalf("no reply for %s", command)
		return ""
	}
}

func decodeItemInfo(t *testing.T, line string) itemView {
	t.Helper()
	if !strings.HasPrefix(line, "OK ") {
		t.Fatal(line)
	}
	var result itemView
	if err := json.Unmarshal([]byte(line[3:]), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestItemInfoAccessibleScopesAndNames(t *testing.T) {
	s, c := itemInfoState(t)
	c.roomID = "loc.town_square"
	for _, tc := range []struct{ target, id string }{
		{"item.rusty_sword", "item.rusty_sword"},
		{"rUsTy SwOrD", "item.rusty_sword"},
		{"item.rare_herbs", "item.rare_herbs"},
		{"sToNe FoUnTaIn", "item.fountain"},
		{"item.fountain", "item.fountain"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			item := decodeItemInfo(t, infoCommandReply(t, s, c, "info "+tc.target))
			catalogue := s.world.Items[tc.id]
			if item.ID != tc.id || item.Name != catalogue.Name || item.Description != catalogue.Description || item.Type != catalogue.Type || item.Obtainable != catalogue.Obtainable || item.DamageBonus != catalogue.DamageBonus || item.DefenseBonus != catalogue.DefenseBonus || item.HealValue != catalogue.HealValue || item.Usage == "" {
				t.Fatalf("descriptor=%+v, catalogue=%+v", item, catalogue)
			}
		})
	}
	// Taking an item out of the current room does not reveal the new owner.
	s.itemLocations["item.fountain"] = itemLocation{owner: &client{username: "bob", roomID: c.roomID}}
	for _, tc := range []struct {
		target   string
		location itemLocation
	}{
		{"item.fountain", s.itemLocations["item.fountain"]},
		{"item.rusty_sword", itemLocation{roomID: "loc.ruins_den"}},
		{"item.rusty_sword", itemLocation{reserved: true}},
		{"item.rusty_sword", itemLocation{consumed: true}},
		{"item.rusty_sword", itemLocation{owner: c, reserved: true}},
		{"item.rusty_sword", itemLocation{owner: c, consumed: true}},
	} {
		s.itemLocations[tc.target] = tc.location
		for _, target := range []string{tc.target, strings.ToUpper(s.world.Items[tc.target].Name)} {
			got := infoCommandReply(t, s, c, "INFO "+target)
			want := fmt.Sprintf("ERR 404 ITEM_NOT_FOUND INFO target %q is not in your inventory or current room; use INVENTORY or LOOK", shortVerb(target))
			if got != want {
				t.Fatalf("inaccessible item disclosed: %s", got)
			}
		}
	}
	if got := infoCommandReply(t, s, c, "INFO Rusty"); !strings.HasPrefix(got, "ERR 404 ITEM_NOT_FOUND ") {
		t.Fatal("partial name resolved: " + got)
	}
}

func TestItemInfoValidationAuthenticationAndAmbiguity(t *testing.T) {
	s, c := itemInfoState(t)
	c.username = ""
	if got := infoCommandReply(t, s, c, "INFO item.rusty_sword"); got != "ERR 403 NOT_AUTHENTICATED Send CONNECT <username> before INFO" {
		t.Fatal(got)
	}
	c.username = "alice"
	for _, tc := range []struct{ command, want string }{
		{"INFO", "ERR 400 INVALID_ARGUMENTS Usage: INFO <item_id or full display name>"},
		{"INFO   ", "ERR 400 INVALID_ARGUMENTS Usage: INFO <item_id or full display name>"},
		{"INFO item.rusty_sword ", "ERR 400 INVALID_ARGUMENTS INFO target must not contain tabs or surrounding spaces"},
		{"INFO  item.rusty_sword", "ERR 400 INVALID_ARGUMENTS INFO target must not contain tabs or surrounding spaces"},
		{"INFO Rusty\tSword", "ERR 400 INVALID_ARGUMENTS INFO target must not contain tabs or surrounding spaces"},
	} {
		if got := infoCommandReply(t, s, c, tc.command); got != tc.want {
			t.Fatalf("%q: %s", tc.command, got)
		}
	}
	duplicate := s.world.Items["item.rusty_sword"]
	duplicate.ID = "item.sword_duplicate"
	s.world.Items[duplicate.ID] = duplicate
	s.itemLocations[duplicate.ID] = itemLocation{roomID: c.roomID}
	if got := infoCommandReply(t, s, c, "INFO RUSTY SWORD"); got != "ERR 400 INVALID_ARGUMENTS INFO name matches multiple items; use an item ID" {
		t.Fatal(got)
	}
	if item := decodeItemInfo(t, infoCommandReply(t, s, c, "INFO item.rusty_sword")); item.ID != "item.rusty_sword" {
		t.Fatal(item)
	}
	// An inaccessible matching name must not make a visible item ambiguous.
	s.itemLocations[duplicate.ID] = itemLocation{reserved: true}
	if item := decodeItemInfo(t, infoCommandReply(t, s, c, "INFO RUSTY SWORD")); item.ID != "item.rusty_sword" {
		t.Fatal(item)
	}
}

func TestItemInfoReadOnlyInCombatAndQuestUsage(t *testing.T) {
	s, c := itemInfoState(t)
	c.roomID = "loc.ruins_den"
	c.hp = 100
	s.attack(c, "npc.bandit_leader", true)
	drain(c)
	if c.target == "" {
		t.Fatal("combat did not start")
	}
	beforeClient := *c
	beforeNPC := *s.npcStates[c.target]
	beforeItems := maps.Clone(s.itemLocations)
	beforeProgress, _ := json.Marshal(s.questProgress)
	beforeClaims := maps.Clone(s.rewardClaims)
	for _, target := range []string{"item.rare_herbs", "item.rusty_sword"} {
		item := decodeItemInfo(t, infoCommandReply(t, s, c, "INFO "+target))
		if target == "item.rare_herbs" && (!strings.Contains(item.Usage, "Village Herbalist") || !strings.Contains(item.Usage, "Garden") || !strings.Contains(item.Usage, "TALK npc.herbalist") || !strings.Contains(item.Usage, "delivery consumes")) {
			t.Fatal("missing quest delivery instructions: " + item.Usage)
		}
		if target == "item.rusty_sword" && (!strings.Contains(item.Usage, "automatically") || !strings.Contains(item.Usage, "highest damage bonus") || !strings.Contains(item.Usage, "highest defense bonus") || !strings.Contains(item.Usage, "do not stack")) {
			t.Fatal("missing equipment instructions: " + item.Usage)
		}
	}
	afterProgress, _ := json.Marshal(s.questProgress)
	if !reflect.DeepEqual(beforeClient, *c) || beforeNPC != *s.npcStates[c.target] || !reflect.DeepEqual(beforeItems, s.itemLocations) || string(beforeProgress) != string(afterProgress) || !reflect.DeepEqual(beforeClaims, s.rewardClaims) || len(c.outbox) != 0 {
		t.Fatal("INFO changed combat, items, quest progress, rewards or broadcast an event")
	}
}

func TestItemInfoMatchesDetailsAndPreservesStandardReplies(t *testing.T) {
	s, c := itemInfoState(t)
	c.roomID = "loc.town_square"
	line := infoCommandReply(t, s, c, "INVENTORY DETAILS")
	var inventory []itemView
	if err := json.Unmarshal([]byte(line[3:]), &inventory); err != nil {
		t.Fatal(err)
	}
	for _, item := range inventory {
		if got := decodeItemInfo(t, infoCommandReply(t, s, c, "INFO "+item.ID)); !reflect.DeepEqual(got, item) {
			t.Fatalf("INVENTORY and INFO differ: %+v vs %+v", item, got)
		}
	}
	var view struct {
		Items []itemView `json:"items"`
	}
	line = infoCommandReply(t, s, c, "LOOK DETAILS")
	if err := json.Unmarshal([]byte(line[3:]), &view); err != nil {
		t.Fatal(err)
	}
	for _, item := range view.Items {
		if got := decodeItemInfo(t, infoCommandReply(t, s, c, "INFO "+item.ID)); !reflect.DeepEqual(got, item) {
			t.Fatalf("LOOK and INFO differ: %+v vs %+v", item, got)
		}
	}
	if got := infoCommandReply(t, s, c, "INVENTORY"); got != `OK ["item.rare_herbs","item.rusty_sword"]` {
		t.Fatal("standard INVENTORY changed: " + got)
	}
	var standard struct {
		Items []string `json:"items"`
	}
	line = infoCommandReply(t, s, c, "LOOK")
	if err := json.Unmarshal([]byte(line[3:]), &standard); err != nil || !reflect.DeepEqual(standard.Items, []string{"item.fountain"}) {
		t.Fatalf("standard LOOK changed: %s (%v)", line, err)
	}
}

func TestItemInfoLineLimitAndFullQueueKeepState(t *testing.T) {
	s, c := itemInfoState(t)
	item := s.world.Items["item.rusty_sword"]
	item.Description = ""
	s.world.Items[item.ID] = item
	base, err := json.Marshal(s.itemViewLocked(item.ID))
	if err != nil {
		t.Fatal(err)
	}
	item.Description = strings.Repeat("x", maxLineBytes-4-len(base))
	s.world.Items[item.ID] = item
	if line := infoCommandReply(t, s, c, "INFO item.rusty_sword"); !strings.HasPrefix(line, "OK ") || len(line)+1 != maxLineBytes {
		t.Fatalf("exactly 4096-byte INFO response rejected: length=%d", len(line)+1)
	}
	item.Description += "x"
	s.world.Items[item.ID] = item
	if line := infoCommandReply(t, s, c, "INFO item.rusty_sword"); !strings.HasPrefix(line, "ERR 413 RESPONSE_TOO_LARGE ") {
		t.Fatal("4097-byte INFO response accepted: " + line)
	}
	item.Description = strings.Repeat("x", maxLineBytes)
	s.world.Items[item.ID] = item
	before := maps.Clone(s.itemLocations)
	for _, command := range []string{"INFO item.rusty_sword", "INVENTORY DETAILS"} {
		line := infoCommandReply(t, s, c, command)
		if !strings.HasPrefix(line, "ERR 413 RESPONSE_TOO_LARGE ") || len(line)+1 > maxLineBytes {
			t.Fatal(line)
		}
	}
	s.itemLocations[item.ID] = itemLocation{roomID: c.roomID}
	if line := infoCommandReply(t, s, c, "LOOK DETAILS"); !strings.HasPrefix(line, "ERR 413 RESPONSE_TOO_LARGE ") {
		t.Fatal(line)
	}
	s.itemLocations[item.ID] = before[item.ID]
	item.Description = ""
	s.world.Items[item.ID] = item
	c.outbox = make(chan string, 1)
	c.outbox <- "full"
	if s.handleCommand(c, "INFO item.rusty_sword") || !reflect.DeepEqual(before, s.itemLocations) {
		t.Fatal("full outbox accepted INFO or changed state")
	}
}

func TestUsefulReplacementItemsAndStrongestBonuses(t *testing.T) {
	s, c := itemInfoState(t)
	salve := s.world.Items["item.healing_salve"]
	shield := s.world.Items["item.captains_shield"]
	if salve.ID == "" || shield.ID == "" {
		t.Fatal("missing useful replacement items")
	}
	if salve.Type != "consumable" || salve.HealValue != 25 || shield.Type != "armor" || shield.DefenseBonus != 8 || !s.itemLocations[shield.ID].reserved {
		t.Fatalf("wrong replacements: %+v %+v", salve, shield)
	}
	s.itemLocations[salve.ID] = itemLocation{owner: c}
	info := decodeItemInfo(t, infoCommandReply(t, s, c, "INFO Healing Salve"))
	if info.HealValue != 25 || !strings.Contains(info.Usage, "USE item.healing_salve outside combat") {
		t.Fatal(info)
	}
	c.hp = 40
	s.useItem(c, salve.ID, true)
	if got := <-c.outbox; got != "OK used=item.healing_salve hp=65/100" || c.hp != 65 || !s.itemLocations[salve.ID].consumed {
		t.Fatalf("salve failed: %s hp=%d", got, c.hp)
	}
	drain(c)
	// Allocate the actual bounty reward to verify its reserved armor instance.
	def := s.world.Rewards["quest.bandit_bounty"]
	if def.RewardItem != shield.ID || !s.grantRewardLocked(c, def) {
		t.Fatal("bounty did not grant the shield")
	}
	s.itemLocations["item.wooden_shield"] = itemLocation{owner: c}
	if s.armorLocked(c) != 8 || s.playerDamageLocked(c, 100) != 22 {
		t.Fatal("carried equipment bonuses do not select the strongest independent stats")
	}
	s.useItem(c, shield.ID, true)
	if got := <-c.outbox; got != "ERR 405 ITEM_NOT_USABLE Item item.captains_shield cannot be consumed with USE" || s.itemLocations[shield.ID].owner != c {
		t.Fatalf("shield was consumable or lost: %s", got)
	}
	// A single hybrid item can improve only one selected stat; bonuses never sum.
	hybrid := Item{ID: "item.hybrid", Name: "Hybrid", DamageBonus: 4, DefenseBonus: 12}
	s.world.Items[hybrid.ID] = hybrid
	s.itemLocations[hybrid.ID] = itemLocation{owner: c}
	if s.armorLocked(c) != 12 || s.playerDamageLocked(c, 100) != 22 {
		t.Fatal("equipment bonuses stacked or failed independent selection")
	}
	c.roomID = "loc.ruins_den"
	s.attack(c, "npc.bandit_leader", true)
	turn := turnReply(t, c)
	if turn.Damage != 22 || turn.CounterDamage != 8 || c.hp != 57 {
		t.Fatalf("equipment bonuses did not apply in combat: %+v", turn)
	}
	for id, item := range s.world.Items {
		if !item.Obtainable || item.HealValue > 0 || item.DamageBonus > 0 || item.DefenseBonus > 0 {
			continue
		}
		isQuestItem := false
		for _, quest := range s.world.Rewards {
			isQuestItem = isQuestItem || quest.TargetItem == id
		}
		if !isQuestItem {
			t.Fatalf("obtainable item %s has no gameplay use", id)
		}
	}
}

func TestItemInfoLegacyMetadataAndScenery(t *testing.T) {
	w := &World{Start: "loc.room", Locations: map[string]Room{"loc.room": {ID: "loc.room", Items: []string{"item.legacy", "item.scenery"}}}, Items: map[string]Item{
		"item.legacy":  {ID: "item.legacy", Name: "Legacy", Obtainable: true},
		"item.scenery": {ID: "item.scenery", Name: "Scenery"},
	}}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), w)
	c := &client{username: "alice", roomID: w.Start, outbox: make(chan string, 2)}
	for _, id := range []string{"item.legacy", "item.scenery"} {
		line := infoCommandReply(t, s, c, "INFO "+id)
		var fields map[string]any
		if err := json.Unmarshal([]byte(line[3:]), &fields); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"id", "name", "obtainable", "description", "type", "usage"} {
			if _, present := fields[field]; !present {
				t.Fatalf("legacy descriptor missing %s: %s", field, line)
			}
		}
		if fields["usage"] == "" || fields["heal_value"] != nil || fields["damage_bonus"] != nil || fields["defense_bonus"] != nil {
			t.Fatalf("bad fallback descriptor: %s", line)
		}
	}
}

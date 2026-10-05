package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func TestWorldFullInventoryDetailsFitsProtocol(t *testing.T) {
	s, c := itemInfoState(t)
	count := 0
	for id, item := range s.world.Items {
		if item.Obtainable {
			s.itemLocations[id] = itemLocation{owner: c}
			count++
		}
	}
	line := infoCommandReply(t, s, c, "INVENTORY DETAILS")
	var inventory []itemView
	if len(line)+1 > maxLineBytes || len(line) < 3 || line[:3] != "OK " {
		t.Fatalf("default world's full inventory must fit a detailed reply: %s", line)
	}
	if err := json.Unmarshal([]byte(line[3:]), &inventory); err != nil || len(inventory) != count {
		t.Fatalf("incomplete full inventory: %v count=%d want=%d", err, len(inventory), count)
	}
}

func TestBountyRewardDefenseAfterDeliveryAndDrop(t *testing.T) {
	w, err := LoadWorld("../../data/world.json")
	if err != nil {
		t.Fatal(err)
	}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), w)
	c := &client{username: "alice", roomID: "loc.ruins_gate", hp: 75, outbox: make(chan string, 32)}
	s.transferItem(c, "TAKE", "Battered Shield", true)
	drain(c)
	if s.armorLocked(c) != 5 {
		t.Fatal("carried shield must provide 5 defense automatically")
	}
	def := w.Rewards["quest.bandit_bounty"]
	s.progressLocked(c.username).Defeated[def.DefeatNPC] = true
	s.talkReply(c, def.Giver, true, true)
	drain(c)
	if def.RewardItem != "item.captains_shield" || s.rewardClaims[def.ID] != c.username || s.itemLocations[def.RewardItem].owner != c || s.armorLocked(c) != 8 {
		t.Fatal("bounty should grant Captain's Shield and apply 8 defense without stacking")
	}
	s.transferItem(c, "DROP", def.RewardItem, true)
	drain(c)
	if s.armorLocked(c) != 5 {
		t.Fatal("dropping reward shield must restore the next strongest bonus")
	}
}

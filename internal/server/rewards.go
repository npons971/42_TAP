package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// RewardDefinition is validated at load time and drives quest interactions.
type RewardDefinition struct {
	ID          string `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Giver       string `json:"giver"`
	Type        string `json:"type"`
	TargetItem  string `json:"target_item"`
	TargetNPC   string `json:"target_npc"`
	DefeatNPC   string `json:"target_npc_defeat"`
	ReportNPC   string `json:"target_npc_report"`
	RewardItem  string `json:"reward_item"`
	Heal        int    `json:"reward_hp_heal"`
	Dialogue    string `json:"completion_dialogue"`
}

func (w *World) validateRewards() error {
	w.Rewards = make(map[string]RewardDefinition)
	usedItems, deliveryNPCs := map[string]string{}, map[string]string{}
	for id, raw := range w.Quests {
		if len(id) > 64 || !questIDPattern.MatchString(id) {
			return fmt.Errorf("invalid quest ID %q", id)
		}
		var def RewardDefinition
		if err := json.Unmarshal(raw, &def); err != nil {
			return fmt.Errorf("quest %q: %w", id, err)
		}
		def.ID = id
		if strings.TrimSpace(def.Title) == "" || strings.TrimSpace(def.Description) == "" {
			return fmt.Errorf("quest %q needs title and description", id)
		}
		if previous := deliveryNPCs[def.Giver]; previous != "" {
			return fmt.Errorf("quests %q and %q share a delivery NPC", previous, id)
		}
		deliveryNPCs[def.Giver] = id
		giver, ok := w.NPCs[def.Giver]
		if !ok || giver.Role != "quest_giver" {
			return fmt.Errorf("quest %q needs a known quest_giver", id)
		}
		item, ok := w.Items[def.RewardItem]
		if !ok || !item.Obtainable || item.InitialLocation == nil {
			return fmt.Errorf("quest %q reward must be an obtainable reserved item", id)
		}
		if previous := usedItems[def.RewardItem]; previous != "" {
			return fmt.Errorf("quests %q and %q share a unique reward", previous, id)
		}
		usedItems[def.RewardItem] = id
		if def.Heal < 0 || strings.TrimSpace(def.Dialogue) == "" {
			return fmt.Errorf("quest %q needs completion dialogue and nonnegative healing", id)
		}
		switch def.Type {
		case "fetch_and_deliver":
			target, ok := w.Items[def.TargetItem]
			if !ok || !target.Obtainable || target.Type != "quest" || target.InitialLocation != nil {
				return fmt.Errorf("quest %q target must be an obtainable room quest item", id)
			}
			if def.TargetNPC != def.Giver || def.TargetItem == def.RewardItem {
				return fmt.Errorf("quest %q delivery target must be its giver and differ from reward", id)
			}

		case "defeat_and_report":
			enemy, ok := w.NPCs[def.DefeatNPC]
			if !ok || enemy.Role != "enemy" || !enemy.Hostile || def.ReportNPC != def.Giver {
				return fmt.Errorf("quest %q needs a hostile enemy and report to its giver", id)
			}
		default:
			return fmt.Errorf("quest %q has unsupported type %q", id, def.Type)
		}
		w.Rewards[id] = def
	}
	return nil
}

// Caller holds mu. All checks precede queuing success and mutation.
func (s *Server) rewardAvailableLocked(def RewardDefinition) bool {
	known, ok := s.world.Rewards[def.ID]
	if !ok || known != def {
		return false
	}
	loc, exists := s.itemLocations[def.RewardItem]
	return exists && loc.reserved && loc.owner == nil && loc.roomID == "" && !loc.consumed && s.rewardClaims[def.ID] == ""
}

// Caller has validated eligibility, availability and queued the success reply.
// A claim is global to this server lifetime and survives the winner disconnecting.
func (s *Server) grantRewardLocked(c *client, def RewardDefinition) bool {
	if c == nil {
		return false
	}
	_, roomExists := s.world.Locations[c.roomID]
	if c.username == "" || !roomExists || !s.rewardAvailableLocked(def) || s.inventoryCountLocked(c) >= maxInventoryItems {
		return false
	}
	s.itemLocations[def.RewardItem] = itemLocation{owner: c}
	s.rewardClaims[def.ID] = c.username
	c.hp = healedHP(c.hp, def.Heal)
	return true
}

// Each giver owns one quest; TALK accepts it or validates its current objective.
func (s *Server) questForNPCLocked(npcID string) *RewardDefinition {
	ids := make([]string, 0, len(s.world.Rewards))
	for id := range s.world.Rewards {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		def := s.world.Rewards[id]
		if def.Giver == npcID {
			return &def
		}
	}
	return nil
}

// Subtract before adding to avoid integer overflow for large configured heals.
func healedHP(hp, heal int) int {
	if heal >= maxPlayerHP-hp {
		return maxPlayerHP
	}
	return hp + heal
}

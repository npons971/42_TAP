package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NPC is immutable catalogue data; mutable health is stored in Server.npcStates.
type NPC struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Dialogue    string `json:"dialogue"`
	Description string `json:"description,omitempty"`
	HP          int    `json:"hp,omitempty"`
	MaxHP       int    `json:"max_hp,omitempty"`
	AttackPower int    `json:"attack_power,omitempty"`
	Hostile     bool   `json:"hostile,omitempty"`
}

// Accept legacy text and ordered lines from Dev B. The wire reply stays a string.
func (n *NPC) UnmarshalJSON(data []byte) error {
	type plain NPC
	var decoded plain
	var fields struct {
		*plain
		Dialogue json.RawMessage `json:"dialogue"`
	}
	fields.plain = &decoded
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var text string
	if err := json.Unmarshal(fields.Dialogue, &text); err != nil {
		var lines []string
		if err := json.Unmarshal(fields.Dialogue, &lines); err != nil {
			return fmt.Errorf("NPC %q dialogue must be text or an array of text: %w", decoded.ID, err)
		}
		if len(lines) == 0 {
			return fmt.Errorf("NPC %q needs dialogue", decoded.ID)
		}
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				return fmt.Errorf("NPC %q needs dialogue in every line", decoded.ID)
			}
		}
		text = strings.Join(lines, "\n")
	}
	decoded.Dialogue = text
	*n = NPC(decoded)
	return nil
}

type npcView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Caller holds Server.mu to read the player's room consistently with MOVE.
func (s *Server) roomNPCsLocked(roomID string) []npcView {
	npcs := make([]npcView, 0)
	for _, id := range s.world.Locations[roomID].NPCs {
		if !s.npcAliveLocked(id) {
			continue
		}
		npc := s.world.NPCs[id]
		npcs = append(npcs, npcView{ID: npc.ID, Name: npc.Name, Role: npc.Role})
	}
	sort.Slice(npcs, func(i, j int) bool { return npcs[i].ID < npcs[j].ID })
	return npcs
}

func (s *Server) talk(c *client, target string, hasArgs bool) bool {
	return s.talkReply(c, target, hasArgs, false)
}

func (s *Server) talkReply(c *client, target string, hasArgs, details bool) bool {
	if !hasArgs || strings.TrimSpace(target) == "" {
		return sendError(c, "invalid_arguments", "Usage: TALK <npc_id or full display name>")
	}
	if target != strings.TrimSpace(target) || strings.ContainsRune(target, '\t') {
		return sendError(c, "invalid_arguments", "TALK target must not contain tabs or surrounding spaces")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "TALK")
	}
	roomID := c.roomID
	if c.target != "" {
		return sendError(c, "in_combat", "TALK is unavailable in combat; use FLEE")
	}
	match, ambiguous := s.resolveNPCLocked(roomID, target)
	if ambiguous {
		return sendError(c, "invalid_arguments", "TALK name matches multiple NPCs; use an NPC ID")
	}
	if match.ID == "" {
		return sendError(c, "target_not_found", fmt.Sprintf("TALK target %q is not in %s; use LOOK", shortVerb(target), roomID))
	}
	s.logger.Info("NPC interaction", "player", c.username, "npc", match.ID)
	def := s.questForNPCLocked(match.ID)
	delivering := false
	if def != nil {
		if winner := s.rewardClaims[def.ID]; winner != "" {
			if winner != c.username {
				return sendError(c, "reward_unavailable", "TALK reward for "+def.ID+" has already been awarded in this world")
			}
			match.Dialogue = "This delivery is already complete; its unique reward has been awarded."
		} else if s.objectiveLocked(c, *def) {
			if !s.rewardAvailableLocked(*def) {
				return sendError(c, "reward_unavailable", "TALK reward for "+def.ID+" is no longer in reserve")
			}
			if def.Type != "fetch_and_deliver" && s.inventoryCountLocked(c) >= maxInventoryItems {
				return sendError(c, "inventory_full", "TALK reward requires an inventory slot; use DROP and talk again")
			}
			delivering = true
			match.Dialogue = def.Dialogue
		}
	}
	payload, err := json.Marshal(struct {
		NPC      string `json:"npc"`
		Dialogue string `json:"dialogue"`
	}{NPC: match.ID, Dialogue: match.Dialogue})
	if err != nil {
		return sendError(c, "internal_error", "TALK response could not be encoded")
	}
	reply := "OK " + strings.Join(strings.Fields(match.Dialogue), " ")
	if details {
		reply = "OK " + string(payload)
	}
	if len(reply)+1 > maxLineBytes {
		return sendError(c, "response_too_large", "TALK response exceeds 4096-byte line limit")
	}
	if !queue(c.outbox, reply) {
		return false
	}
	if def != nil {
		s.progressLocked(c.username).Started[def.ID] = true
		s.logger.Info("quest started or discussed", "player", c.username, "quest", def.ID, "npc", match.ID)
	}
	if delivering {
		if def.Type == "fetch_and_deliver" {
			s.itemLocations[def.TargetItem] = itemLocation{consumed: true}
		}
		s.grantRewardLocked(c, *def)
		if def.Type == "fetch_and_deliver" {
			s.broadcastItemLocked(c.roomID, "DELIVER", c.username, def.TargetItem)
		}
		s.logger.Info("quest completed", "player", c.username, "quest", def.ID, "reward", def.RewardItem)
		s.broadcastItemLocked(c.roomID, "REWARD", c.username, def.RewardItem)
	}
	return true
}

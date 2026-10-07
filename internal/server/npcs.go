package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NPC is a static catalogue entry. Combat and quests will add runtime state.
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
		npc := s.world.NPCs[id]
		npcs = append(npcs, npcView{ID: npc.ID, Name: npc.Name, Role: npc.Role})
	}
	sort.Slice(npcs, func(i, j int) bool { return npcs[i].ID < npcs[j].ID })
	return npcs
}

func (s *Server) talk(c *client, target string, hasArgs bool) bool {
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
	var match NPC
	// An exact ID takes precedence over display names, as with TAKE and DROP.
	for _, id := range s.world.Locations[roomID].NPCs {
		if id == target {
			match = s.world.NPCs[id]
			break
		}
	}
	if match.ID == "" {
		for _, id := range s.world.Locations[roomID].NPCs {
			npc := s.world.NPCs[id]
			if strings.EqualFold(npc.Name, target) {
				if match.ID != "" {
					return sendError(c, "invalid_arguments", "TALK name matches multiple NPCs; use an NPC ID")
				}
				match = npc
			}
		}
	}
	if match.ID == "" {
		return sendError(c, "target_not_found", fmt.Sprintf("TALK target %q is not in %s; use LOOK", shortVerb(target), roomID))
	}
	def := s.deliveryLocked(c, match.ID)
	delivering := false
	if def != nil {
		if winner := s.rewardClaims[def.ID]; winner != "" {
			if winner != c.username {
				return sendError(c, "reward_unavailable", "TALK reward for "+def.ID+" has already been awarded in this world")
			}
			match.Dialogue = "This delivery is already complete; its unique reward has been awarded."
		} else if s.itemLocations[def.TargetItem].owner == c {
			if !s.rewardAvailableLocked(*def) {
				return sendError(c, "reward_unavailable", "TALK reward for "+def.ID+" is no longer in reserve")
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
	if len(payload)+4 > maxLineBytes {
		return sendError(c, "response_too_large", "TALK response exceeds 4096-byte line limit")
	}
	if !queue(c.outbox, "OK "+string(payload)) {
		return false
	}
	if delivering {
		s.itemLocations[def.TargetItem] = itemLocation{consumed: true}
		s.grantRewardLocked(c, *def)
		s.broadcastItemLocked(c.roomID, "DELIVER", c.username, def.TargetItem)
		s.broadcastItemLocked(c.roomID, "REWARD", c.username, def.RewardItem)
	}
	return true
}

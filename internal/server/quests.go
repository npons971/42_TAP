package server

import (
	"encoding/json"
	"sort"
	"strings"
)

type questProgress struct {
	Started  map[string]bool
	Defeated map[string]bool
}

func (s *Server) progressLocked(username string) *questProgress {
	p := s.questProgress[username]
	if p == nil {
		p = &questProgress{Started: map[string]bool{}, Defeated: map[string]bool{}}
		s.questProgress[username] = p
	}
	return p
}
func (s *Server) objectiveLocked(c *client, def RewardDefinition) bool {
	if def.Type == "fetch_and_deliver" {
		return s.itemLocations[def.TargetItem].owner == c
	}
	p := s.questProgress[c.username]
	return p != nil && p.Defeated[def.DefeatNPC]
}
func (s *Server) questStatusLocked(c *client, def RewardDefinition) string {
	if winner := s.rewardClaims[def.ID]; winner != "" {
		if winner == c.username {
			return "COMPLETED"
		}
		return "UNAVAILABLE"
	}
	p := s.questProgress[c.username]
	if p == nil || !p.Started[def.ID] {
		return "NOT_STARTED"
	}
	if s.objectiveLocked(c, def) {
		return "OBJECTIVES_MET"
	}
	return "IN_PROGRESS"
}
func (s *Server) quests(c *client, args []string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c.username == "" {
		return notAuthenticated(c, args[0])
	}
	if args[0] == "QUESTS" {
		if !validDetailsArgs(args) {
			return sendError(c, "invalid_arguments", "QUESTS takes no arguments")
		}
		if len(args) == 1 {
			return s.playerQuestsLocked(c)
		}
		type entry struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		}
		result := []entry{}
		for id, def := range s.world.Rewards {
			result = append(result, entry{id, def.Title, s.questStatusLocked(c, def)})
		}
		sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
		return queueJSON(c, "QUESTS", result)
	}
	if len(args) != 2 {
		return sendError(c, "invalid_arguments", "Usage: QUESTINFO <quest_id>")
	}
	def, ok := s.world.Rewards[args[1]]
	if !ok {
		return sendError(c, "quest_not_found", "QUESTINFO ID is unknown; use QUESTS DETAILS")
	}
	type objective struct {
		Type       string `json:"type"`
		TargetItem string `json:"target_item,omitempty"`
		TargetNPC  string `json:"target_npc,omitempty"`
		Completed  bool   `json:"completed"`
	}
	kind, target := "FETCH", def.TargetNPC
	if def.Type == "defeat_and_report" {
		kind, target = "DEFEAT", def.DefeatNPC
	}
	status := s.questStatusLocked(c, def)
	result := struct {
		ID          string    `json:"id"`
		Title       string    `json:"title"`
		Giver       string    `json:"giver"`
		Description string    `json:"description"`
		Status      string    `json:"status"`
		Objective   objective `json:"objective"`
		Reward      string    `json:"reward"`
	}{def.ID, def.Title, def.Giver, def.Description, status, objective{kind, def.TargetItem, target, status == "COMPLETED" || s.objectiveLocked(c, def)}, s.world.Items[def.RewardItem].Name}
	return queueJSON(c, "QUESTINFO", result)
}

func queueJSON(c *client, command string, value any) bool {
	payload, err := json.Marshal(value)
	if err != nil {
		return sendError(c, "internal_error", command+" response could not be encoded")
	}
	if len(payload)+4 > maxLineBytes {
		return sendError(c, "response_too_large", command+" response exceeds 4096-byte line limit")
	}
	return queue(c.outbox, "OK "+string(payload))
}

// QUEST requests a quest from a local NPC, using its ID or full display name.
func (s *Server) requestQuest(c *client, target string, hasArgs bool) bool {
	if !hasArgs || strings.TrimSpace(target) == "" || target != strings.TrimSpace(target) || strings.ContainsRune(target, '\t') {
		return sendError(c, "invalid_arguments", "Usage: QUEST <npc_id or full display name>")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "QUEST")
	}
	if c.target != "" {
		return sendError(c, "in_combat", "QUEST is unavailable in combat; use FLEE")
	}
	npc, ambiguous := s.resolveNPCLocked(c.roomID, target)
	if ambiguous {
		return sendError(c, "invalid_arguments", "QUEST name matches multiple NPCs; use an NPC ID")
	}
	if npc.ID == "" {
		return sendError(c, "target_not_found", "QUEST NPC is not in your room; use LOOK")
	}
	def := s.questForNPCLocked(npc.ID)
	if def == nil || !s.rewardAvailableLocked(*def) {
		return sendError(c, "no_quest_available", "QUEST NPC has no available quest or its unique reward has already been awarded")
	}
	status := "available"
	if p := s.questProgress[c.username]; p != nil && p.Started[def.ID] {
		status = "active"
	}
	result := struct {
		ID          string `json:"quest_id"`
		Description string `json:"description"`
		Reward      string `json:"reward"`
		Status      string `json:"status"`
	}{def.ID, def.Description, def.RewardItem, status}
	payload, err := json.Marshal(result)
	if err != nil {
		return sendError(c, "internal_error", "QUEST response could not be encoded")
	}
	if len(payload)+4 > maxLineBytes {
		return sendError(c, "response_too_large", "QUEST response exceeds 4096-byte line limit")
	}
	if !queue(c.outbox, "OK "+string(payload)) {
		return false
	}
	s.progressLocked(c.username).Started[def.ID] = true
	s.logger.Info("quest accepted", "player", c.username, "quest", def.ID, "npc", npc.ID)
	return true
}
func (s *Server) playerQuestsLocked(c *client) bool {
	type entry struct {
		ID       string `json:"quest_id"`
		Status   string `json:"status"`
		Progress string `json:"progress,omitempty"`
	}
	result := []entry{}
	for id, def := range s.world.Rewards {
		status := s.questStatusLocked(c, def)
		p := s.questProgress[c.username]
		if status != "COMPLETED" && (p == nil || !p.Started[id]) {
			continue
		}
		e := entry{ID: id, Status: "active", Progress: "0/1"}
		if status == "COMPLETED" {
			e.Status = "completed"
			e.Progress = ""
		} else if status == "UNAVAILABLE" {
			e.Status = "unavailable"
		} else if status == "OBJECTIVES_MET" {
			e.Progress = "1/1"
		}
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return queueJSON(c, "QUESTS", result)
}

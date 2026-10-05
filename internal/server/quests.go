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

type questStep struct {
	Type         string `json:"type"`
	Label        string `json:"label"`
	TargetID     string `json:"target_id"`
	LocationID   string `json:"location_id"`
	LocationName string `json:"location_name"`
	Completed    bool   `json:"completed"`
	Hint         string `json:"hint,omitempty"`
}

type questDetails struct {
	ID                string      `json:"id"`
	Title             string      `json:"title"`
	Description       string      `json:"description"`
	Status            string      `json:"status"`
	Giver             string      `json:"giver"`
	GiverName         string      `json:"giver_name"`
	GiverLocationID   string      `json:"giver_location_id"`
	GiverLocationName string      `json:"giver_location_name"`
	Reward            string      `json:"reward"`
	RewardHPHeal      int         `json:"reward_hp_heal,omitempty"`
	Steps             []questStep `json:"steps"`
}

// Placements and display names come from the validated world catalogue.
func (s *Server) questTargetRoomLocked(targetID string, item bool) string {
	for id, room := range s.world.Locations {
		targets := room.NPCs
		if item {
			targets = room.Items
		}
		for _, target := range targets {
			if target == targetID {
				return id
			}
		}
	}
	return ""
}

func (s *Server) questDetailsLocked(c *client, def RewardDefinition) questDetails {
	status := s.questStatusLocked(c, def)
	completed := status == "COMPLETED"
	giverName := s.world.NPCs[def.Giver].Name
	giverRoom := s.questTargetRoomLocked(def.Giver, false)
	first := questStep{Completed: completed || s.objectiveLocked(c, def)}
	last := questStep{TargetID: def.Giver, LocationID: giverRoom, LocationName: s.world.Locations[giverRoom].Name, Completed: completed}
	if def.Type == "fetch_and_deliver" {
		name := s.world.Items[def.TargetItem].Name
		first.Type, first.Label, first.TargetID = "FETCH", "Collect "+name, def.TargetItem
		first.LocationID = s.questTargetRoomLocked(def.TargetItem, true)
		location := s.itemLocations[def.TargetItem]
		if location.owner == nil && !location.reserved && !location.consumed && location.roomID != "" {
			first.LocationID = location.roomID
		}
		if !completed && location.owner != nil && location.owner != c {
			first.Hint = "Another player is carrying " + name + "; it is not currently on the ground."
		}
		if !first.Completed && location.consumed {
			first.Hint = name + " has already been delivered and is no longer available in this world."
		}
		last.Type, last.Label = "DELIVER", "Deliver "+name+" to "+giverName
	} else {
		name := s.world.NPCs[def.DefeatNPC].Name
		first.Type, first.Label, first.TargetID = "DEFEAT", "Defeat "+name+" yourself", def.DefeatNPC
		first.LocationID = s.questTargetRoomLocked(def.DefeatNPC, false)
		if state := s.npcStates[def.DefeatNPC]; !first.Completed && state != nil && state.HP <= 0 {
			first.Hint = name + " was defeated by another player; you must land the final blow yourself, and this enemy does not respawn."
		}
		last.Type, last.Label = "REPORT", "Report your victory to "+giverName
	}
	first.LocationName = s.world.Locations[first.LocationID].Name
	return questDetails{
		ID: def.ID, Title: def.Title, Description: def.Description, Status: status,
		Giver: def.Giver, GiverName: giverName, GiverLocationID: giverRoom, GiverLocationName: last.LocationName,
		Reward: s.world.Items[def.RewardItem].Name, RewardHPHeal: def.Heal, Steps: []questStep{first, last},
	}
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
		result := []questDetails{}
		for _, def := range s.world.Rewards {
			result = append(result, s.questDetailsLocked(c, def))
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
	details := s.questDetailsLocked(c, def)
	result := struct {
		questDetails
		Objective objective `json:"objective"`
	}{details, objective{kind, def.TargetItem, target, details.Steps[0].Completed}}
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

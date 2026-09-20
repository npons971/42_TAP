package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type npcState struct {
	HP       int
	Opponent *client
}
type combatView struct {
	TargetID    string `json:"target_id"`
	TargetName  string `json:"target_name"`
	TargetHP    int    `json:"target_hp"`
	TargetMaxHP int    `json:"target_max_hp"`
}
type combatResult struct {
	AttackerHP    int    `json:"attacker_hp"`
	Status        string `json:"status"`
	Player        string `json:"player"`
	Target        string `json:"target"`
	Damage        int    `json:"damage"`
	CounterDamage int    `json:"counter_damage"`
	PlayerHP      int    `json:"player_hp"`
	TargetHP      int    `json:"target_hp"`
	Outcome       string `json:"outcome"`
	Room          string `json:"room"`
}

func (s *Server) npcAliveLocked(id string) bool {
	state, ok := s.npcStates[id]
	return !ok || state.HP > 0
}
func (s *Server) resolveNPCLocked(roomID, target string) (NPC, bool) {
	ids := s.world.Locations[roomID].NPCs
	for _, id := range ids {
		if id == target && s.npcAliveLocked(id) {
			return s.world.NPCs[id], false
		}
	}
	var match NPC
	for _, id := range ids {
		if s.npcAliveLocked(id) && strings.EqualFold(s.world.NPCs[id].Name, target) {
			if match.ID != "" {
				return NPC{}, true
			}
			match = s.world.NPCs[id]
		}
	}
	return match, false
}
func (s *Server) releaseCombatLocked(c *client) {
	if state := s.npcStates[c.target]; state != nil && state.Opponent == c {
		state.Opponent = nil
	}
	c.target = ""
}
func (s *Server) armorLocked(c *client) int {
	best := 0
	for id, loc := range s.itemLocations {
		if loc.owner == c && s.world.Items[id].DefenseBonus > best {
			best = s.world.Items[id].DefenseBonus
		}
	}
	return best
}
func (s *Server) playerDamageLocked(c *client, hp int) int {
	bonus := 0
	for id, loc := range s.itemLocations {
		if loc.owner == c && s.world.Items[id].DamageBonus > bonus {
			bonus = s.world.Items[id].DamageBonus
		}
	}
	if hp <= 12 || bonus >= hp-12 {
		return hp
	}
	return 12 + bonus
}
func (s *Server) attack(c *client, target string, hasArgs bool) bool {
	if !hasArgs || strings.TrimSpace(target) == "" {
		return sendError(c, "invalid_arguments", "Usage: ATTACK <npc_id or full display name>")
	}
	if target != strings.TrimSpace(target) || strings.ContainsRune(target, '\t') {
		return sendError(c, "invalid_arguments", "ATTACK target must not contain tabs or surrounding spaces")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "ATTACK")
	}
	npc, ambiguous := s.resolveNPCLocked(c.roomID, target)
	if ambiguous {
		return sendError(c, "invalid_arguments", "ATTACK name matches multiple NPCs; use an NPC ID")
	}
	if npc.ID == "" {
		return sendError(c, "target_not_found", fmt.Sprintf("ATTACK target %q is not in %s; use LOOK", shortVerb(target), c.roomID))
	}
	state := s.npcStates[npc.ID]
	if npc.Role != "enemy" || !npc.Hostile || state == nil {
		return sendError(c, "cannot_attack", "ATTACK requires a living hostile enemy")
	}
	if c.target != "" && c.target != npc.ID {
		return sendError(c, "in_combat", "ATTACK cannot change target; use FLEE first")
	}
	if state.Opponent != nil && state.Opponent != c {
		return sendError(c, "target_busy", "ATTACK enemy is engaged by another player")
	}
	damage := s.playerDamageLocked(c, state.HP)
	remaining := state.HP - damage
	counter := 0
	if remaining > 0 {
		counter = max(1, npc.AttackPower-s.armorLocked(c))
	}
	return s.combatTurnLocked(c, npc, damage, counter, remaining, "ongoing", "")
}
func (s *Server) combatAction(c *client, args []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	verb := args[0]
	if c.username == "" {
		return notAuthenticated(c, verb)
	}
	if (verb == "DEFEND" && len(args) != 1) || (verb == "FLEE" && len(args) > 2) {
		return sendError(c, "invalid_arguments", "Usage: DEFEND or FLEE [direction]")
	}
	state := s.npcStates[c.target]
	if c.target == "" || state == nil || state.Opponent != c {
		return sendError(c, "not_in_combat", verb+" requires an active combat")
	}
	npc := s.world.NPCs[c.target]
	counter := max(1, npc.AttackPower-s.armorLocked(c))
	if verb == "DEFEND" {
		return s.combatTurnLocked(c, npc, 0, counter/2, state.HP, "defended", "")
	}
	exits := s.world.Locations[c.roomID].Exits
	direction := ""
	if len(args) == 2 {
		direction = args[1]
	} else {
		keys := []string{}
		for key := range exits {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			direction = keys[0]
		}
	}
	destination, ok := exits[direction]
	if !ok {
		return sendError(c, "invalid_direction", "FLEE requires an available exit; possible directions: "+availableDirections(s.world.Locations[c.roomID]))
	}
	if s.fleeRoll() < 70 {
		return s.combatTurnLocked(c, npc, 0, 0, state.HP, "fled", destination)
	}
	return s.combatTurnLocked(c, npc, 0, counter, state.HP, "flee_failed", "")
}

// Stage and encode the entire turn before any shared state mutation.
func (s *Server) combatTurnLocked(c *client, npc NPC, damage, counter, remaining int, outcome, destination string) bool {
	counter = min(counter, c.hp)
	hp := c.hp - counter
	if remaining == 0 {
		outcome = "victory"
	} else if hp == 0 {
		outcome = "respawn"
		hp = 30
		destination = s.world.Respawn
	}
	room := c.roomID
	if destination != "" {
		room = destination
	}
	wireStatus := outcome
	if outcome == "ongoing" || outcome == "defended" || outcome == "flee_failed" {
		wireStatus = "combat"
	}
	result := combatResult{Player: c.username, Target: npc.ID, Damage: damage, CounterDamage: counter, PlayerHP: hp, AttackerHP: hp, TargetHP: remaining, Outcome: outcome, Status: wireStatus, Room: room}
	payload, err := json.Marshal(result)
	if err != nil || len(payload)+len("EVT ROOM COMBAT ")+1 > maxLineBytes {
		return sendError(c, "response_too_large", "Combat result exceeds line limit")
	}
	if !queue(c.outbox, "OK "+string(payload)) {
		return false
	}
	oldRoom := c.roomID
	state := s.npcStates[npc.ID]
	state.HP = remaining
	c.hp = hp
	if outcome == "victory" || outcome == "respawn" || outcome == "fled" {
		s.releaseCombatLocked(c)
	} else {
		state.Opponent = c
		c.target = npc.ID
	}
	if outcome == "victory" {
		s.progressLocked(c.username).Defeated[npc.ID] = true
	}
	event := "EVT ROOM COMBAT " + string(payload)
	for _, other := range s.players {
		if other.roomID == oldRoom {
			queue(other.outbox, event)
		}
	}
	if destination != "" && destination != oldRoom {
		c.roomID = destination
		s.broadcastRoomLocked(oldRoom, c, "EVT ROOM PRESENCE LEAVE "+c.username)
		s.broadcastRoomLocked(destination, c, "EVT ROOM PRESENCE ENTER "+c.username)
	}
	s.logger.Info("combat turn", "player", c.username, "target", npc.ID, "result", result)
	if outcome == "victory" {
		s.logger.Info("quest objective", "player", c.username, "defeated", npc.ID)
	}
	return true
}

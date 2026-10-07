package server

import (
	"fmt"
	"strings"
)

func (s *Server) useItem(c *client, target string, hasArgs bool) bool {
	if !hasArgs || strings.TrimSpace(target) == "" {
		return sendError(c, "invalid_arguments", "Usage: USE <item_id or full display name>")
	}
	if target != strings.TrimSpace(target) || strings.ContainsRune(target, '\t') {
		return sendError(c, "invalid_arguments", "USE target must not contain tabs or surrounding spaces")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "USE")
	}
	if c.target != "" {
		return sendError(c, "in_combat", "USE"+" is unavailable in combat; use FLEE")
	}
	id, ambiguous := s.resolveItemLocked(c, "DROP", target)
	if ambiguous {
		return sendError(c, "invalid_arguments", "USE name matches multiple items; use an item ID")
	}
	if id == "" {
		return sendError(c, "not_in_inventory", fmt.Sprintf("USE target %q is not in your inventory; use INVENTORY", shortVerb(target)))
	}
	item := s.world.Items[id]
	if item.Type != "consumable" || item.HealValue <= 0 {
		return sendError(c, "item_not_usable", "Item "+id+" cannot be consumed with USE")
	}
	if c.hp >= maxPlayerHP {
		return sendError(c, "health_full", "USE cannot restore HP: health is already full; item kept")
	}
	hp := healedHP(c.hp, item.HealValue)
	if !queue(c.outbox, fmt.Sprintf("OK used=%s hp=%d/%d", id, hp, maxPlayerHP)) {
		return false
	}
	s.itemLocations[id] = itemLocation{consumed: true}
	c.hp = hp
	s.logger.Info("item consumed", "player", c.username, "item", id, "hp", hp)
	s.broadcastItemLocked(c.roomID, "USE", c.username, id)
	return true
}

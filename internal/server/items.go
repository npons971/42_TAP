package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type itemLocation struct {
	roomID   string
	owner    *client
	reserved bool
	consumed bool
}

type itemView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Obtainable   bool   `json:"obtainable"`
	Description  string `json:"description"`
	Type         string `json:"type"`
	HealValue    int    `json:"heal_value,omitempty"`
	DamageBonus  int    `json:"damage_bonus,omitempty"`
	DefenseBonus int    `json:"defense_bonus,omitempty"`
	Usage        string `json:"usage"`
}

type roomView struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"`
}

type inventoryItem = itemView

// Item inspection only describes accessible instances; catalogue-only fields
// such as reward reservations must never be sent to a player.
func (s *Server) itemViewLocked(id string) itemView {
	item := s.world.Items[id]
	usage := []string{}
	if item.Type == "consumable" && item.HealValue > 0 {
		usage = append(usage, fmt.Sprintf("Use USE %s outside combat to restore up to %d HP; the item is consumed. It is kept if your health is already full.", id, item.HealValue))
	}
	if item.DamageBonus > 0 || item.DefenseBonus > 0 {
		usage = append(usage, "Bonuses apply automatically while carried in your inventory. Only the highest damage bonus and the highest defense bonus apply independently; bonuses do not stack. No USE or equip command is needed.")
	}
	quests := make([]string, 0)
	for questID, def := range s.world.Rewards {
		if def.Type == "fetch_and_deliver" && def.TargetItem == id {
			quests = append(quests, questID)
		}
	}
	sort.Strings(quests)
	for _, questID := range quests {
		def := s.world.Rewards[questID]
		roomID := s.questTargetRoomLocked(def.Giver, false)
		usage = append(usage, fmt.Sprintf("Carry this quest item to %s in %s and use TALK %s to deliver it for %s; delivery consumes the item.", s.world.NPCs[def.Giver].Name, s.world.Locations[roomID].Name, def.Giver, def.Title))
	}
	if len(usage) == 0 {
		if !item.Obtainable {
			usage = append(usage, "Scenery: inspect it with INFO; it cannot be picked up or consumed.")
		} else {
			usage = append(usage, "This item has no direct usable effect or automatic bonus. Inspect it with INFO.")
		}
	}
	return itemView{
		ID: id, Name: item.Name, Obtainable: item.Obtainable, Description: item.Description,
		Type: item.Type, HealValue: item.HealValue, DamageBonus: item.DamageBonus,
		DefenseBonus: item.DefenseBonus, Usage: strings.Join(usage, " "),
	}
}

func (s *Server) itemInfo(c *client, target string, hasArgs bool) bool {
	if !hasArgs || strings.TrimSpace(target) == "" {
		return sendError(c, "invalid_arguments", "Usage: INFO <item_id or full display name>")
	}
	if target != strings.TrimSpace(target) || strings.ContainsRune(target, '\t') {
		return sendError(c, "invalid_arguments", "INFO target must not contain tabs or surrounding spaces")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c.username == "" {
		return notAuthenticated(c, "INFO")
	}
	id, ambiguous := s.resolveItemLocked(c, "INFO", target)
	if ambiguous {
		return sendError(c, "invalid_arguments", "INFO name matches multiple items; use an item ID")
	}
	if id == "" {
		return sendError(c, "item_not_found", fmt.Sprintf("INFO target %q is not in your inventory or current room; use INVENTORY or LOOK", shortVerb(target)))
	}
	return queueJSON(c, "INFO", s.itemViewLocked(id))
}

func (s *Server) transferItem(c *client, verb, target string, hasArgs bool) bool {
	if !hasArgs || strings.TrimSpace(target) == "" {
		return sendError(c, "invalid_arguments", "Usage: "+verb+" <item_id or full display name>")
	}
	if target != strings.TrimSpace(target) || strings.ContainsRune(target, '\t') {
		return sendError(c, "invalid_arguments", verb+" target must not contain tabs or surrounding spaces")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, verb)
	}
	if c.target != "" {
		return sendError(c, "in_combat", verb+" is unavailable in combat; use FLEE")
	}
	id, ambiguous := s.resolveItemLocked(c, verb, target)
	if ambiguous {
		return sendError(c, "invalid_arguments", verb+" name matches multiple items; use an item ID")
	}
	if id == "" {
		if verb == "TAKE" {
			return sendError(c, "item_not_found", fmt.Sprintf("TAKE target %q is not in %s; use LOOK", shortVerb(target), c.roomID))
		}
		return sendError(c, "not_in_inventory", fmt.Sprintf("DROP target %q is not in your inventory; use INVENTORY", shortVerb(target)))
	}
	if verb == "TAKE" && !s.world.Items[id].Obtainable {
		return sendError(c, "item_not_obtainable", "Item "+id+" cannot be picked up with TAKE")
	}
	if verb == "TAKE" {
		count := s.inventoryCountLocked(c)
		if count >= maxInventoryItems {
			return sendError(c, "inventory_full", "TAKE inventory limit reached; use DROP before taking another item")
		}
	}
	reply := "OK taken=" + id
	if verb == "DROP" {
		reply = "OK dropped=" + id
	}
	if !queue(c.outbox, reply) {
		return false
	}
	if verb == "TAKE" {
		s.itemLocations[id] = itemLocation{owner: c}
	} else {
		s.itemLocations[id] = itemLocation{roomID: c.roomID}
	}
	s.broadcastItemLocked(c.roomID, verb, c.username, id)
	s.logger.Info("item moved", "action", verb, "item", id, "player", c.username, "room", c.roomID)
	return true
}

// All resolution and transfers happen under the same state lock, so concurrent
// TAKE requests cannot acquire the same instance.
func (s *Server) resolveItemLocked(c *client, verb, target string) (string, bool) {
	inContext := func(location itemLocation) bool {
		if location.reserved || location.consumed {
			return false
		}
		if verb == "DROP" {
			return location.owner == c
		}
		if verb == "INFO" && location.owner == c {
			return true
		}
		return location.owner == nil && location.roomID == c.roomID
	}
	if location, exists := s.itemLocations[target]; exists {
		if inContext(location) {
			return target, false
		}
		return "", false
	}
	match := ""
	for id, location := range s.itemLocations {
		if inContext(location) && strings.EqualFold(s.world.Items[id].Name, target) {
			if match != "" {
				return "", true
			}
			match = id
		}
	}
	return match, false
}

func (s *Server) inventory(c *client, args []string) bool {
	if !validDetailsArgs(args) {
		return sendError(c, "invalid_arguments", "INVENTORY takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return notAuthenticated(c, "INVENTORY")
	}
	items := make([]inventoryItem, 0)
	for id, location := range s.itemLocations {
		if !location.reserved && !location.consumed && location.owner == c {
			items = append(items, s.itemViewLocked(id))
		}
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	var data any = items
	if len(args) == 1 {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		data = ids
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return sendError(c, "internal_error", "INVENTORY response could not be encoded")
	}
	if len(payload)+4 > maxLineBytes {
		return sendError(c, "response_too_large", "INVENTORY response exceeds 4096-byte line limit")
	}
	return queue(c.outbox, "OK "+string(payload))
}

func (s *Server) roomItemsLocked(roomID string) []itemView {
	items := make([]itemView, 0)
	for id, location := range s.itemLocations {
		if !location.reserved && !location.consumed && location.owner == nil && location.roomID == roomID {
			items = append(items, s.itemViewLocked(id))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (s *Server) broadcastItemLocked(roomID, action, username, itemID string) {
	event := "EVT ROOM ITEM " + action + " " + username + " " + itemID
	for _, other := range s.players {
		if other.roomID == roomID && !queue(other.outbox, event) {
			s.logger.Warn("client outbox full", "player", other.username, "event", event)
		}
	}
}

// Player state disappears on disconnect; return held instances to the current
// room so no unique object becomes lost or is recreated on the next login.
func (s *Server) dropInventoryLocked(c *client) {
	ids := make([]string, 0)
	for id, location := range s.itemLocations {
		if location.owner == c {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		s.itemLocations[id] = itemLocation{roomID: c.roomID}
		s.broadcastItemLocked(c.roomID, "DROP", c.username, id)
		s.logger.Info("item returned on disconnect", "item", id, "player", c.username, "room", c.roomID)
	}
}

func (s *Server) inventoryCountLocked(c *client) int {
	count := 0
	for _, location := range s.itemLocations {
		if location.owner == c {
			count++
		}
	}
	return count
}

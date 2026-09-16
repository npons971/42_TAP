package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type itemLocation struct {
	roomID string
	owner  *client
}

type roomView struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"`
}

type inventoryItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
		if verb == "DROP" {
			return location.owner == c
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
	if len(args) != 1 {
		return sendError(c, "invalid_arguments", "INVENTORY takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return notAuthenticated(c, "INVENTORY")
	}
	items := make([]inventoryItem, 0)
	for id, location := range s.itemLocations {
		if location.owner == c {
			items = append(items, inventoryItem{ID: id, Name: s.world.Items[id].Name})
		}
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	payload, err := json.Marshal(items)
	if err != nil {
		return sendError(c, "internal_error", "INVENTORY response could not be encoded")
	}
	if len(payload)+4 > maxLineBytes {
		return sendError(c, "response_too_large", "INVENTORY response exceeds 4096-byte line limit")
	}
	return queue(c.outbox, "OK "+string(payload))
}

func (s *Server) roomItemsLocked(roomID string) []Item {
	items := make([]Item, 0)
	for id, location := range s.itemLocations {
		if location.owner == nil && location.roomID == roomID {
			items = append(items, s.world.Items[id])
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

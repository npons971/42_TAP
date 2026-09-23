package server

import (
	"fmt"
	"strings"
)

// Numeric RFC codes retain a stable token followed by a command-specific hint.
var errorSpecs = map[string]string{
	"username_taken": "201 NAME_IN_USE", "invalid_direction": "301 NO_EXIT",
	"group_required": "401 NOT_IN_GROUP", "already_in_group": "402 ALREADY_IN_GROUP",
	"item_not_found": "404 ITEM_NOT_FOUND", "not_in_inventory": "404 ITEM_NOT_IN_INVENTORY",
	"target_not_found": "404 NPC_NOT_FOUND", "cannot_attack": "405 NPC_NOT_HOSTILE",
	"reward_unavailable": "406 NO_QUEST_AVAILABLE", "no_quest_available": "406 NO_QUEST_AVAILABLE",
	"inventory_full":    "409 INVENTORY_FULL",
	"invalid_arguments": "400 INVALID_ARGUMENTS", "not_authenticated": "403 NOT_AUTHENTICATED",
	"already_authenticated": "409 ALREADY_AUTHENTICATED", "unknown_command": "400 UNKNOWN_COMMAND",
	"in_combat": "409 IN_COMBAT", "not_in_combat": "409 NOT_IN_COMBAT", "target_busy": "409 TARGET_BUSY",
	"item_not_obtainable": "405 ITEM_NOT_OBTAINABLE", "item_not_usable": "405 ITEM_NOT_USABLE",
	"health_full": "409 HEALTH_FULL", "quest_not_found": "404 QUEST_NOT_FOUND",
	"group_not_found": "404 GROUP_NOT_FOUND", "player_not_found": "404 PLAYER_NOT_FOUND",
	"response_too_large": "413 RESPONSE_TOO_LARGE", "rate_limited": "429 RATE_LIMITED",
	"group_exists": "409 GROUP_EXISTS", "internal_error": "500 INTERNAL_ERROR",
}

func errorReply(code, message string) string {
	spec, ok := errorSpecs[code]
	if !ok {
		spec = "500 INTERNAL_ERROR"
	}
	return "ERR " + spec + " " + message
}
func validDetailsArgs(args []string) bool {
	return len(args) == 1 || len(args) == 2 && strings.EqualFold(args[1], "DETAILS")
}

// Caller holds mu; updates follow the command reply and presence events.
func (s *Server) broadcastStatsLocked() {
	event := fmt.Sprintf("EVT STATS players=%d", len(s.players))
	for _, c := range s.players {
		if !queue(c.outbox, event) {
			s.logger.Warn("client outbox full", "player", c.username, "event", event)
		}
	}
}

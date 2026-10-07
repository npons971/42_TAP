package server

import (
	"fmt"
	"sort"
	"strings"
)

func (s *Server) group(c *client, args []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "GROUP")
	}
	if len(args) < 2 {
		return sendError(c, "invalid_arguments", "Usage: GROUP CREATE, GROUP INVITE <username>, GROUP JOIN <leader-name> or GROUP LEAVE")
	}
	action := strings.ToUpper(args[1])
	switch action {
	case "CREATE":
		if len(args) != 2 {
			return sendError(c, "invalid_arguments", "GROUP CREATE takes no arguments")
		}
		if c.group != "" {
			return sendError(c, "already_in_group", "GROUP requires leaving your current group first")
		}
		id := fmt.Sprintf("group.%d", s.nextGroup+1)
		if !queue(c.outbox, "OK group="+id) {
			return false
		}
		s.nextGroup++
		s.groups[id] = map[*client]bool{c: true}
		s.groupLeaders[id] = c.username
		c.group = id
		s.logger.Info("group created", "player", c.username, "group", id)
		return true
	case "INVITE":
		if len(args) != 3 {
			return sendError(c, "invalid_arguments", "Usage: GROUP INVITE <username>")
		}
		if c.group == "" {
			return sendError(c, "group_required", "GROUP INVITE requires group membership")
		}
		target := s.players[args[2]]
		if target == nil {
			return sendError(c, "player_not_found", "GROUP INVITE player is not connected")
		}
		if target.group != "" {
			return sendError(c, "already_in_group", "GROUP INVITE player already belongs to a group")
		}
		if len(target.outbox) == cap(target.outbox) {
			return sendError(c, "target_busy", "GROUP INVITE recipient cannot receive the invitation")
		}
		if !queue(c.outbox, "OK") {
			return false
		}
		queue(target.outbox, "EVT GROUP INVITE "+s.groupLeaders[c.group])
		return true
	case "JOIN":
		if len(args) != 3 {
			return sendError(c, "invalid_arguments", "Usage: GROUP JOIN <leader-name>")
		}
		if c.group != "" {
			return sendError(c, "already_in_group", "GROUP requires leaving your current group first")
		}
		leader := s.players[args[2]]
		if leader == nil || leader.group == "" || s.groupLeaders[leader.group] != leader.username {
			return sendError(c, "group_not_found", "GROUP JOIN requires a connected group leader; use the name from GROUP INVITE")
		}
		id := leader.group
		if !queue(c.outbox, "OK group="+id) {
			return false
		}
		s.groups[id][c] = true
		c.group = id
		s.broadcastGroupLocked(id, "EVT GROUP JOIN "+c.username)
		s.logger.Info("group joined", "player", c.username, "group", id)
		return true
	case "LEAVE":
		if len(args) != 2 {
			return sendError(c, "invalid_arguments", "GROUP LEAVE takes no group name")
		}
		if c.group == "" {
			return sendError(c, "group_required", "GROUP LEAVE requires group membership")
		}
		if !queue(c.outbox, "OK") {
			return false
		}
		// The departing player receives the event too, after its reply.
		s.broadcastGroupLocked(c.group, "EVT GROUP LEAVE "+c.username)
		s.removeGroupMemberLocked(c)
		return true
	default:
		return sendError(c, "invalid_arguments", "GROUP action must be CREATE, INVITE, JOIN or LEAVE")
	}
}
func (s *Server) broadcastGroupLocked(id, event string) {
	for member := range s.groups[id] {
		if !queue(member.outbox, event) {
			s.logger.Warn("client outbox full", "player", member.username, "event", event)
		}
	}
}

// Disconnect removes the departing member before notifying remaining members.
func (s *Server) leaveGroupLocked(c *client) {
	if c.group == "" {
		return
	}
	id := c.group
	s.removeGroupMemberLocked(c)
	s.broadcastGroupLocked(id, "EVT GROUP LEAVE "+c.username)
}
func (s *Server) removeGroupMemberLocked(c *client) {
	id := c.group
	delete(s.groups[id], c)
	c.group = ""
	if len(s.groups[id]) == 0 {
		delete(s.groups, id)
		delete(s.groupLeaders, id)
	} else if s.groupLeaders[id] == c.username {
		names := make([]string, 0, len(s.groups[id]))
		for m := range s.groups[id] {
			names = append(names, m.username)
		}
		sort.Strings(names)
		s.groupLeaders[id] = names[0]
		s.broadcastGroupLocked(id, "EVT GROUP LEADER "+names[0])
	}
	s.logger.Info("group left", "player", c.username, "group", id)
}

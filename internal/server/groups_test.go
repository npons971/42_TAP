package server

import (
	"strings"
	"sync"
	"testing"
)

func TestGroupsChatAndDisconnectTCP(t *testing.T) {
	s, addr, stop := lifecycleServer(t)
	defer stop()
	a, ar := dialTestClient(t, addr)
	b, br := dialTestClient(t, addr)
	outsider, orr := dialTestClient(t, addr)
	sendCommand(t, a, "CONNECT alice")
	expectLine(t, ar, "OK connected")
	sendCommand(t, b, "CONNECT bob")
	expectLine(t, br, "OK connected")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER bob")
	sendCommand(t, outsider, "CONNECT outsider")
	expectLine(t, orr, "OK connected")
	expectLine(t, ar, "EVT ROOM PRESENCE ENTER outsider")
	expectLine(t, br, "EVT ROOM PRESENCE ENTER outsider")
	sendCommand(t, a, "GROUP CREATE")
	expectLine(t, ar, "OK group=group.1")
	sendCommand(t, a, "GROUP INVITE bob")
	expectLine(t, ar, "OK")
	expectLine(t, br, "EVT GROUP INVITE alice")
	sendCommand(t, b, "GROUP JOIN alice")
	expectLine(t, br, "OK group=group.1")
	expectLine(t, br, "EVT GROUP JOIN bob")
	expectLine(t, ar, "EVT GROUP JOIN bob")
	sendCommand(t, a, "GROUP JOIN alice")
	expectLine(t, ar, "ERR 402 ALREADY_IN_GROUP GROUP requires leaving your current group first")
	sendCommand(t, b, "MOVE north")
	expectLine(t, br, "OK room=loc.garden")
	expectLine(t, ar, "EVT ROOM PRESENCE LEAVE bob")
	expectLine(t, orr, "EVT ROOM PRESENCE LEAVE bob")
	sendCommand(t, a, "CHAT GROUP meet here")
	expectLine(t, ar, "OK")
	expectLine(t, ar, "EVT GROUP CHAT alice meet here")
	expectLine(t, br, "EVT GROUP CHAT alice meet here")
	sendCommand(t, outsider, "WHO")
	expectLine(t, orr, "OK players=3")
	// Leader departure elects bob and preserves the group across rooms.
	sendCommand(t, a, "GROUP LEAVE")
	expectLine(t, ar, "OK")
	expectLine(t, ar, "EVT GROUP LEAVE alice")
	expectLine(t, br, "EVT GROUP LEAVE alice")
	expectLine(t, br, "EVT GROUP LEADER bob")
	sendCommand(t, outsider, "GROUP JOIN alice")
	expectLine(t, orr, "ERR 404 GROUP_NOT_FOUND GROUP JOIN requires a connected group leader; use the name from GROUP INVITE")
	sendCommand(t, a, "GROUP JOIN bob")
	expectLine(t, ar, "OK group=group.1")
	expectLine(t, ar, "EVT GROUP JOIN alice")
	expectLine(t, br, "EVT GROUP JOIN alice")
	_ = a.Close()
	expectLine(t, br, "EVT GROUP LEAVE alice")
	expectLine(t, orr, "EVT ROOM PRESENCE LEAVE alice")
	sendCommand(t, b, "GROUP LEAVE")
	expectLine(t, br, "OK")
	expectLine(t, br, "EVT GROUP LEAVE bob")
	s.mu.RLock()
	if len(s.groups) != 0 || len(s.groupLeaders) != 0 {
		t.Error("empty group retained")
	}
	s.mu.RUnlock()
}

func TestConcurrentGroupCreationAndFullQueue(t *testing.T) {
	s, a := combatClient(t)
	b := &client{username: "bob", outbox: make(chan string, 16)}
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for _, c := range []*client{a, b} {
		wg.Add(1)
		go func(c *client) { defer wg.Done(); <-gate; s.group(c, []string{"GROUP", "CREATE"}) }(c)
	}
	close(gate)
	wg.Wait()
	if len(s.groups) != 2 || a.group == b.group {
		t.Fatal("group IDs were not unique")
	}
	for _, c := range []*client{a, b} {
		if got := <-c.outbox; !strings.HasPrefix(got, "OK group=group.") {
			t.Fatal(got)
		}
		drain(c)
	}
	id := a.group
	a.outbox = make(chan string, 1)
	a.outbox <- "blocked"
	if s.group(a, []string{"GROUP", "LEAVE"}) || a.group != id {
		t.Fatal("full queue removed membership")
	}
	drain(a)
	s.removeGroupMemberLocked(b)
	drain(b)
	s.players[b.username] = b
	b.outbox = make(chan string, 1)
	b.outbox <- "blocked"
	s.group(a, []string{"GROUP", "INVITE", "bob"})
	if got := <-a.outbox; !strings.HasPrefix(got, "ERR 409 TARGET_BUSY ") {
		t.Fatal(got)
	}
}

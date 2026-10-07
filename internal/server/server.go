package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxLineBytes      = 4096 // Includes LF, as documented in the protocol proposal.
	outboxSize        = 128
	writeTimeout      = 5 * time.Second
	maxPlayerHP       = 100
	maxConnections    = 128
	maxInventoryItems = 32
)

var (
	errLineTooLong   = errors.New("line too long")
	errInvalidUTF8   = errors.New("invalid UTF-8")
	errEmptyLine     = errors.New("empty line")
	errControlInLine = errors.New("control character in line")
	errCRInLine      = errors.New("carriage return in line")
	usernamePattern  = regexp.MustCompile(`^[\p{Ll}][\p{Ll}\p{Nd}_]{2,19}$`)
)

type client struct {
	conn        net.Conn
	outbox      chan string
	writerDone  chan struct{}
	connectedAt time.Time
	username    string // Protected by Server.mu.
	roomID      string // Protected by Server.mu.
	hp          int    // Protected by Server.mu.
	target      string
	group       string
}

// Server owns the connected-player state. World data is immutable after load.
type Server struct {
	logger            *slog.Logger
	world             *World
	mu                sync.RWMutex
	clients           map[net.Conn]*client
	players           map[string]*client
	itemLocations     map[string]itemLocation // One authoritative location per instance.
	rewardClaims      map[string]string       // Quest ID -> winning username, protected by mu.
	groups            map[string]map[*client]bool
	groupLeaders      map[string]string
	nextGroup         uint64
	npcStates         map[string]*npcState
	questProgress     map[string]*questProgress
	connectionHistory map[string]connectionWindow
	fleeRoll          func() int
	wg                sync.WaitGroup
}

func New(logger *slog.Logger, world *World) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		logger: logger, world: world,
		clients:       make(map[net.Conn]*client),
		players:       make(map[string]*client),
		itemLocations: make(map[string]itemLocation),
		rewardClaims:  make(map[string]string),
		groups:        map[string]map[*client]bool{}, npcStates: map[string]*npcState{}, questProgress: map[string]*questProgress{}, fleeRoll: func() int { return rand.Intn(100) },
		connectionHistory: map[string]connectionWindow{}, groupLeaders: map[string]string{},
	}
	for id, npc := range world.NPCs {
		if npc.Role == "enemy" {
			s.npcStates[id] = &npcState{HP: npc.HP}
		}
	}
	for id, item := range world.Items {
		if item.InitialLocation != nil && item.InitialLocation.Kind == "reserve" {
			s.itemLocations[id] = itemLocation{reserved: true}
		}
	}
	for roomID, room := range world.Locations {
		for _, itemID := range room.Items {
			s.itemLocations[itemID] = itemLocation{roomID: roomID}
		}
	}
	return s
}

// Serve stops accepting and closes all active sessions when ctx is cancelled.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	stopClosing := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-stopClosing:
		}
	}()
	defer close(stopClosing)
	defer func() {
		s.mu.Lock()
		for conn := range s.clients {
			_ = conn.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		c := &client{conn: conn, outbox: make(chan string, outboxSize), writerDone: make(chan struct{}), connectedAt: time.Now()}
		s.mu.Lock()
		if len(s.clients) >= maxConnections {
			s.mu.Unlock()
			_ = conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
			_, _ = io.WriteString(conn, "OK hello proto=1\nERR 503 CONNECTION_LIMIT Server connection limit reached\n")
			_ = conn.Close()
			continue
		}
		s.recordConnectionLocked(conn.RemoteAddr(), time.Now())
		s.clients[conn] = c
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serveClient(c)
	}
}

func (s *Server) serveClient(c *client) {
	defer s.wg.Done()
	go s.writeClient(c)
	defer s.disconnect(c)

	s.logger.Info("client connected", "remote_addr", c.conn.RemoteAddr().String())
	if !queue(c.outbox, "OK hello proto=1") {
		return
	}

	reader := bufio.NewReaderSize(c.conn, maxLineBytes)
	var limiter tokenBucket
	for {
		line, err := readLine(reader)
		if err == nil {
			username, remote := s.identity(c)
			verb, params, _ := strings.Cut(line, " ")
			s.logger.Info("command received", "event", "COMMAND_RECEIVED", "player", username, "remote_addr", remote, "command", verb, "params", params)
		}
		if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) && !limiter.allow(time.Now()) {
			username, remote := s.identity(c)
			s.logger.Warn("command flood", "event", "ABUSE_FLOOD", "player", username, "remote_addr", remote)
			sendError(c, "rate_limited", "Command flood detected; connection closing")
			return
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			if !errors.Is(err, errLineTooLong) && !errors.Is(err, errInvalidUTF8) && !errors.Is(err, errEmptyLine) && !errors.Is(err, errCRInLine) && !errors.Is(err, errControlInLine) {
				s.logger.Warn("client read failed", "remote_addr", c.conn.RemoteAddr().String(), "error", err)
				return
			}
			s.logger.Warn("invalid request", "remote_addr", c.conn.RemoteAddr().String(), "error", err)
			reply := "Empty command"
			if errors.Is(err, errLineTooLong) {
				reply = "Line exceeds 4096 bytes"
			} else if errors.Is(err, errInvalidUTF8) {
				reply = "Line must be valid UTF-8"
			} else if errors.Is(err, errControlInLine) {
				reply = "Control characters are forbidden"
			} else if errors.Is(err, errCRInLine) {
				reply = "Use LF without CR"
			}
			if !sendError(c, "invalid_arguments", reply) {
				return
			}
			continue
		}
		if !s.handleCommand(c, line) {
			return
		}
	}
}

// disconnect unregisters the player before broadcasting departure. Removing
// the client while holding mu also makes closing its outbox safe for broadcasts.
func (s *Server) disconnect(c *client) {
	s.mu.Lock()
	username := c.username
	if username != "" {
		delete(s.players, username)
		s.releaseCombatLocked(c)
		s.leaveGroupLocked(c)
		s.dropInventoryLocked(c)
		s.broadcastRoomLocked(c.roomID, c, "EVT ROOM PRESENCE LEAVE "+username)
		s.broadcastStatsLocked()
	}
	delete(s.clients, c.conn)
	s.mu.Unlock()

	close(c.outbox)
	<-c.writerDone
	_ = c.conn.Close()
	s.logger.Info("client disconnected", "remote_addr", c.conn.RemoteAddr().String(), "player", username, "duration_ms", time.Since(c.connectedAt).Milliseconds())
}

func (s *Server) handleCommand(c *client, line string) bool {
	verb, rest, hasArgs := strings.Cut(line, " ")
	verb = strings.ToUpper(verb)
	if verb == "CHAT" {
		return s.chat(c, rest, hasArgs)
	}
	if verb == "TAKE" || verb == "DROP" {
		return s.transferItem(c, verb, rest, hasArgs)
	}
	if verb == "ATTACK" {
		return s.attack(c, rest, hasArgs)
	}
	if verb == "USE" {
		return s.useItem(c, rest, hasArgs)
	}
	if verb == "TALK" || verb == "TALKJSON" {
		return s.talkReply(c, rest, hasArgs, verb == "TALKJSON")
	}
	if verb == "QUEST" {
		return s.requestQuest(c, rest, hasArgs)
	}
	args := strings.Split(line, " ")
	args[0] = verb
	if strings.ContainsRune(line, '\t') || args[0] == "" {
		return sendError(c, "invalid_arguments", "Command must start with a verb and use spaces, not tabs")
	}
	for _, arg := range args {
		if arg == "" {
			return sendError(c, "invalid_arguments", args[0]+" must use one space between fixed arguments")
		}
	}
	switch args[0] {
	case "CONNECT":
		return s.connect(c, args)
	case "LOOK":
		return s.look(c, args)
	case "MOVE":
		return s.move(c, args)
	case "WHO":
		return s.who(c, args)
	case "STATUS":
		return s.status(c, args)
	case "INVENTORY":
		return s.inventory(c, args)
	case "GROUP":
		return s.group(c, args)
	case "QUESTINFO", "QUESTS":
		return s.quests(c, args)
	case "DEFEND", "FLEE":
		return s.combatAction(c, args)
	case "QUIT":
		if len(args) != 1 {
			return sendError(c, "invalid_arguments", "QUIT takes no arguments")
		}
		queue(c.outbox, "OK bye")
		return false
	default:
		name := shortVerb(args[0])

		return sendError(c, "unknown_command", fmt.Sprintf("Unknown command %q", name))
	}
}

func sendError(c *client, code, message string) bool {
	return queue(c.outbox, errorReply(code, message))
}

func notAuthenticated(c *client, verb string) bool {
	return sendError(c, "not_authenticated", "Send CONNECT <username> before "+verb)
}

func shortVerb(verb string) string {
	runes := []rune(verb)
	if len(runes) > 24 {
		return string(runes[:24]) + "…"
	}
	return verb
}

func (s *Server) connect(c *client, args []string) bool {
	if len(args) != 2 {
		return sendError(c, "invalid_arguments", "Usage: CONNECT <username>")
	}
	if !usernamePattern.MatchString(args[1]) {
		return sendError(c, "invalid_arguments", "CONNECT username must start with a lowercase letter and contain 3-20 lowercase letters, digits or underscores")
	}
	username := args[1]
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username != "" {
		return sendError(c, "already_authenticated", fmt.Sprintf("Connection already uses username %q", c.username))
	}
	if _, exists := s.players[username]; exists {
		return sendError(c, "username_taken", fmt.Sprintf("Username %q is already connected", username))
	}
	if !queue(c.outbox, "OK connected") {
		return false
	}
	c.username = username
	c.roomID = s.world.Start
	c.hp = maxPlayerHP
	s.players[username] = c
	s.broadcastRoomLocked(c.roomID, c, "EVT ROOM PRESENCE ENTER "+username)
	s.broadcastStatsLocked()
	return true
}

func (s *Server) look(c *client, args []string) bool {
	if !validDetailsArgs(args) {
		return sendError(c, "invalid_arguments", "LOOK takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return notAuthenticated(c, "LOOK")
	}
	room := s.world.Locations[c.roomID]
	items := s.roomItemsLocked(c.roomID)
	npcs := s.roomNPCsLocked(c.roomID)
	players := make([]string, 0)
	for username, other := range s.players {
		if other.roomID == c.roomID {
			players = append(players, username)
		}
	}
	s.mu.RUnlock()
	sort.Strings(players)
	var itemData, npcData any = items, npcs
	if len(args) == 1 {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		itemData = ids
		ids = make([]string, 0, len(npcs))
		for _, npc := range npcs {
			ids = append(ids, npc.ID)
		}
		npcData = ids
	}
	payload, err := json.Marshal(struct {
		Room    roomView `json:"room"`
		Players []string `json:"players"`
		Items   any      `json:"items"`
		NPCs    any      `json:"npcs"`
	}{Room: roomView{ID: room.ID, Name: room.Name, Description: room.Description, Exits: room.Exits}, Players: players, Items: itemData, NPCs: npcData})
	if err != nil {
		s.logger.Error("serialize LOOK failed", "error", err)
		return sendError(c, "internal_error", "LOOK response could not be encoded")
	}
	reply := "OK " + string(payload)
	if len(reply)+1 > maxLineBytes {
		return sendError(c, "response_too_large", "LOOK response exceeds 4096-byte line limit")
	}
	return queue(c.outbox, reply)
}

func (s *Server) move(c *client, args []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "MOVE")
	}
	oldRoom := c.roomID
	if c.target != "" {
		return sendError(c, "in_combat", "MOVE is unavailable in combat; use FLEE")
	}
	room := s.world.Locations[oldRoom]
	available := availableDirections(room)
	if len(args) != 2 {
		return sendError(c, "invalid_arguments", fmt.Sprintf("Usage: MOVE <direction>; available from %s: %s", oldRoom, available))
	}
	if !directions[args[1]] {
		return sendError(c, "invalid_direction", fmt.Sprintf("Direction %q is unknown; available from %s: %s", shortVerb(args[1]), oldRoom, available))
	}
	destination, exists := room.Exits[args[1]]
	if !exists {
		return sendError(c, "invalid_direction", fmt.Sprintf("No %s exit from %s; available directions: %s", args[1], oldRoom, available))
	}
	if !queue(c.outbox, "OK room="+destination) {
		return false
	}
	c.roomID = destination
	s.logger.Info("player moved", "player", c.username, "from", oldRoom, "to", destination)
	if destination != oldRoom {
		s.broadcastRoomLocked(oldRoom, c, "EVT ROOM PRESENCE LEAVE "+c.username)
		s.broadcastRoomLocked(destination, c, "EVT ROOM PRESENCE ENTER "+c.username)
	}
	return true
}

func availableDirections(room Room) string {
	if len(room.Exits) == 0 {
		return "none"
	}
	available := make([]string, 0, len(room.Exits))
	for direction := range room.Exits {
		available = append(available, direction)
	}
	sort.Strings(available)
	return strings.Join(available, ", ")
}

func (s *Server) chat(c *client, rest string, hasArgs bool) bool {
	if !hasArgs || strings.ContainsRune(rest, '\t') {
		return sendError(c, "invalid_arguments", "Usage: CHAT <GLOBAL|ROOM|GROUP> <message>")
	}
	channel, message, hasMessage := strings.Cut(rest, " ")
	channel = strings.ToUpper(channel)
	if !hasMessage {
		return sendError(c, "invalid_arguments", "Usage: CHAT <GLOBAL|ROOM|GROUP> <message>")
	}
	if strings.TrimSpace(message) == "" {
		return sendError(c, "invalid_arguments", "CHAT message cannot be empty")
	}
	if channel != "GLOBAL" && channel != "ROOM" && channel != "GROUP" {
		return sendError(c, "invalid_arguments", fmt.Sprintf("CHAT channel %q is invalid; use GLOBAL, ROOM or GROUP", shortVerb(channel)))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return notAuthenticated(c, "CHAT")
	}
	if channel == "GROUP" && c.group == "" {
		return sendError(c, "group_required", "CHAT GROUP requires group membership; use GROUP CREATE or GROUP JOIN")
	}
	event := "EVT " + channel + " CHAT " + c.username + " " + message
	if len(event)+1 > maxLineBytes {
		return sendError(c, "invalid_arguments", "CHAT message makes the event exceed 4096 bytes; shorten it")
	}
	if !queue(c.outbox, "OK") {
		return false
	}
	for _, other := range s.players {
		if (channel == "GLOBAL" || (channel == "ROOM" && other.roomID == c.roomID) || (channel == "GROUP" && other.group == c.group)) && !queue(other.outbox, event) {
			s.logger.Warn("client outbox full", "player", other.username, "event", "CHAT")
		}
	}
	return true
}

func (s *Server) who(c *client, args []string) bool {
	if !validDetailsArgs(args) {
		return sendError(c, "invalid_arguments", "WHO takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return notAuthenticated(c, "WHO")
	}
	roomPlayers := make([]string, 0)
	for name, other := range s.players {
		if other.roomID == c.roomID {
			roomPlayers = append(roomPlayers, name)
		}
	}
	serverCount := len(s.players)
	s.mu.RUnlock()
	if len(args) == 1 {
		return queue(c.outbox, fmt.Sprintf("OK players=%d", serverCount))
	}
	sort.Strings(roomPlayers)
	payload, err := json.Marshal(struct {
		Room   []string `json:"room"`
		Server int      `json:"server"`
	}{Room: roomPlayers, Server: serverCount})
	if err != nil {
		s.logger.Error("serialize WHO failed", "error", err)
		return sendError(c, "internal_error", "WHO response could not be encoded")
	}
	reply := "OK " + string(payload)
	if len(reply)+1 > maxLineBytes {
		return sendError(c, "response_too_large", "WHO response exceeds 4096-byte line limit")
	}
	return queue(c.outbox, reply)
}

func (s *Server) status(c *client, args []string) bool {
	if len(args) != 1 {
		return sendError(c, "invalid_arguments", "STATUS takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return notAuthenticated(c, "STATUS")
	}
	username, hp := c.username, c.hp
	state := "HORS_COMBAT"
	var combat *combatView
	if target := s.npcStates[c.target]; c.target != "" && target != nil {
		npc := s.world.NPCs[c.target]
		state = "EN_COMBAT"
		combat = &combatView{npc.ID, npc.Name, target.HP, npc.MaxHP}
	}
	s.mu.RUnlock()
	health := "healthy"
	if hp < maxPlayerHP {
		health = "injured"
	}
	if state == "EN_COMBAT" {
		health = "combat"
	}
	payload, err := json.Marshal(struct {
		Status string      `json:"status"`
		Player string      `json:"player"`
		HP     int         `json:"hp"`
		MaxHP  int         `json:"max_hp"`
		State  string      `json:"state"`
		Combat interface{} `json:"combat"`
	}{Status: health, Player: username, HP: hp, MaxHP: maxPlayerHP, State: state, Combat: combat})
	if err != nil {
		s.logger.Error("serialize STATUS failed", "error", err)
		return sendError(c, "internal_error", "STATUS response could not be encoded")
	}
	if len(payload)+4 > maxLineBytes {
		return sendError(c, "response_too_large", "STATUS response exceeds 4096-byte line limit")
	}
	return queue(c.outbox, "OK "+string(payload))
}

// Caller holds Server.mu. Only queueing occurs here; no socket writes or waits.
func (s *Server) broadcastRoomLocked(roomID string, actor *client, message string) {
	for _, other := range s.players {
		if other != actor && other.roomID == roomID && !queue(other.outbox, message) {
			s.logger.Warn("client outbox full", "player", other.username, "event", message)
		}
	}
}

// readLine bounds memory use while consuming an oversized request through LF.
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		for errors.Is(err, bufio.ErrBufferFull) {
			_, err = reader.ReadSlice('\n')
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return "", errLineTooLong
	}
	if err != nil {
		return "", err
	}
	if len(line) > maxLineBytes {
		return "", errLineTooLong
	}
	line = line[:len(line)-1]
	if !utf8.Valid(line) {
		return "", errInvalidUTF8
	}
	if len(line) == 0 {
		return "", errEmptyLine
	}
	if strings.ContainsRune(string(line), '\r') {
		return "", errCRInLine
	}
	for _, r := range string(line) {
		if unicode.IsControl(r) && r != '\t' {
			return "", errControlInLine
		}
	}
	return string(line), nil
}

func queue(outbox chan<- string, message string) bool {
	select {
	case outbox <- message:
		return true
	default:
		return false
	}
}

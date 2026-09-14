package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxLineBytes = 4096 // Includes LF, as documented in the protocol proposal.
	outboxSize   = 128
	writeTimeout = 5 * time.Second
	maxPlayerHP  = 100
)

var (
	errLineTooLong  = errors.New("line too long")
	errInvalidUTF8  = errors.New("invalid UTF-8")
	errInvalidLine  = errors.New("invalid line")
	usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,19}$`)
)

type client struct {
	conn       net.Conn
	outbox     chan string
	writerDone chan struct{}
	username   string // Protected by Server.mu.
	roomID     string // Protected by Server.mu.
	hp         int    // Protected by Server.mu.
}

// Server owns the connected-player state. World data is immutable after load.
type Server struct {
	logger  *slog.Logger
	world   *World
	mu      sync.RWMutex
	clients map[net.Conn]*client
	players map[string]*client
	wg      sync.WaitGroup
}

func New(logger *slog.Logger, world *World) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		logger: logger, world: world,
		clients: make(map[net.Conn]*client),
		players: make(map[string]*client),
	}
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
		c := &client{conn: conn, outbox: make(chan string, outboxSize), writerDone: make(chan struct{})}
		s.mu.Lock()
		s.clients[conn] = c
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serveClient(c)
	}
}

func (s *Server) serveClient(c *client) {
	defer s.wg.Done()
	go writeLoop(c.conn, c.outbox, c.writerDone)
	defer s.disconnect(c)

	s.logger.Info("client connected", "remote_addr", c.conn.RemoteAddr().String())
	if !queue(c.outbox, "OK hello proto=42TAP/1") {
		return
	}

	reader := bufio.NewReaderSize(c.conn, maxLineBytes)
	for {
		line, err := readLine(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			if !errors.Is(err, errLineTooLong) && !errors.Is(err, errInvalidUTF8) && !errors.Is(err, errInvalidLine) {
				s.logger.Warn("client read failed", "remote_addr", c.conn.RemoteAddr().String(), "error", err)
				return
			}
			reply := "ERR invalid_arguments Invalid line"
			if errors.Is(err, errLineTooLong) {
				reply = "ERR invalid_arguments Message too long"
			} else if errors.Is(err, errInvalidUTF8) {
				reply = "ERR invalid_arguments Invalid UTF-8"
			}
			if !queue(c.outbox, reply) {
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
		s.broadcastRoomLocked(c.roomID, c, "EVT ROOM PRESENCE LEAVE "+username)
	}
	delete(s.clients, c.conn)
	s.mu.Unlock()

	close(c.outbox)
	<-c.writerDone
	_ = c.conn.Close()
	s.logger.Info("client disconnected", "remote_addr", c.conn.RemoteAddr().String(), "player", username)
}

func (s *Server) handleCommand(c *client, line string) bool {
	verb, rest, hasArgs := strings.Cut(line, " ")
	if verb == "CHAT" {
		return s.chat(c, rest, hasArgs)
	}
	args := strings.Split(line, " ")
	if strings.ContainsRune(line, '\t') || args[0] == "" {
		return queue(c.outbox, "ERR invalid_arguments Invalid spacing")
	}
	for _, arg := range args {
		if arg == "" {
			return queue(c.outbox, "ERR invalid_arguments Invalid spacing")
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
	case "QUIT":
		if len(args) != 1 {
			return queue(c.outbox, "ERR invalid_arguments QUIT takes no arguments")
		}
		return false
	default:
		return queue(c.outbox, "ERR unknown_command Unknown command")
	}
}

func (s *Server) connect(c *client, args []string) bool {
	if len(args) != 2 {
		return queue(c.outbox, "ERR invalid_arguments CONNECT needs one username")
	}
	if !usernamePattern.MatchString(args[1]) {
		return queue(c.outbox, "ERR invalid_arguments Invalid username")
	}
	username := args[1]
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username != "" {
		return queue(c.outbox, "ERR already_authenticated Already connected")
	}
	if _, exists := s.players[username]; exists {
		return queue(c.outbox, "ERR username_taken Username already in use")
	}
	if !queue(c.outbox, "OK connected") {
		return false
	}
	c.username = username
	c.roomID = s.world.Start
	c.hp = maxPlayerHP
	s.players[username] = c
	s.broadcastRoomLocked(c.roomID, c, "EVT ROOM PRESENCE ENTER "+username)
	return true
}

func (s *Server) look(c *client, args []string) bool {
	if len(args) != 1 {
		return queue(c.outbox, "ERR invalid_arguments LOOK takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return queue(c.outbox, "ERR not_authenticated Send CONNECT first")
	}
	room := s.world.Locations[c.roomID]
	players := make([]string, 0)
	for username, other := range s.players {
		if other.roomID == c.roomID {
			players = append(players, username)
		}
	}
	s.mu.RUnlock()
	sort.Strings(players)
	payload, err := json.Marshal(struct {
		Room    Room          `json:"room"`
		Players []string      `json:"players"`
		Items   []interface{} `json:"items"`
		NPCs    []interface{} `json:"npcs"`
	}{Room: room, Players: players, Items: []interface{}{}, NPCs: []interface{}{}})
	if err != nil {
		s.logger.Error("serialize LOOK failed", "error", err)
		return queue(c.outbox, "ERR invalid_arguments Unable to show room")
	}
	reply := "OK " + string(payload)
	if len(reply)+1 > maxLineBytes {
		return queue(c.outbox, "ERR invalid_arguments Response too large")
	}
	return queue(c.outbox, reply)
}

func (s *Server) move(c *client, args []string) bool {
	if len(args) != 2 {
		return queue(c.outbox, "ERR invalid_arguments MOVE needs one direction")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return queue(c.outbox, "ERR not_authenticated Send CONNECT first")
	}
	oldRoom := c.roomID
	destination, exists := s.world.Locations[oldRoom].Exits[args[1]]
	if !exists {
		return queue(c.outbox, "ERR invalid_direction No exit in that direction")
	}
	if !queue(c.outbox, "OK room="+destination) {
		return false
	}
	c.roomID = destination
	if destination != oldRoom {
		s.broadcastRoomLocked(oldRoom, c, "EVT ROOM PRESENCE LEAVE "+c.username)
		s.broadcastRoomLocked(destination, c, "EVT ROOM PRESENCE ENTER "+c.username)
	}
	return true
}

func (s *Server) chat(c *client, rest string, hasArgs bool) bool {
	if !hasArgs || strings.ContainsRune(rest, '\t') {
		return queue(c.outbox, "ERR invalid_arguments CHAT needs a channel and message")
	}
	channel, message, hasMessage := strings.Cut(rest, " ")
	if !hasMessage || strings.TrimSpace(message) == "" {
		return queue(c.outbox, "ERR invalid_arguments CHAT needs a channel and message")
	}
	if channel != "GLOBAL" && channel != "ROOM" && channel != "GROUP" {
		return queue(c.outbox, "ERR invalid_arguments Invalid chat channel")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.username == "" {
		return queue(c.outbox, "ERR not_authenticated Send CONNECT first")
	}
	if channel == "GROUP" {
		return queue(c.outbox, "ERR invalid_arguments Not in a group")
	}
	event := "EVT " + channel + " CHAT " + c.username + " " + message
	if len(event)+1 > maxLineBytes {
		return queue(c.outbox, "ERR invalid_arguments Message too long")
	}
	if !queue(c.outbox, "OK") {
		return false
	}
	for _, other := range s.players {
		if (channel == "GLOBAL" || other.roomID == c.roomID) && !queue(other.outbox, event) {
			s.logger.Warn("client outbox full", "player", other.username, "event", "CHAT")
		}
	}
	return true
}

func (s *Server) who(c *client, args []string) bool {
	if len(args) != 1 {
		return queue(c.outbox, "ERR invalid_arguments WHO takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return queue(c.outbox, "ERR not_authenticated Send CONNECT first")
	}
	roomPlayers := make([]string, 0)
	for name, other := range s.players {
		if other.roomID == c.roomID {
			roomPlayers = append(roomPlayers, name)
		}
	}
	serverCount := len(s.players)
	s.mu.RUnlock()
	sort.Strings(roomPlayers)
	payload, err := json.Marshal(struct {
		Room   []string `json:"room"`
		Server int      `json:"server"`
	}{Room: roomPlayers, Server: serverCount})
	if err != nil {
		return queue(c.outbox, "ERR invalid_arguments Unable to list players")
	}
	reply := "OK " + string(payload)
	if len(reply)+1 > maxLineBytes {
		return queue(c.outbox, "ERR invalid_arguments Response too large")
	}
	return queue(c.outbox, reply)
}

func (s *Server) status(c *client, args []string) bool {
	if len(args) != 1 {
		return queue(c.outbox, "ERR invalid_arguments STATUS takes no arguments")
	}
	s.mu.RLock()
	if c.username == "" {
		s.mu.RUnlock()
		return queue(c.outbox, "ERR not_authenticated Send CONNECT first")
	}
	username, hp := c.username, c.hp
	s.mu.RUnlock()
	payload, err := json.Marshal(struct {
		Player string      `json:"player"`
		HP     int         `json:"hp"`
		MaxHP  int         `json:"max_hp"`
		State  string      `json:"state"`
		Combat interface{} `json:"combat"`
	}{Player: username, HP: hp, MaxHP: maxPlayerHP, State: "HORS_COMBAT", Combat: nil})
	if err != nil {
		return queue(c.outbox, "ERR invalid_arguments Unable to show status")
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
	if len(line) == 0 || strings.ContainsRune(string(line), '\r') {
		return "", errInvalidLine
	}
	return string(line), nil
}

// Only this goroutine writes to a client socket.
func writeLoop(conn net.Conn, outbox <-chan string, done chan<- struct{}) {
	defer close(done)
	for message := range outbox {
		_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := io.WriteString(conn, message+"\n"); err != nil {
			_ = conn.Close()
			return
		}
	}
}

func queue(outbox chan<- string, message string) bool {
	select {
	case outbox <- message:
		return true
	default:
		return false
	}
}

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App manages the GUI client state and TCP networking.
type App struct {
	ctx        context.Context
	conn       net.Conn
	writer     *bufio.Writer
	writeMu    sync.Mutex
	connMu     sync.RWMutex
	connected  bool
	username   string
	generation uint64
	sessionMu  sync.Mutex
	replyMu    sync.Mutex
	pending    []string

	// OnEvent is an optional callback invoked on every server event (for integration tests/headless mode)
	OnEvent func(eventType, payload string)
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the runtime starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown cleans up active network resources upon exit.
func (a *App) shutdown(ctx context.Context) {
	_ = a.Disconnect()
}

// ConnectionDefaults supplies the launcher's address to the connection form.
func (a *App) ConnectionDefaults() map[string]interface{} {
	host := os.Getenv("TAP_GUI_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(os.Getenv("TAP_GUI_PORT"))
	if err != nil || port < 1 || port > 65535 {
		port = 4242
	}
	return map[string]interface{}{"host": host, "port": port}
}

// Connect completes the protocol handshake before publishing an active session.
func (a *App) Connect(host string, port int, username string) error {
	a.sessionMu.Lock()
	conn, reader, err := a.connect(host, port, username)
	a.sessionMu.Unlock()
	if err != nil {
		return err
	}
	a.emitFor(conn, "raw_message", "OK connected")
	a.emitFor(conn, "server_ok", "connected")
	a.emitReply(conn, "OK", "CONNECT", "connected", "CONNECT "+strings.TrimSpace(username))
	go a.listenServer(conn, reader)
	return nil
}

func (a *App) connect(host string, port int, username string) (net.Conn, *bufio.Reader, error) {
	if a.IsConnected() {
		return nil, nil, fmt.Errorf("already connected")
	}
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, " \t\r\n") || port < 1 || port > 65535 {
		return nil, nil, fmt.Errorf("enter a server host and a port between 1 and 65535")
	}
	cleanUser := strings.TrimSpace(username)
	count := utf8.RuneCountInString(cleanUser)
	if !utf8.ValidString(cleanUser) || count < 3 || count > 20 {
		return nil, nil, fmt.Errorf("username must contain 3–20 lowercase letters, digits or underscores")
	}
	for i, r := range cleanUser {
		lowerLetter := unicode.IsLetter(r) && unicode.IsLower(r)
		if (i == 0 && !lowerLetter) || (i > 0 && !lowerLetter && !unicode.IsDigit(r) && r != '_') {
			return nil, nil, fmt.Errorf("username must start with a lowercase letter and contain only lowercase letters, digits or underscores")
		}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("connection failed to %s: %w", addr, err)
	}
	success := false
	defer func() {
		if !success {
			conn.Close()
		}
	}()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	greeting, err := readFrame(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("server greeting failed: %w", err)
	}
	if greeting != "OK hello proto=1" {
		return nil, nil, fmt.Errorf("unexpected server greeting: %s", greeting)
	}
	if _, err := fmt.Fprintf(conn, "CONNECT %s\n", cleanUser); err != nil {
		return nil, nil, fmt.Errorf("CONNECT failed: %w", err)
	}
	// Events may arrive while another player connects. The deadline bounds this loop.
	for {
		reply, err := readFrame(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("login failed: %w", err)
		}
		if strings.HasPrefix(reply, "EVT ") {
			continue
		}
		if reply != "OK connected" {
			return nil, nil, fmt.Errorf("login rejected: %s", reply)
		}
		break
	}
	_ = conn.SetDeadline(time.Time{})
	a.connMu.Lock()
	a.conn = conn
	a.writer = bufio.NewWriter(conn)
	a.connected = true
	a.username = cleanUser
	a.generation++
	a.replyMu.Lock()
	a.pending = nil
	a.replyMu.Unlock()
	a.connMu.Unlock()
	success = true
	return conn, reader, nil
}

func readFrame(reader *bufio.Reader) (string, error) {
	frame, err := reader.ReadSlice('\n')
	if err != nil {
		return "", err
	}
	if len(frame) > 4096 || !utf8.Valid(frame) || strings.ContainsRune(string(frame), '\r') {
		return "", fmt.Errorf("invalid server frame (UTF-8, LF and 4096-byte limit required)")
	}
	return strings.TrimSuffix(string(frame), "\n"), nil
}

func (a *App) isCurrent(conn net.Conn) bool {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	return a.conn == conn
}

func (a *App) emitFor(conn net.Conn, kind, payload string) {
	a.connMu.RLock()
	if a.conn != conn {
		a.connMu.RUnlock()
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, kind, payload)
	}
	a.connMu.RUnlock()
	if a.isCurrent(conn) && a.OnEvent != nil {
		a.OnEvent(kind, payload)
	}
}

func (a *App) emitReply(conn net.Conn, kind, command, payload, request string) {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	if a.conn == conn && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "server_reply", map[string]string{"kind": kind, "command": command, "payload": payload, "request": request})
	}
}

// detachConnection selects the sole owner of the session's disconnected event.
func (a *App) detachConnection(conn net.Conn) (uint64, bool) {
	a.connMu.Lock()
	if a.conn != conn {
		a.connMu.Unlock()
		return 0, false
	}
	a.conn = nil
	a.writer = nil
	a.connected = false
	a.username = ""
	a.replyMu.Lock()
	a.pending = nil
	a.replyMu.Unlock()
	generation := a.generation
	a.connMu.Unlock()
	return generation, true
}

func (a *App) notifyDisconnected(generation uint64, reason string) {
	a.connMu.RLock()
	if a.generation != generation || a.conn != nil {
		a.connMu.RUnlock()
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "disconnected", reason)
	}
	a.connMu.RUnlock()
	if a.OnEvent != nil {
		a.OnEvent("disconnected", reason)
	}
}

func (a *App) clearConnection(conn net.Conn, reason string) {
	if generation, detached := a.detachConnection(conn); detached {
		a.notifyDisconnected(generation, reason)
	}
}

// Disconnect gracefully terminates the TCP connection.
func (a *App) Disconnect() error {
	a.sessionMu.Lock()
	a.connMu.RLock()
	conn := a.conn
	a.connMu.RUnlock()
	if conn == nil {
		a.sessionMu.Unlock()
		return nil
	}
	// Detach first: the reader cannot race us to issue a second notification.
	// A short direct write keeps QUIT best-effort even if outgoing commands stall.
	generation, detached := a.detachConnection(conn)
	_ = conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
	_, _ = io.WriteString(conn, "QUIT\n")
	err := conn.Close()
	a.sessionMu.Unlock()
	if detached {
		a.notifyDisconnected(generation, "Client disconnected")
	}
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// SendCommand sends a line-terminated command to the server.
func (a *App) SendCommand(cmd string) error {
	cleanCmd := strings.TrimSpace(cmd)
	if cleanCmd == "" {
		return nil
	}
	if len(cleanCmd)+1 > 4096 || !utf8.ValidString(cleanCmd) || strings.ContainsAny(cleanCmd, "\r\n") {
		return fmt.Errorf("command must be valid UTF-8 on one line of at most 4096 bytes")
	}
	return a.sendRaw(cleanCmd + "\n")
}

func (a *App) sendRaw(payload string) error {
	a.connMu.RLock()
	defer a.connMu.RUnlock()

	if !a.connected || a.writer == nil {
		return fmt.Errorf("not connected to server")
	}

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	a.replyMu.Lock()
	if len(a.pending) >= 128 {
		a.replyMu.Unlock()
		return fmt.Errorf("too many pending commands; wait for server replies")
	}
	a.pending = append(a.pending, strings.TrimSuffix(payload, "\n"))
	a.replyMu.Unlock()
	_ = a.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := a.writer.WriteString(payload); err != nil {
		_ = a.conn.Close()
		return err
	}
	err := a.writer.Flush()
	if err != nil {
		_ = a.conn.Close()
	}
	return err
}

// Specific gameplay action bindings
func (a *App) Move(direction string) error {
	return a.SendCommand(fmt.Sprintf("MOVE %s", strings.TrimSpace(direction)))
}

func (a *App) Look() error {
	return a.SendCommand("LOOK DETAILS")
}

func (a *App) Take(item string) error {
	return a.SendCommand(fmt.Sprintf("TAKE %s", strings.TrimSpace(item)))
}

func (a *App) Drop(item string) error {
	return a.SendCommand(fmt.Sprintf("DROP %s", strings.TrimSpace(item)))
}

func (a *App) Inventory() error {
	return a.SendCommand("INVENTORY DETAILS")
}

func (a *App) Talk(npc string) error {
	return a.SendCommand(fmt.Sprintf("TALKJSON %s", strings.TrimSpace(npc)))
}

func (a *App) Attack(target string) error {
	return a.SendCommand(fmt.Sprintf("ATTACK %s", strings.TrimSpace(target)))
}

func (a *App) Chat(channel string, message string) error {
	return a.SendCommand(fmt.Sprintf("CHAT %s %s", strings.TrimSpace(channel), strings.TrimSpace(message)))
}

func (a *App) Status() error {
	return a.SendCommand("STATUS")
}

func (a *App) Quest(npc string) error {
	return a.SendCommand(fmt.Sprintf("QUEST %s", strings.TrimSpace(npc)))
}

func (a *App) Quests() error {
	return a.SendCommand("QUESTS")
}

func (a *App) Who() error {
	return a.SendCommand("WHO")
}

func (a *App) Group(action string, args string) error {
	if strings.TrimSpace(args) == "" {
		return a.SendCommand(fmt.Sprintf("GROUP %s", strings.TrimSpace(action)))
	}
	return a.SendCommand(fmt.Sprintf("GROUP %s %s", strings.TrimSpace(action), strings.TrimSpace(args)))
}

// IsConnected returns current connection status.
func (a *App) IsConnected() bool {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	return a.connected
}

// GetUsername returns logged-in username.
func (a *App) GetUsername() string {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	return a.username
}

// listenServer is a background loop reading \n-terminated messages from TCP.
func (a *App) listenServer(conn net.Conn, reader *bufio.Reader) {
	defer func() {
		conn.Close()
		a.clearConnection(conn, "Connection closed by server")
	}()
	for {
		line, err := readFrame(reader)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				a.emitFor(conn, "server_error", err.Error())
			}
			return
		}
		a.connMu.RLock()
		if a.conn != conn {
			a.connMu.RUnlock()
			return
		}
		command, request := "", ""
		if line == "OK" || strings.HasPrefix(line, "OK ") || strings.HasPrefix(line, "ERR ") {
			a.replyMu.Lock()
			if len(a.pending) > 0 {
				request = a.pending[0]
				command = strings.ToUpper(strings.Fields(request)[0])
				a.pending = a.pending[1:]
			}
			a.replyMu.Unlock()
		}
		a.connMu.RUnlock()
		if line == "" {
			continue
		}
		a.emitFor(conn, "raw_message", line)
		kind, payload := "", ""
		switch {
		case line == "OK" || strings.HasPrefix(line, "OK "):
			kind, payload = "OK", strings.TrimSpace(strings.TrimPrefix(line, "OK"))
			a.emitFor(conn, "server_ok", payload)
		case strings.HasPrefix(line, "ERR "):
			kind, payload = "ERR", strings.TrimSpace(strings.TrimPrefix(line, "ERR"))
			a.emitFor(conn, "server_err", payload)
		case strings.HasPrefix(line, "EVT "):
			a.emitFor(conn, "server_evt", strings.TrimSpace(strings.TrimPrefix(line, "EVT")))
		default:
			a.emitFor(conn, "server_error", "Unknown server frame")
			return
		}
		if kind != "" {
			a.emitReply(conn, kind, command, payload, request)
		}
	}
}

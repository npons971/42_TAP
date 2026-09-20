package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App manages the GUI client state and TCP networking.
type App struct {
	ctx       context.Context
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	writeMu   sync.Mutex
	connMu    sync.RWMutex
	connected bool
	username  string
	sessionMu sync.Mutex
	replyMu   sync.Mutex
	pending   []string
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

// Connect opens a TCP connection to the server and registers the player.
func (a *App) Connect(host string, port int, username string) error {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	a.connMu.Lock()
	if a.connected && a.conn != nil {
		a.connMu.Unlock()
		return fmt.Errorf("already connected")
	}
	a.connMu.Unlock()

	cleanUser := strings.TrimSpace(username)
	if cleanUser == "" {
		return fmt.Errorf("username cannot be empty")
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed to %s: %w", addr, err)
	}

	a.connMu.Lock()
	a.conn = conn
	reader := bufio.NewReader(conn)
	a.reader = reader
	a.writer = bufio.NewWriter(conn)
	a.connected = true
	a.username = cleanUser
	a.replyMu.Lock()
	a.pending = nil
	a.replyMu.Unlock()
	a.connMu.Unlock()

	// Launch async reader goroutine for continuous streaming
	go a.listenServer(conn, reader)

	// Send initial CONNECT command
	connectCmd := fmt.Sprintf("CONNECT %s", cleanUser)
	if err := a.SendCommand(connectCmd); err != nil {
		_ = a.disconnect()
		return fmt.Errorf("failed to send CONNECT: %w", err)
	}

	return nil
}

// Disconnect gracefully terminates the TCP connection.
func (a *App) Disconnect() error {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	return a.disconnect()
}

func (a *App) disconnect() error {
	a.connMu.RLock()
	conn := a.conn
	a.connMu.RUnlock()
	if conn == nil {
		return nil
	}
	_ = a.SendCommand("QUIT")
	err := conn.Close()
	a.connMu.Lock()
	if a.conn == conn {
		a.conn = nil
		a.reader = nil
		a.writer = nil
		a.connected = false
		a.username = ""
	}
	a.connMu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "disconnected", "Client disconnected")
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
		a.connMu.Lock()
		current := a.conn == conn
		if current {
			a.connected = false
			a.conn = nil
			a.reader = nil
			a.writer = nil
		}
		a.connMu.Unlock()
		if current && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "disconnected", "Connection closed by server")
		}
	}()
	for {

		if reader == nil {
			break
		}

		frame, err := reader.ReadSlice('\n')
		line := string(frame)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "server_error", err.Error())
				}
			}
			break
		}

		if len(line) > 4096 || !utf8.ValidString(line) || strings.ContainsRune(line, '\r') {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "server_error", "Invalid server frame")
			}
			return
		}
		line = strings.TrimSuffix(line, "\n")
		a.connMu.RLock()
		current := a.conn == conn
		if !current {
			a.connMu.RUnlock()
			return
		}
		command, request := "", ""
		if (line == "OK" || strings.HasPrefix(line, "OK ") || strings.HasPrefix(line, "ERR ")) && !strings.HasPrefix(line, "OK hello ") {
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

		if a.ctx == nil {
			continue
		}

		// Emit raw message for general logging
		runtime.EventsEmit(a.ctx, "raw_message", line)

		// Typed event emission for reactive UI updates
		switch {
		case line == "OK" || strings.HasPrefix(line, "OK "):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "OK"))
			runtime.EventsEmit(a.ctx, "server_ok", payload)
			runtime.EventsEmit(a.ctx, "server_reply", map[string]string{"kind": "OK", "command": command, "payload": payload, "request": request})
		case strings.HasPrefix(line, "ERR "):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "ERR"))
			runtime.EventsEmit(a.ctx, "server_err", payload)
			runtime.EventsEmit(a.ctx, "server_reply", map[string]string{"kind": "ERR", "command": command, "payload": payload, "request": request})
		case strings.HasPrefix(line, "EVT "):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "EVT"))
			runtime.EventsEmit(a.ctx, "server_evt", payload)
		}
	}
}

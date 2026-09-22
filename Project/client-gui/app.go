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

// Connect opens a TCP connection to the server and registers the player.
func (a *App) Connect(host string, port int, username string) error {
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
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connection failed to %s: %w", addr, err)
	}

	a.connMu.Lock()
	a.conn = conn
	a.reader = bufio.NewReader(conn)
	a.writer = bufio.NewWriter(conn)
	a.connected = true
	a.username = cleanUser
	a.connMu.Unlock()

	// Launch async reader goroutine for continuous streaming
	go a.listenServer()

	// Send initial CONNECT command
	connectCmd := fmt.Sprintf("CONNECT %s", cleanUser)
	if err := a.SendCommand(connectCmd); err != nil {
		_ = a.Disconnect()
		return fmt.Errorf("failed to send CONNECT: %w", err)
	}

	return nil
}

// Disconnect gracefully terminates the TCP connection.
func (a *App) Disconnect() error {
	// Try sending QUIT if still alive (sendRaw acquires RLock)
	_ = a.sendRaw("QUIT\n")

	a.connMu.Lock()
	defer a.connMu.Unlock()

	if !a.connected || a.conn == nil {
		return nil
	}

	err := a.conn.Close()
	a.conn = nil
	a.reader = nil
	a.writer = nil
	a.connected = false
	a.username = ""

	if a.OnEvent != nil {
		a.OnEvent("disconnected", "Client disconnected")
	}
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

	if _, err := a.writer.WriteString(payload); err != nil {
		return err
	}
	return a.writer.Flush()
}

// Specific gameplay action bindings
func (a *App) Move(direction string) error {
	return a.SendCommand(fmt.Sprintf("MOVE %s", strings.TrimSpace(direction)))
}

func (a *App) Look() error {
	return a.SendCommand("LOOK")
}

func (a *App) Take(item string) error {
	return a.SendCommand(fmt.Sprintf("TAKE %s", strings.TrimSpace(item)))
}

func (a *App) Drop(item string) error {
	return a.SendCommand(fmt.Sprintf("DROP %s", strings.TrimSpace(item)))
}

func (a *App) Inventory() error {
	return a.SendCommand("INVENTORY")
}

func (a *App) Talk(npc string) error {
	return a.SendCommand(fmt.Sprintf("TALK %s", strings.TrimSpace(npc)))
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

func (a *App) Quest(questID string) error {
	return a.SendCommand(fmt.Sprintf("QUEST %s", strings.TrimSpace(questID)))
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
func (a *App) listenServer() {
	for {
		a.connMu.RLock()
		reader := a.reader
		a.connMu.RUnlock()

		if reader == nil {
			break
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				if a.OnEvent != nil {
					a.OnEvent("server_error", err.Error())
				}
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "server_error", err.Error())
				}
			}
			a.connMu.Lock()
			a.connected = false
			a.conn = nil
			a.reader = nil
			a.writer = nil
			a.connMu.Unlock()

			if a.OnEvent != nil {
				a.OnEvent("disconnected", "Connection closed by server")
			}
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "disconnected", "Connection closed by server")
			}
			break
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		// Emit raw message for general logging
		if a.OnEvent != nil {
			a.OnEvent("raw_message", line)
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "raw_message", line)
		}

		// Typed event emission for reactive UI updates
		switch {
		case strings.HasPrefix(line, "OK"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "OK"))
			if a.OnEvent != nil {
				a.OnEvent("server_ok", payload)
			}
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "server_ok", payload)
			}
		case strings.HasPrefix(line, "ERR"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "ERR"))
			if a.OnEvent != nil {
				a.OnEvent("server_err", payload)
			}
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "server_err", payload)
			}
		case strings.HasPrefix(line, "EVT"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "EVT"))
			if a.OnEvent != nil {
				a.OnEvent("server_evt", payload)
			}
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "server_evt", payload)
			}
		}
	}
}

package server

import (
	"io"
	"net"
	"strings"
	"time"
)

const commandBurst = 128
const commandsPerSecond = 20

type tokenBucket struct {
	tokens  float64
	updated time.Time
}

func (b *tokenBucket) allow(now time.Time) bool {
	if b.updated.IsZero() {
		b.tokens = commandBurst
		b.updated = now
	}
	elapsed := now.Sub(b.updated).Seconds()
	if elapsed > 0 {
		b.tokens = min(float64(commandBurst), b.tokens+elapsed*commandsPerSecond)
		b.updated = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *Server) recordConnectionLocked(addr net.Addr, now time.Time) {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	for ip, entry := range s.connectionHistory {
		if now.Sub(entry.start) > 2*time.Second {
			delete(s.connectionHistory, ip)
		}
	}
	entry := s.connectionHistory[host]
	if entry.start.IsZero() {
		entry.start = now
	}
	entry.count++
	// This is monitoring, not IP banning: users may share one school/NAT address.
	if entry.count > 5 {
		s.logger.Warn("rapid connections", "event", "RAPID_RECONNECT", "client_ip", host, "count", entry.count)
	}
	if len(s.connectionHistory) < 1024 || s.connectionHistory[host].count > 0 {
		s.connectionHistory[host] = entry
	}
}

type connectionWindow struct {
	start time.Time
	count int
}

func (s *Server) identity(c *client) (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return c.username, c.conn.RemoteAddr().String()
}

// Rejected connections never enter the session/writer lifecycle, so audit them
// here using the same response logger as admitted clients.
func (s *Server) rejectConnection(c *client) {
	remote := c.conn.RemoteAddr().String()
	s.logger.Info("client connected", "remote_addr", remote)
	s.logger.Warn("connection rejected", "event", "CONNECTION_REJECTED", "remote_addr", remote, "error_code", "503", "limit", maxConnections)
	defer func() {
		_ = c.conn.Close()
		s.logger.Info("client disconnected", "remote_addr", remote, "player", "", "duration_ms", time.Since(c.connectedAt).Milliseconds(), "reason", "connection_limit")
	}()
	if err := c.conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		s.logger.Error("socket write deadline failed", "remote_addr", remote, "error", err)
		return
	}
	for _, message := range []string{"OK hello proto=1", "ERR 503 CONNECTION_LIMIT Server connection limit reached"} {
		if _, err := io.WriteString(c.conn, message+"\n"); err != nil {
			s.logger.Error("socket write failed", "remote_addr", remote, "error", err)
			return
		}
		s.logResponse("", remote, message)
	}
}

func (s *Server) logResponse(username, remote, message string) {
	attrs := []any{"event", "SERVER_RESPONSE", "player", username, "remote_addr", remote, "response", message}
	if strings.HasPrefix(message, "ERR ") {
		parts := strings.SplitN(message, " ", 3)
		if len(parts) > 1 {
			attrs = append(attrs, "error_code", parts[1])
		}
		s.logger.Warn("server response", attrs...)
	} else {
		s.logger.Info("server response", attrs...)
	}
}

func (s *Server) writeClient(c *client) {
	defer close(c.writerDone)
	for message := range c.outbox {
		_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := c.conn.Write([]byte(message + "\n")); err != nil {
			s.logger.Error("socket write failed", "remote_addr", c.conn.RemoteAddr().String(), "error", err)
			_ = c.conn.Close()
			return
		}
		username, remote := s.identity(c)
		s.logResponse(username, remote, message)
	}
}

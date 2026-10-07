package server

import (
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
}

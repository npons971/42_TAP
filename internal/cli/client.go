// Package cli implements a concurrent, line-oriented TAP terminal client.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const maxLineBytes = 4096

// Run owns conn and input, closing them on completion/cancellation. Input is
// independent from reception so an idle terminal never blocks incoming events.
func Run(ctx context.Context, conn net.Conn, input io.ReadCloser, output io.Writer, interactive bool) error {
	display := &terminalDisplay{out: output, interactive: interactive}
	if interactive {
		display.redraw()
	}
	received := make(chan error, 1)
	sent := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	workers.Add(2)
	defer func() { cancel(); conn.Close(); input.Close(); workers.Wait() }()
	go func() {
		defer workers.Done()
		r := bufio.NewReaderSize(conn, maxLineBytes)
		for {
			line, err := readWireLine(r)
			if err != nil {
				received <- err
				return
			}
			if err := display.message(line); err != nil {
				received <- err
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		if interactive {
			sent <- readInteractive(runCtx, input, display, conn)
			return
		}
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, maxLineBytes), maxLineBytes+1)
		for scanner.Scan() {
			line := strings.TrimSuffix(scanner.Text(), "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			if err := sendLine(conn, line); err != nil {
				sent <- err
				return
			}
			if strings.EqualFold(line, "QUIT") {
				sent <- nil
				return
			}
		}
		if err := scanner.Err(); err != nil {
			sent <- fmt.Errorf("input: %w", err)
			return
		}
		sent <- sendLine(conn, "QUIT")
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-sent:
			if err != nil {
				return err
			}
			// QUIT/EOF waits for server closure, but a broken server cannot hang forever.
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			sent = nil
		case err := <-received:
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("server: %w", err)
		}
	}
}

func sendLine(conn net.Conn, line string) error {
	if len(line)+1 > maxLineBytes || !utf8.ValidString(line) || strings.ContainsAny(line, "\r\n") {
		return fmt.Errorf("command must be valid UTF-8 on one LF-terminated line of at most 4096 bytes")
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	data := []byte(line + "\n")
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func readWireLine(r *bufio.Reader) (string, error) {
	data, err := r.ReadSlice('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && len(data) > 0 {
			return "", fmt.Errorf("unterminated server reply")
		}
		return "", err
	}
	if len(data) > maxLineBytes || !utf8.Valid(data) || strings.ContainsRune(string(data), '\r') {
		return "", fmt.Errorf("invalid server line")
	}
	line := string(data[:len(data)-1])
	if line != "OK" && !strings.HasPrefix(line, "OK ") && !strings.HasPrefix(line, "ERR ") && !strings.HasPrefix(line, "EVT ") {
		return "", fmt.Errorf("unrecognized server reply %q", line)
	}
	return line, nil
}

type terminalDisplay struct {
	mu          sync.Mutex
	out         io.Writer
	interactive bool
	pending     []byte
}

func (d *terminalDisplay) redraw() {
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprintf(d.out, "\r\033[2K> %s", d.pending)
}
func (d *terminalDisplay) message(line string) error {
	// Peer-supplied terminal controls must not erase the prompt or execute ANSI.
	var safe strings.Builder
	for _, r := range line {
		if r < 32 || (r >= 127 && r <= 159) {
			fmt.Fprintf(&safe, "\\u%04x", r)
		} else {
			safe.WriteRune(r)
		}
	}
	line = safe.String()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.interactive {
		_, err := fmt.Fprintf(d.out, "\r\033[2K%s\r\n> %s", line, d.pending)
		return err
	}
	_, err := fmt.Fprintln(d.out, line)
	return err
}
func readInteractive(ctx context.Context, input io.Reader, d *terminalDisplay, conn net.Conn) error {
	one := make([]byte, 1)
	escape := false
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, err := input.Read(one)
		if err == io.EOF {
			return sendLine(conn, "QUIT")
		}
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		b := one[0]
		// Ignore ANSI escape sequences (arrows); backspace and Ctrl-U are supported.
		if b == 27 {
			escape = true
			continue
		}
		if escape {
			if b >= 64 && b <= 126 && b != '[' {
				escape = false
			}
			continue
		}
		if b == 3 || b == 4 {
			return sendLine(conn, "QUIT")
		}
		d.mu.Lock()
		switch b {
		case '\r', '\n':
			line := string(d.pending)
			d.pending = nil
			fmt.Fprint(d.out, "\r\n> ")
			d.mu.Unlock()
			if strings.TrimSpace(line) == "" {
				continue
			}
			if err := sendLine(conn, line); err != nil {
				d.message("LOCAL ERROR: " + err.Error())
				continue
			}
			if strings.EqualFold(line, "QUIT") {
				return nil
			}
			continue
		case 127, 8:
			if len(d.pending) > 0 {
				_, size := utf8.DecodeLastRune(d.pending)
				d.pending = d.pending[:len(d.pending)-size]
			}
		case 21:
			d.pending = nil
		default:
			if (b >= 32 || b == '\t') && len(d.pending) <= maxLineBytes {
				d.pending = append(d.pending, b)
			}
		}
		// A partial UTF-8 sequence is buffered until the next byte completes it.
		if utf8.Valid(d.pending) {
			fmt.Fprintf(d.out, "\r\033[2K> %s", d.pending)
		}
		d.mu.Unlock()
	}
}

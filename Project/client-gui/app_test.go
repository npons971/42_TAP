package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestGUIProtocolCommandsAndDisconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan string, 16)
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		fmt.Fprintln(conn, "OK hello proto=1")
		r := bufio.NewScanner(conn)
		for r.Scan() {
			line := r.Text()
			commands <- line
			if line == "QUIT" {
				fmt.Fprintln(conn, "OK bye")
				done <- nil
				return
			}
			fmt.Fprintln(conn, "OK")
			fmt.Fprintln(conn, "EVT STATS players=1")
		}
		done <- r.Err()
	}()
	app := NewApp()
	addr := listener.Addr().(*net.TCPAddr)
	if err := app.Connect("127.0.0.1", addr.Port, "élise"); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		action func() error
		want   string
	}{
		{nil, "CONNECT élise"}, {app.Look, "LOOK DETAILS"}, {app.Inventory, "INVENTORY DETAILS"},
		{func() error { return app.Talk("Village Guard") }, "TALKJSON Village Guard"},
		{app.Who, "WHO"}, {app.Quests, "QUESTS"}, {func() error { return app.Quest("npc.herbalist") }, "QUEST npc.herbalist"},
		{func() error { return app.Group("CREATE", "") }, "GROUP CREATE"},
		{func() error { return app.Group("INVITE", "bob") }, "GROUP INVITE bob"},
		{func() error { return app.Group("JOIN", "bob") }, "GROUP JOIN bob"},
	}
	for _, step := range steps {
		if step.action != nil {
			if err := step.action(); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case got := <-commands:
			if got != step.want {
				t.Fatalf("got %s, want %s", got, step.want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("GUI send blocked")
		}
	}
	for _, cmd := range []string{"LOOK\nQUIT", "CHAT GLOBAL \xff", strings.Repeat("x", 4096)} {
		if app.SendCommand(cmd) == nil {
			t.Fatalf("invalid frame accepted: %q", cmd)
		}
	}
	disconnected := make(chan error, 1)
	go func() { disconnected <- app.Disconnect() }()
	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Fatal("GUI Disconnect deadlocked")
	}
	if app.IsConnected() {
		t.Fatal("still connected")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive QUIT")
	}
}

func TestGUIConcurrentConnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprintln(conn, "OK hello proto=1")
		r := bufio.NewScanner(conn)
		for r.Scan() {
			if r.Text() == "QUIT" {
				return
			}
			fmt.Fprintln(conn, "OK connected")
		}
	}()
	app := NewApp()
	port := listener.Addr().(*net.TCPAddr).Port
	errs := make(chan error, 2)
	gate := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() { <-gate; errs <- app.Connect("127.0.0.1", port, "alice") }()
	}
	close(gate)
	success := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err == nil {
				success++
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent connect blocked")
		}
	}
	if success != 1 {
		t.Fatalf("%d simultaneous connections accepted", success)
	}
	app.Disconnect()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connection leaked")
	}
}

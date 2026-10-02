//go:build linux

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Run the real CLI entry point in a child process attached to a pseudo-terminal.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("TAP_CLI_TEST_PROCESS") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("tap-cli", flag.ExitOnError)
	os.Args = []string{"tap-cli", "-addr", os.Getenv("TAP_CLI_TEST_ADDR")}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func terminalIoctl(t *testing.T, file *os.File, request uintptr, value unsafe.Pointer) {
	t.Helper()
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), request, uintptr(value))
	if errno != 0 {
		t.Fatal(errno)
	}
}

func TestCLITerminal(t *testing.T) {
	for _, scenario := range []string{"disconnect", "input", "signal"} {
		t.Run(scenario, func(t *testing.T) {
			master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
			var unlock, number uint32
			terminalIoctl(t, master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock))
			terminalIoctl(t, master, syscall.TIOCGPTN, unsafe.Pointer(&number))
			slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|syscall.O_NOCTTY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer slave.Close()
			var before, after syscall.Termios
			terminalIoctl(t, slave, syscall.TCGETS, unsafe.Pointer(&before))

			listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			listener.SetDeadline(time.Now().Add(10 * time.Second))
			commands := make(chan []string, 1)
			serverErrors := make(chan error, 1)
			greeted := make(chan struct{})
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					serverErrors <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(10 * time.Second))
				if _, err := fmt.Fprintln(conn, "OK hello proto=1"); err != nil {
					serverErrors <- err
					return
				}
				if scenario == "disconnect" {
					<-greeted
					_, err = fmt.Fprintln(conn, "EVT GLOBAL CHAT bob hello")
					serverErrors <- err
					return
				}
				var received []string
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					command := scanner.Text()
					received = append(received, command)
					if command == "QUIT" {
						break
					}
					if _, err := fmt.Fprintln(conn, "OK"); err != nil {
						serverErrors <- err
						return
					}
				}
				commands <- received
				serverErrors <- scanner.Err()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			process := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCLIProcess$")
			process.Env = append(os.Environ(), "TAP_CLI_TEST_PROCESS=1", "TAP_CLI_TEST_ADDR="+listener.Addr().String())
			process.Stdin, process.Stdout, process.Stderr = slave, slave, slave
			if err := process.Start(); err != nil {
				t.Fatal(err)
			}
			defer process.Process.Kill()
			ready := make(chan error, 1)
			go func() {
				reader := bufio.NewReader(master)
				for {
					line, err := reader.ReadString('\n')
					if strings.Contains(line, "OK hello proto=1") {
						ready <- nil
						return
					}
					if err != nil {
						ready <- err
						return
					}
				}
			}()
			select {
			case err := <-ready:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("CLI greeting not displayed")
			}
			close(greeted)
			switch scenario {
			case "input":
				if _, err := master.WriteString("TALK Village Guard\rQUIT\r"); err != nil {
					t.Fatal(err)
				}
			case "signal":
				if err := process.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			if err := process.Wait(); err != nil {
				t.Fatalf("CLI did not exit cleanly: %v", err)
			}
			terminalIoctl(t, slave, syscall.TCGETS, unsafe.Pointer(&after))
			if before != after {
				t.Fatal("terminal attributes were not restored")
			}
			select {
			case err := <-serverErrors:
				if err != nil && scenario != "signal" {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("mock server remained blocked")
			}
			if scenario == "input" {
				if got := <-commands; !reflect.DeepEqual(got, []string{"TALK Village Guard", "QUIT"}) {
					t.Fatalf("unexpected commands: %v", got)
				}
			}
		})
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/npons971/42_TAP/internal/cli"
)

func run() error {
	addr := flag.String("addr", "127.0.0.1:4242", "TCP server address")
	flag.Parse()
	conn, err := net.DialTimeout("tcp", *addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	restore, interactive, err := cli.RawTerminal(os.Stdin)
	if err != nil {
		return err
	}
	defer restore()
	input, err := cli.InputFile(os.Stdin)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, conn, &terminalInput{File: input, restore: restore}, os.Stdout, interactive)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "TAP CLI:", err)
		os.Exit(1)
	}
}

type terminalInput struct {
	*os.File
	restore func()
}

func (t *terminalInput) Close() error { t.restore(); return t.File.Close() }

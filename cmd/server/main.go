package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/npons971/42_TAP/internal/server"
)

func main() {
	addr := flag.String("addr", ":4242", "TCP listen address")
	worldPath := flag.String("world", "data/world.json", "JSON world file (default: full nine-room world)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	world, err := server.LoadWorld(*worldPath)
	if err != nil {
		logger.Error("world load failed", "path", *worldPath, "error", err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Error("listen failed", "addr", *addr, "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("server listening", "addr", listener.Addr().String())
	if err := server.New(logger, world).Serve(ctx, listener); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

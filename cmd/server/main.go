package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	logPath := flag.String("log-file", "", "append structured JSON logs to a file (default: stdout)")
	flag.Parse()

	logOutput := os.Stdout
	if *logPath != "" {
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			slog.Error("open log file failed", "error", err)
			os.Exit(1)
		}
		defer file.Close()
		logOutput = file
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
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
	printClientCommands(listener)
	if err := server.New(logger, world).Serve(ctx, listener); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// Show only addresses reachable through the interface on which we listen.
func printClientCommands(listener net.Listener) {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return
	}
	var hosts []string
	if !address.IP.IsUnspecified() {
		host := address.IP.String()
		if address.Zone != "" {
			host += "%" + address.Zone
		}
		hosts = append(hosts, host)
	} else {
		interfaces, _ := net.Interfaces()
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, _ := iface.Addrs()
			for _, candidate := range addresses {
				ip, _, err := net.ParseCIDR(candidate.String())
				if err != nil || !ip.IsGlobalUnicast() {
					continue
				}
				// An IPv4-only listener cannot accept IPv6 connections.
				if address.IP.To4() != nil && ip.To4() == nil {
					continue
				}
				hosts = append(hosts, ip.String())
			}
		}
	}
	if len(hosts) == 0 {
		fmt.Println("Aucune adresse réseau détectée pour afficher les commandes des clients.")
		return
	}
	if address.IP.IsLoopback() {
		fmt.Println("Serveur accessible uniquement depuis ce PC :")
	} else {
		fmt.Println("Depuis un autre PC, utilise l'adresse de l'interface sur le même réseau :")
	}
	for _, host := range hosts {
		port := fmt.Sprint(address.Port)
		fmt.Printf("\n  make run-cli CLI_ADDR='%s'\n", net.JoinHostPort(host, port))
		fmt.Printf("  make run-gui GUI_HOST='%s' GUI_PORT=%s GUI_SERVER=off\n", host, port)
	}
	fmt.Println()
}

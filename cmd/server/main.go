package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amalg/go-bomberman/internal/game"
	"github.com/amalg/go-bomberman/internal/network"
	"github.com/amalg/go-bomberman/internal/ui"
)

func main() {
	port := flag.Int("port", 9999, "Port to listen on")
	name := flag.String("name", "Host", "Your player name")
	width := flag.Int("width", 15, "Board width (odd number)")
	height := flag.Int("height", 13, "Board height (odd number)")
	maxPlayers := flag.Int("max-players", 4, "Maximum number of players")
	logFile := flag.String("log", "", "Log file path (default: discard server logs)")
	flag.Parse()

	// Ensure odd dimensions for proper wall grid
	if *width%2 == 0 {
		*width++
	}
	if *height%2 == 0 {
		*height++
	}

	config := game.DefaultConfig()
	config.Width = *width
	config.Height = *height
	config.MaxPlayers = *maxPlayers

	addr := fmt.Sprintf("0.0.0.0:%d", *port)

	// Redirect log output IMMEDIATELY — before any server code runs.
	// Server goroutines use log.Printf which writes to stderr by default.
	// Any stderr output will corrupt Bubbletea's terminal rendering.
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		log.SetOutput(f)
	} else {
		log.SetOutput(io.Discard)
	}

	// Create and start the server
	server := network.NewServer(addr, config)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Give the TCP listener time to be fully ready
	time.Sleep(200 * time.Millisecond)

	// Connect as the host player (local loopback)
	clientAddr := fmt.Sprintf("127.0.0.1:%d", *port)
	client, err := network.NewClient(clientAddr, *name)
	if err != nil {
		server.Stop()
		fmt.Fprintf(os.Stderr, "Failed to connect as host: %v\n", err)
		os.Exit(1)
	}

	// Print connection info for other players
	fmt.Printf("💣 Bomberman Server on port %d\n", *port)
	printLocalAddrs(*port)
	fmt.Printf("\nConnected as %s. Starting TUI...\n", *name)

	// Small pause so the user can read the IPs
	time.Sleep(500 * time.Millisecond)

	// Handle OS signals for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		client.Close()
		server.Stop()
		os.Exit(0)
	}()

	// Start the TUI — this takes over the terminal completely
	model := ui.NewModel(client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		client.Close()
		server.Stop()
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Clean shutdown after TUI exits
	client.Close()
	server.Stop()
}

// printLocalAddrs prints all local network addresses for players to connect to.
func printLocalAddrs(port int) {
	fmt.Println("Players can connect using:")
	fmt.Printf("  127.0.0.1:%d (this machine)\n", port)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				fmt.Printf("  %s:%d\n", ipnet.IP.String(), port)
			}
		}
	}
}

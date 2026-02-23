package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amalg/go-bomberman/internal/network"
	"github.com/amalg/go-bomberman/internal/ui"
)

func main() {
	addr := flag.String("addr", "", "Server address (e.g., 192.168.1.5:9999)")
	name := flag.String("name", "Player", "Your player name")
	flag.Parse()

	if *addr == "" {
		fmt.Fprintln(os.Stderr, "Usage: client --addr <host:port> [--name <name>]")
		fmt.Fprintln(os.Stderr, "  Example: client --addr 192.168.1.5:9999 --name Alice")
		os.Exit(1)
	}

	fmt.Printf("Connecting to %s as %s...\n", *addr, *name)

	client, err := network.NewClient(*addr, *name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Printf("Connected! Player ID: %s\n", client.PlayerID())
	fmt.Println("Starting TUI...")
	time.Sleep(500 * time.Millisecond)

	// Start the TUI
	model := ui.NewModel(client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amalg/go-bomberman/internal/ui"
)

func main() {
	name := flag.String("name", "", "Your player name")
	port := flag.Int("port", 9999, "Game port (for hosting)")
	flag.Parse()

	model := ui.NewModel(*name, *port)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

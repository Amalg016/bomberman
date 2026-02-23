package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/amalg/go-bomberman/internal/game"
)

// Color palette
var (
	// Tile styles
	hardWallStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3a3a3a")).
			Foreground(lipgloss.Color("#555555"))

	softWallStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#8B6914")).
			Foreground(lipgloss.Color("#A0772B"))

	emptyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).
			Foreground(lipgloss.Color("#1a1a2e"))

	bombStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).
			Foreground(lipgloss.Color("#ff4444")).
			Bold(true)

	fireStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#ff6600")).
			Foreground(lipgloss.Color("#ffcc00")).
			Bold(true)

	// Player colors (4 distinct colors for up to 4 players)
	playerColors = []lipgloss.Color{
		lipgloss.Color("#00ff88"), // Green
		lipgloss.Color("#4488ff"), // Blue
		lipgloss.Color("#ff44ff"), // Magenta
		lipgloss.Color("#ffff44"), // Yellow
	}

	deadPlayerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).
			Foreground(lipgloss.Color("#666666")).
			Strikethrough(true)

	// HUD styles
	hudBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444466")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff8844")).
			Bold(true)

	lobbyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44aaff")).
			Bold(true)

	winnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff88")).
			Bold(true).
			Blink(true)
)

// RenderBoard converts the game state into a styled terminal string.
func RenderBoard(state *game.GameState, myID string) string {
	if state == nil || len(state.Board) == 0 {
		return "Waiting for game state..."
	}

	// Build fire lookup
	fireSet := make(map[game.Position]bool)
	for _, f := range state.Fires {
		fireSet[f.Pos] = true
	}

	// Build bomb lookup
	bombSet := make(map[game.Position]*game.Bomb)
	for _, b := range state.Bombs {
		bombSet[b.Pos] = b
	}

	// Build player lookup
	playerSet := make(map[game.Position]*game.Player)
	for _, p := range state.Players {
		if p.Alive {
			playerSet[p.Pos] = p
		}
	}

	var rows []string
	for y := 0; y < state.Height; y++ {
		var cells []string
		for x := 0; x < state.Width; x++ {
			pos := game.Position{X: x, Y: y}
			cell := renderCell(state.Board[y][x], pos, fireSet, bombSet, playerSet, myID)
			cells = append(cells, cell)
		}
		rows = append(rows, strings.Join(cells, ""))
	}

	return strings.Join(rows, "\n")
}

// renderCell renders a single board cell with the appropriate style.
// Each cell is 2 characters wide for a square-ish appearance.
func renderCell(
	tile game.TileType,
	pos game.Position,
	fireSet map[game.Position]bool,
	bombSet map[game.Position]*game.Bomb,
	playerSet map[game.Position]*game.Player,
	myID string,
) string {
	// Priority: Player > Fire > Bomb > Tile
	if p, ok := playerSet[pos]; ok {
		style := lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).
			Bold(true)
		colorIdx := p.Color % len(playerColors)
		style = style.Foreground(playerColors[colorIdx])

		label := fmt.Sprintf("P%d", p.Color+1)
		if p.ID == myID {
			label = "██"
			style = style.Foreground(playerColors[colorIdx]).
				Background(playerColors[colorIdx])
		}
		return style.Render(label)
	}

	if fireSet[pos] {
		return fireStyle.Render("░░")
	}

	if _, ok := bombSet[pos]; ok {
		return bombStyle.Render("()")
	}

	switch tile {
	case game.HardWall:
		return hardWallStyle.Render("██")
	case game.SoftWall:
		return softWallStyle.Render("▒▒")
	default:
		return emptyStyle.Render("  ")
	}
}

// RenderHUD renders the heads-up display showing player info and game status.
func RenderHUD(state *game.GameState, myID string) string {
	if state == nil {
		return ""
	}

	var parts []string

	// Title
	parts = append(parts, titleStyle.Render("💣 BOMBERMAN"))
	parts = append(parts, "")

	// Game status
	switch state.Status {
	case game.StatusLobby:
		parts = append(parts, lobbyStyle.Render("⏳ LOBBY — Waiting for players..."))
		parts = append(parts, "   Press [Enter] to start!")
	case game.StatusRunning:
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4444")).Render("🔥 GAME IN PROGRESS"))
	case game.StatusOver:
		if state.Winner != "" {
			if p, ok := state.Players[state.Winner]; ok {
				parts = append(parts, winnerStyle.Render(fmt.Sprintf("🏆 %s WINS!", p.Name)))
			}
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("💀 DRAW — Everyone died!"))
		}
	}
	parts = append(parts, "")

	// Player list
	parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Players:"))
	for _, p := range state.Players {
		colorIdx := p.Color % len(playerColors)
		nameStyle := lipgloss.NewStyle().Foreground(playerColors[colorIdx])

		status := "❤️ "
		if !p.Alive {
			status = "💀"
			nameStyle = deadPlayerStyle
		}

		marker := "  "
		if p.ID == myID {
			marker = "→ "
		}

		line := fmt.Sprintf("%s%s %s [💣×%d 🔥%d]",
			marker,
			status,
			nameStyle.Render(p.Name),
			p.BombMax-p.BombsUsed,
			p.BombRange,
		)
		parts = append(parts, line)
	}

	parts = append(parts, "")
	parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render("WASD/Arrows: Move | Space: Bomb | Q: Quit"))

	return hudBorderStyle.Render(strings.Join(parts, "\n"))
}

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/amalg/go-bomberman/internal/discovery"
	"github.com/amalg/go-bomberman/internal/game"
)

// Color palette
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff8844")).Bold(true)

	menuItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ccccdd")).PaddingLeft(2)

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff8844")).Bold(true)

	menuBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444466")).
			Padding(1, 3)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44aaff")).Bold(true)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#aaaacc"))

	roomStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#ccccdd"))
	roomSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff88")).Bold(true)
	roomEmptyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666688")).Italic(true)

	hardWallStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3a3a3a")).Foreground(lipgloss.Color("#555555"))
	softWallStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#8B6914")).Foreground(lipgloss.Color("#A0772B"))
	emptyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#1a1a2e"))
	bombStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#ff4444")).Bold(true)
	fireStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#ff6600")).Foreground(lipgloss.Color("#ffcc00")).Bold(true)

	playerColors = []lipgloss.Color{
		lipgloss.Color("#00ff88"),
		lipgloss.Color("#4488ff"),
		lipgloss.Color("#ff44ff"),
		lipgloss.Color("#ffff44"),
	}

	deadPlayerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#666666")).Strikethrough(true)
	hudBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#444466")).Padding(0, 1)
	lobbyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#44aaff")).Bold(true)
	winnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff88")).Bold(true).Blink(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4444"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#555566"))
)

func RenderMainMenu(cursor int) string {
	title := titleStyle.Render(`
  ╔══════════════════════════╗
  ║   💣  B O M B E R M A N  ║
  ╚══════════════════════════╝`)

	items := []string{"🎮 Create Room", "🔍 Join Room", "🚪 Quit"}
	var menu []string
	for i, item := range items {
		if i == cursor {
			menu = append(menu, menuSelectedStyle.Render("▸ "+item))
		} else {
			menu = append(menu, menuItemStyle.Render("  "+item))
		}
	}

	content := strings.Join([]string{
		title, "",
		strings.Join(menu, "\n"), "",
		helpStyle.Render("↑↓ Navigate  •  Enter Select"),
	}, "\n")

	return menuBoxStyle.Render(content) + "\n"
}

func RenderCreateRoom(roomName, playerName string, editing int) string {
	fields := []struct{ label, value string }{
		{"Room Name", roomName},
		{"Your Name", playerName},
	}

	var lines []string
	for i, f := range fields {
		label := inputLabelStyle.Render(f.label + ": ")
		value := f.value
		if i == editing {
			value = inputStyle.Render(value + "▌")
			lines = append(lines, menuSelectedStyle.Render("▸ ")+label+value)
		} else {
			value = lipgloss.NewStyle().Foreground(lipgloss.Color("#ccccdd")).Render(value)
			lines = append(lines, "  "+label+value)
		}
	}

	content := strings.Join([]string{
		titleStyle.Render("🎮 Create Room"), "",
		strings.Join(lines, "\n"), "",
		helpStyle.Render("Tab Switch field  •  Enter Create  •  Esc Back"),
	}, "\n")

	return menuBoxStyle.Render(content) + "\n"
}

func RenderBrowseRooms(rooms []discovery.RoomInfo, cursor int, playerName string, editing bool) string {
	var body string
	if editing {
		body = inputLabelStyle.Render("Your Name: ") + inputStyle.Render(playerName+"▌")
	} else if len(rooms) == 0 {
		body = roomEmptyStyle.Render("  Searching for rooms on the network...\n  Make sure someone has created a room.")
	} else {
		var lines []string
		for i, r := range rooms {
			line := fmt.Sprintf("%s's Room \"%s\"  [%d/%d players]",
				r.HostName, r.RoomName, r.PlayerCount, r.MaxPlayers)
			if i == cursor {
				lines = append(lines, roomSelectedStyle.Render("▸ "+line))
			} else {
				lines = append(lines, roomStyle.Render("  "+line))
			}
		}
		body = strings.Join(lines, "\n")
	}

	helpText := "↑↓ Navigate  •  Enter Join  •  Esc Back"
	if editing {
		helpText = "Type your name  •  Enter Confirm  •  Esc Back"
	}

	content := strings.Join([]string{
		titleStyle.Render("🔍 Join Room"), "",
		body, "",
		helpStyle.Render(helpText),
	}, "\n")

	return menuBoxStyle.Render(content) + "\n"
}

func RenderBoard(state *game.GameState, myID string) string {
	if state == nil || len(state.Board) == 0 {
		return "Waiting for game state..."
	}

	fireSet := make(map[game.Position]bool)
	for _, f := range state.Fires {
		fireSet[f.Pos] = true
	}
	bombSet := make(map[game.Position]*game.Bomb)
	for _, b := range state.Bombs {
		bombSet[b.Pos] = b
	}
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
			cells = append(cells, renderCell(state.Board[y][x], pos, fireSet, bombSet, playerSet, myID))
		}
		rows = append(rows, strings.Join(cells, ""))
	}
	return strings.Join(rows, "\n")
}

func renderCell(tile game.TileType, pos game.Position,
	fireSet map[game.Position]bool, bombSet map[game.Position]*game.Bomb,
	playerSet map[game.Position]*game.Player, myID string) string {

	if p, ok := playerSet[pos]; ok {
		colorIdx := p.Color % len(playerColors)
		style := lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Bold(true).
			Foreground(playerColors[colorIdx])
		if p.ID == myID {
			return style.Background(playerColors[colorIdx]).Render("██")
		}
		return style.Render(fmt.Sprintf("P%d", p.Color+1))
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

func RenderHUD(state *game.GameState, myID string) string {
	if state == nil {
		return ""
	}
	var parts []string
	parts = append(parts, titleStyle.Render("💣 BOMBERMAN"), "")

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
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("💀 DRAW"))
		}
	}

	parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Players:"))
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
		parts = append(parts, fmt.Sprintf("%s%s %s [💣×%d 🔥%d]",
			marker, status, nameStyle.Render(p.Name), p.BombMax-p.BombsUsed, p.BombRange))
	}

	parts = append(parts, "", helpStyle.Render("WASD/Arrows: Move | Space: Bomb | Q: Quit"))
	return hudBorderStyle.Render(strings.Join(parts, "\n"))
}

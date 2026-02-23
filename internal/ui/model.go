package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amalg/go-bomberman/internal/game"
	"github.com/amalg/go-bomberman/internal/network"
)

// stateUpdateMsg carries a new game state from the network client.
type stateUpdateMsg game.GameState

// errMsg carries an error.
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Model is the Bubbletea model for the game client.
type Model struct {
	client   *network.Client
	state    *game.GameState
	playerID string
	err      error
	quitting bool
}

// NewModel creates a new TUI model connected to the given network client.
func NewModel(client *network.Client) Model {
	return Model{
		client:   client,
		playerID: client.PlayerID(),
	}
}

// Init starts listening for state updates from the server.
func (m Model) Init() tea.Cmd {
	return waitForState(m.client)
}

// Update handles incoming messages (key presses, state updates).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case stateUpdateMsg:
		state := game.GameState(msg)
		m.state = &state
		return m, waitForState(m.client)

	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

// View renders the current game state.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye! 👋\n"
	}

	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff4444")).
			Render("Error: "+m.err.Error()) + "\n"
	}

	board := RenderBoard(m.state, m.playerID)
	hud := RenderHUD(m.state, m.playerID)

	// Layout: board on the left, HUD on the right
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		board,
		"  ",
		hud,
	) + "\n"
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit

	case "up", "w":
		m.client.SendAction(game.ActionMove, game.DirUp)
	case "down", "s":
		m.client.SendAction(game.ActionMove, game.DirDown)
	case "left", "a":
		m.client.SendAction(game.ActionMove, game.DirLeft)
	case "right", "d":
		m.client.SendAction(game.ActionMove, game.DirRight)
	case " ":
		m.client.SendAction(game.ActionPlaceBomb, 0)
	case "enter":
		m.client.SendStart()
	}

	return m, nil
}

// waitForState returns a Cmd that waits for the next state update from the server.
func waitForState(client *network.Client) tea.Cmd {
	return func() tea.Msg {
		state, ok := <-client.StateChan()
		if !ok {
			return errMsg{err: fmt.Errorf("server connection closed")}
		}
		return stateUpdateMsg(state)
	}
}

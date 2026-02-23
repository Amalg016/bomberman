package ui

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amalg/go-bomberman/internal/discovery"
	"github.com/amalg/go-bomberman/internal/game"
	"github.com/amalg/go-bomberman/internal/network"
)

// Screen represents which screen is currently shown.
type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenCreateRoom
	ScreenBrowseRooms
	ScreenGame
)

// --- Messages ---

type stateUpdateMsg game.GameState
type roomsUpdateMsg []discovery.RoomInfo
type errMsg struct{ err error }
type serverReadyMsg struct {
	server *network.Server
	client *network.Client
	bc     *discovery.Broadcaster
}
type clientConnectedMsg struct {
	client *network.Client
}
type tickMsg time.Time

func (e errMsg) Error() string { return e.err.Error() }

// --- Model ---

type Model struct {
	screen     Screen
	playerName string
	port       int

	// Main menu
	menuCursor int

	// Create room
	roomName    string
	createField int

	// Browse rooms
	listener       *discovery.Listener
	rooms          []discovery.RoomInfo
	roomCursor     int
	browseEditName bool

	// Game
	server   *network.Server
	client   *network.Client
	bc       *discovery.Broadcaster
	state    *game.GameState
	playerID string
	isHost   bool

	err      error
	quitting bool
}

func NewModel(playerName string, port int) Model {
	if playerName == "" {
		playerName = "Player"
	}
	return Model{
		screen:     ScreenMainMenu,
		playerName: playerName,
		port:       port,
		roomName:   "Bomberman",
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		return m, nil

	case serverReadyMsg:
		m.server = msg.server
		m.client = msg.client
		m.bc = msg.bc
		m.playerID = msg.client.PlayerID()
		m.isHost = true
		m.screen = ScreenGame
		return m, waitForState(m.client)

	case clientConnectedMsg:
		m.client = msg.client
		m.playerID = msg.client.PlayerID()
		m.isHost = false
		m.screen = ScreenGame
		if m.listener != nil {
			m.listener.Stop()
			m.listener = nil
		}
		return m, waitForState(m.client)

	case stateUpdateMsg:
		state := game.GameState(msg)
		m.state = &state
		if m.bc != nil {
			m.bc.UpdatePlayerCount(len(state.Players))
		}
		return m, waitForState(m.client)

	case roomsUpdateMsg:
		m.rooms = []discovery.RoomInfo(msg)
		if m.screen == ScreenBrowseRooms {
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			})
		}
		return m, nil

	case tickMsg:
		if m.screen == ScreenBrowseRooms && m.listener != nil && !m.browseEditName {
			return m, refreshRooms(m.listener)
		}
		return m, nil
	}

	switch m.screen {
	case ScreenMainMenu:
		return m.updateMainMenu(msg)
	case ScreenCreateRoom:
		return m.updateCreateRoom(msg)
	case ScreenBrowseRooms:
		return m.updateBrowseRooms(msg)
	case ScreenGame:
		return m.updateGame(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Goodbye! 👋\n"
	}

	var view string
	switch m.screen {
	case ScreenMainMenu:
		view = RenderMainMenu(m.menuCursor)
	case ScreenCreateRoom:
		view = RenderCreateRoom(m.roomName, m.playerName, m.createField)
	case ScreenBrowseRooms:
		view = RenderBrowseRooms(m.rooms, m.roomCursor, m.playerName, m.browseEditName)
	case ScreenGame:
		board := RenderBoard(m.state, m.playerID)
		hud := RenderHUD(m.state, m.playerID)
		view = lipgloss.JoinHorizontal(lipgloss.Top, board, "  ", hud)
	}

	if m.err != nil {
		view += "\n" + errorStyle.Render("Error: "+m.err.Error())
	}
	return view + "\n"
}

// --- Screen handlers ---

func (m Model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < 2 {
				m.menuCursor++
			}
		case "enter":
			switch m.menuCursor {
			case 0:
				m.screen = ScreenCreateRoom
				m.createField = 0
				m.err = nil
			case 1:
				m.screen = ScreenBrowseRooms
				m.browseEditName = true
				m.roomCursor = 0
				m.err = nil
			case 2:
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) updateCreateRoom(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.screen = ScreenMainMenu
			m.err = nil
			return m, nil
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			m.createField = (m.createField + 1) % 2
		case "enter":
			if m.roomName == "" {
				m.roomName = "Bomberman"
			}
			if m.playerName == "" {
				m.playerName = "Host"
			}
			return m, startServer(m.roomName, m.playerName, m.port)
		case "backspace":
			if m.createField == 0 && len(m.roomName) > 0 {
				m.roomName = m.roomName[:len(m.roomName)-1]
			} else if m.createField == 1 && len(m.playerName) > 0 {
				m.playerName = m.playerName[:len(m.playerName)-1]
			}
		default:
			ch := keyMsg.String()
			if len(ch) == 1 {
				if m.createField == 0 {
					m.roomName += ch
				} else {
					m.playerName += ch
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateBrowseRooms(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.browseEditName {
			switch keyMsg.String() {
			case "esc":
				m.screen = ScreenMainMenu
				if m.listener != nil {
					m.listener.Stop()
					m.listener = nil
				}
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				if m.playerName == "" {
					m.playerName = "Player"
				}
				m.browseEditName = false
				m.listener = discovery.NewListener()
				if err := m.listener.Start(); err != nil {
					m.err = err
					return m, nil
				}
				return m, refreshRooms(m.listener)
			case "backspace":
				if len(m.playerName) > 0 {
					m.playerName = m.playerName[:len(m.playerName)-1]
				}
			default:
				ch := keyMsg.String()
				if len(ch) == 1 {
					m.playerName += ch
				}
			}
			return m, nil
		}

		switch keyMsg.String() {
		case "esc":
			m.screen = ScreenMainMenu
			if m.listener != nil {
				m.listener.Stop()
				m.listener = nil
			}
			m.err = nil
			return m, nil
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.roomCursor > 0 {
				m.roomCursor--
			}
		case "down", "j":
			if m.roomCursor < len(m.rooms)-1 {
				m.roomCursor++
			}
		case "enter":
			if len(m.rooms) > 0 && m.roomCursor < len(m.rooms) {
				room := m.rooms[m.roomCursor]
				return m, connectToRoom(room.GameAddr, m.playerName)
			}
		}
	}
	return m, nil
}

func (m Model) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c", "esc":
			m.cleanup()
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
			if m.client != nil {
				m.client.SendStart()
			}
		}
	}
	return m, nil
}

func (m *Model) cleanup() {
	if m.bc != nil {
		m.bc.Stop()
	}
	if m.client != nil {
		m.client.Close()
	}
	if m.server != nil {
		m.server.Stop()
	}
	if m.listener != nil {
		m.listener.Stop()
	}
}

// --- Commands ---

func waitForState(client *network.Client) tea.Cmd {
	return func() tea.Msg {
		state, ok := <-client.StateChan()
		if !ok {
			return errMsg{err: fmt.Errorf("server connection closed")}
		}
		return stateUpdateMsg(state)
	}
}

func refreshRooms(listener *discovery.Listener) tea.Cmd {
	return func() tea.Msg {
		return roomsUpdateMsg(listener.Rooms())
	}
}

func startServer(roomName, playerName string, port int) tea.Cmd {
	return func() tea.Msg {
		log.SetOutput(io.Discard)

		config := game.DefaultConfig()
		addr := fmt.Sprintf("0.0.0.0:%d", port)

		server := network.NewServer(addr, config)
		if err := server.Start(); err != nil {
			return errMsg{err: fmt.Errorf("start server: %w", err)}
		}

		time.Sleep(200 * time.Millisecond)

		clientAddr := fmt.Sprintf("127.0.0.1:%d", port)
		client, err := network.NewClient(clientAddr, playerName)
		if err != nil {
			server.Stop()
			return errMsg{err: fmt.Errorf("connect as host: %w", err)}
		}

		gameAddr := fmt.Sprintf("%s:%d", getLocalIP(), port)
		bc := discovery.NewBroadcaster(discovery.RoomInfo{
			RoomName:    roomName,
			HostName:    playerName,
			PlayerCount: 1,
			MaxPlayers:  config.MaxPlayers,
			GameAddr:    gameAddr,
		})
		bc.Start()

		return serverReadyMsg{server: server, client: client, bc: bc}
	}
}

func connectToRoom(addr, playerName string) tea.Cmd {
	return func() tea.Msg {
		client, err := network.NewClient(addr, playerName)
		if err != nil {
			return errMsg{err: fmt.Errorf("join room: %w", err)}
		}
		return clientConnectedMsg{client: client}
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

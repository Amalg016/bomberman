package network

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/amalg/go-bomberman/internal/game"
)

// Client connects to a game server and provides methods to send actions
// and receive state updates.
type Client struct {
	conn     net.Conn
	playerID string
	config   game.GameConfig
	stateCh  chan game.GameState
	done     chan struct{}
	mu       sync.Mutex
}

// NewClient creates a new client and connects to the server.
func NewClient(addr, name string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}

	c := &Client{
		conn:    conn,
		stateCh: make(chan game.GameState, 10),
		done:    make(chan struct{}),
	}

	// Send join message
	if err := Encode(conn, MsgJoin, JoinMsg{Name: name}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send join: %w", err)
	}

	// Read welcome message
	env, err := Decode(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read welcome: %w", err)
	}

	if env.Type == MsgError {
		var errMsg ErrorMsg
		DecodePayload(env, &errMsg)
		conn.Close()
		return nil, fmt.Errorf("server error: %s", errMsg.Message)
	}

	if env.Type != MsgWelcome {
		conn.Close()
		return nil, fmt.Errorf("expected welcome, got %s", env.Type)
	}

	var welcome WelcomeMsg
	if err := DecodePayload(env, &welcome); err != nil {
		conn.Close()
		return nil, fmt.Errorf("decode welcome: %w", err)
	}

	c.playerID = welcome.PlayerID
	c.config = welcome.Config

	// Start receiving state updates
	go c.receiveLoop()

	return c, nil
}

// PlayerID returns the client's assigned player ID.
func (c *Client) PlayerID() string {
	return c.playerID
}

// Config returns the game configuration received from the server.
func (c *Client) Config() game.GameConfig {
	return c.config
}

// StateChan returns a channel that yields game state updates.
func (c *Client) StateChan() <-chan game.GameState {
	return c.stateCh
}

// SendAction sends a player action to the server.
func (c *Client) SendAction(actionType game.ActionType, dir game.Direction) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return Encode(c.conn, MsgAction, ActionMsg{
		ActionType: actionType,
		Direction:  dir,
	})
}

// SendStart requests the server to start the game.
func (c *Client) SendStart() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return Encode(c.conn, MsgStart, struct{}{})
}

// Close disconnects from the server.
func (c *Client) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.conn.Close()
}

func (c *Client) receiveLoop() {
	defer close(c.stateCh)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		env, err := Decode(c.conn)
		if err != nil {
			return
		}

		switch env.Type {
		case MsgState:
			var stateMsg StateMsg
			if err := DecodePayload(env, &stateMsg); err != nil {
				continue
			}
			// Non-blocking send to state channel
			select {
			case c.stateCh <- stateMsg.State:
			default:
				// Drop old state if consumer is slow — latest state matters most
				select {
				case <-c.stateCh:
				default:
				}
				c.stateCh <- stateMsg.State
			}
		case MsgError:
			var errMsg ErrorMsg
			DecodePayload(env, &errMsg)
			// Could surface this to the TUI in the future
		}
	}
}

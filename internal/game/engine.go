package game

import (
	"fmt"
	"sync"
	"time"
)

// Engine is the authoritative game loop that processes all game logic.
type Engine struct {
	State   *GameState
	Config  GameConfig
	actions chan Action
	done    chan struct{}
	mu      sync.Mutex
	onTick  func(GameState) // Callback after each tick with a COPY of state
}

// NewEngine creates a new game engine with the given config.
func NewEngine(config GameConfig) *Engine {
	state := &GameState{
		Board:   NewBoard(config),
		Players: make(map[string]*Player),
		Bombs:   make([]*Bomb, 0),
		Fires:   make([]Fire, 0),
		Width:   config.Width,
		Height:  config.Height,
		Status:  StatusLobby,
	}

	return &Engine{
		State:   state,
		Config:  config,
		actions: make(chan Action, 256),
		done:    make(chan struct{}),
	}
}

// OnTick sets a callback that is invoked after every game tick with a copy of the state.
// Used by the network server to broadcast state to clients.
func (e *Engine) OnTick(fn func(GameState)) {
	e.onTick = fn
}

// Run starts the game loop at the configured tick rate.
// This blocks until Stop() is called.
func (e *Engine) Run() {
	ticker := time.NewTicker(time.Second / time.Duration(e.Config.TickRate))
	defer ticker.Stop()

	for {
		select {
		case <-e.done:
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

// Stop halts the game loop.
func (e *Engine) Stop() {
	close(e.done)
}

// EnqueueAction sends a player action to be processed on the next tick.
func (e *Engine) EnqueueAction(a Action) {
	select {
	case e.actions <- a:
	default:
		// Drop action if buffer is full (prevents blocking)
	}
}

// AddPlayer adds a new player to the game.
// Returns an error if the game is full or already running.
func (e *Engine) AddPlayer(id, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State.Status == StatusRunning {
		return fmt.Errorf("game already in progress")
	}
	if len(e.State.Players) >= e.Config.MaxPlayers {
		return fmt.Errorf("game is full (%d/%d players)", len(e.State.Players), e.Config.MaxPlayers)
	}
	if _, exists := e.State.Players[id]; exists {
		return fmt.Errorf("player %s already exists", id)
	}

	spawns := SpawnPositions(e.Config.Width, e.Config.Height)
	spawnIdx := len(e.State.Players)
	if spawnIdx >= len(spawns) {
		spawnIdx = spawnIdx % len(spawns)
	}

	e.State.Players[id] = &Player{
		ID:        id,
		Name:      name,
		Pos:       spawns[spawnIdx],
		Alive:     true,
		BombMax:   1,
		BombRange: 2,
		BombsUsed: 0,
		Color:     spawnIdx,
	}
	return nil
}

// RemovePlayer removes a player from the game.
func (e *Engine) RemovePlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.State.Players, id)
}

// StartGame transitions the game from lobby to running.
func (e *Engine) StartGame() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.State.Players) < 1 {
		return fmt.Errorf("need at least 1 player to start")
	}
	e.State.Status = StatusRunning
	return nil
}

// tick processes one game tick: drain actions, update bombs, clear fires, check win.
// IMPORTANT: We copy the state while holding the lock, then release the lock
// BEFORE calling onTick to avoid deadlock (onTick may call back into the engine).
func (e *Engine) tick() {
	e.mu.Lock()

	if e.State.Status == StatusRunning {
		// Process game logic while holding the lock
		e.drainActions()
		e.tickBombs()
		e.clearExpiredFires()
		e.checkWinCondition()
	}

	// Copy state while still holding the lock
	stateCopy := e.copyStateLocked()

	// Release lock BEFORE calling the callback
	e.mu.Unlock()

	// Broadcast the copy — safe, no lock held
	if e.onTick != nil {
		e.onTick(stateCopy)
	}
}

// drainActions processes all queued player actions.
func (e *Engine) drainActions() {
	for {
		select {
		case a := <-e.actions:
			switch a.Type {
			case ActionMove:
				e.movePlayer(a.PlayerID, a.Dir)
			case ActionPlaceBomb:
				e.placeBomb(a.PlayerID)
			}
		default:
			return
		}
	}
}

// checkWinCondition checks if the game is over.
func (e *Engine) checkWinCondition() {
	if e.State.Status != StatusRunning {
		return
	}

	alive := make([]*Player, 0)
	for _, p := range e.State.Players {
		if p.Alive {
			alive = append(alive, p)
		}
	}

	switch len(alive) {
	case 0:
		// Draw — everyone died simultaneously
		e.State.Status = StatusOver
		e.State.Winner = ""
	case 1:
		// We have a winner, but only if there were multiple players
		if len(e.State.Players) > 1 {
			e.State.Status = StatusOver
			e.State.Winner = alive[0].ID
		}
	}
}

// GetStateCopy returns a deep copy of the game state safe for serialization.
func (e *Engine) GetStateCopy() GameState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.copyStateLocked()
}

// copyStateLocked creates a deep copy of the game state.
// MUST be called while e.mu is held.
func (e *Engine) copyStateLocked() GameState {
	// Copy board
	boardCopy := make([][]TileType, e.State.Height)
	for y := range boardCopy {
		boardCopy[y] = make([]TileType, e.State.Width)
		copy(boardCopy[y], e.State.Board[y])
	}

	// Copy players
	playersCopy := make(map[string]*Player, len(e.State.Players))
	for id, p := range e.State.Players {
		cp := *p
		playersCopy[id] = &cp
	}

	// Copy bombs
	bombsCopy := make([]*Bomb, len(e.State.Bombs))
	for i, b := range e.State.Bombs {
		cb := *b
		bombsCopy[i] = &cb
	}

	// Copy fires
	firesCopy := make([]Fire, len(e.State.Fires))
	copy(firesCopy, e.State.Fires)

	return GameState{
		Board:   boardCopy,
		Players: playersCopy,
		Bombs:   bombsCopy,
		Fires:   firesCopy,
		Width:   e.State.Width,
		Height:  e.State.Height,
		Status:  e.State.Status,
		Winner:  e.State.Winner,
	}
}

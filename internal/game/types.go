package game

import (
	"time"
)

// TileType represents the type of a cell on the game board.
type TileType int

const (
	Empty    TileType = iota
	HardWall          // Indestructible
	SoftWall          // Destructible by bombs
)

// Direction represents a movement direction.
type Direction int

const (
	DirUp Direction = iota
	DirDown
	DirLeft
	DirRight
)

// ActionType represents the type of player action.
type ActionType int

const (
	ActionMove ActionType = iota
	ActionPlaceBomb
)

// Action represents a player's input action.
type Action struct {
	PlayerID string
	Type     ActionType
	Dir      Direction // Only relevant for ActionMove
}

// Position represents a coordinate on the board.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Player represents a connected player.
type Player struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Pos       Position `json:"pos"`
	Alive     bool     `json:"alive"`
	BombMax   int      `json:"bomb_max"`   // Max simultaneous bombs
	BombRange int      `json:"bomb_range"` // Explosion range in tiles
	BombsUsed int      `json:"bombs_used"` // Currently active bombs
	Color     int      `json:"color"`      // Player color index (0-3)
}

// Bomb represents an active bomb on the board.
type Bomb struct {
	OwnerID   string    `json:"owner_id"`
	Pos       Position  `json:"pos"`
	Range     int       `json:"range"`
	PlacedAt  time.Time `json:"placed_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Fire represents an active fire tile from an explosion.
type Fire struct {
	Pos       Position  `json:"pos"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GameStatus represents the current game phase.
type GameStatus int

const (
	StatusLobby   GameStatus = iota // Waiting for players
	StatusRunning                   // Game in progress
	StatusOver                      // Game finished
)

// GameState is the authoritative state of the game, owned by the server.
// Concurrency protection is handled by the Engine's mutex, not by this struct.
type GameState struct {
	Board   [][]TileType       `json:"board"`
	Players map[string]*Player `json:"players"`
	Bombs   []*Bomb            `json:"bombs"`
	Fires   []Fire             `json:"fires"`
	Width   int                `json:"width"`
	Height  int                `json:"height"`
	Status  GameStatus         `json:"status"`
	Winner  string             `json:"winner,omitempty"`
}

// GameConfig holds configurable parameters for a game session.
type GameConfig struct {
	Width           int           `json:"width"`
	Height          int           `json:"height"`
	BombTimer       time.Duration `json:"bomb_timer"`
	FireDuration    time.Duration `json:"fire_duration"`
	TickRate        int           `json:"tick_rate"` // Ticks per second
	MaxPlayers      int           `json:"max_players"`
	SoftWallDensity float64       `json:"soft_wall_density"` // 0.0 to 1.0
}

// DefaultConfig returns a sensible default game configuration.
func DefaultConfig() GameConfig {
	return GameConfig{
		Width:           15,
		Height:          13,
		BombTimer:       3 * time.Second,
		FireDuration:    500 * time.Millisecond,
		TickRate:        20,
		MaxPlayers:      4,
		SoftWallDensity: 0.4,
	}
}

// SpawnPositions returns the corner spawn positions for players.
// These corners and their adjacent tiles are kept clear of soft walls.
func SpawnPositions(width, height int) []Position {
	return []Position{
		{X: 1, Y: 1},                  // Top-left
		{X: width - 2, Y: 1},          // Top-right
		{X: 1, Y: height - 2},         // Bottom-left
		{X: width - 2, Y: height - 2}, // Bottom-right
	}
}

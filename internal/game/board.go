package game

import (
	"math/rand"
)

// NewBoard generates a classic Bomberman grid layout.
//
// Layout rules:
//   - Border is all HardWall
//   - HardWall at every position where both X and Y are even
//   - Random SoftWall fill at the given density
//   - Player spawn corners (and their adjacent 2 tiles) are kept clear
func NewBoard(config GameConfig) [][]TileType {
	board := make([][]TileType, config.Height)
	for y := 0; y < config.Height; y++ {
		board[y] = make([]TileType, config.Width)
		for x := 0; x < config.Width; x++ {
			switch {
			case x == 0 || y == 0 || x == config.Width-1 || y == config.Height-1:
				// Border walls
				board[y][x] = HardWall
			case x%2 == 0 && y%2 == 0:
				// Interior pillar pattern
				board[y][x] = HardWall
			default:
				board[y][x] = Empty
			}
		}
	}

	// Determine safe zones around spawn positions
	spawns := SpawnPositions(config.Width, config.Height)
	safeSet := makeSafeSet(spawns)

	// Fill soft walls randomly, avoiding safe zones
	for y := 1; y < config.Height-1; y++ {
		for x := 1; x < config.Width-1; x++ {
			if board[y][x] != Empty {
				continue
			}
			pos := Position{X: x, Y: y}
			if safeSet[pos] {
				continue
			}
			if rand.Float64() < config.SoftWallDensity {
				board[y][x] = SoftWall
			}
		}
	}

	return board
}

// makeSafeSet returns a set of positions that must remain clear for player spawning.
// Each spawn corner gets 3 clear tiles: the spawn position plus the two adjacent positions.
func makeSafeSet(spawns []Position) map[Position]bool {
	safe := make(map[Position]bool)
	for _, sp := range spawns {
		safe[sp] = true
		// Adjacent tiles in both axes (L-shaped clear zone)
		safe[Position{X: sp.X + 1, Y: sp.Y}] = true
		safe[Position{X: sp.X, Y: sp.Y + 1}] = true
		safe[Position{X: sp.X - 1, Y: sp.Y}] = true
		safe[Position{X: sp.X, Y: sp.Y - 1}] = true
	}
	return safe
}

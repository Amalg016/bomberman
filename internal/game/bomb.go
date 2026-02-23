package game

import "time"

// placeBomb places a bomb at the player's current position.
func (e *Engine) placeBomb(playerID string) {
	p, ok := e.State.Players[playerID]
	if !ok || !p.Alive {
		return
	}

	// Check bomb limit
	if p.BombsUsed >= p.BombMax {
		return
	}

	// Check if bomb already exists at this position
	for _, b := range e.State.Bombs {
		if b.Pos == p.Pos {
			return
		}
	}

	now := time.Now()
	bomb := &Bomb{
		OwnerID:   playerID,
		Pos:       p.Pos,
		Range:     p.BombRange,
		PlacedAt:  now,
		ExpiresAt: now.Add(e.Config.BombTimer),
	}

	e.State.Bombs = append(e.State.Bombs, bomb)
	p.BombsUsed++
}

// tickBombs checks all active bombs and detonates any whose timer has expired.
func (e *Engine) tickBombs() {
	now := time.Now()
	detonated := make(map[int]bool)

	// First pass: find bombs that need to detonate
	for i, b := range e.State.Bombs {
		if now.After(b.ExpiresAt) {
			detonated[i] = true
		}
	}

	// Explode all detonated bombs (may chain-react to more)
	for i := range detonated {
		e.explode(e.State.Bombs[i], detonated)
	}

	// Remove detonated bombs and return bomb count to owners
	remaining := make([]*Bomb, 0, len(e.State.Bombs))
	for i, b := range e.State.Bombs {
		if detonated[i] {
			if p, ok := e.State.Players[b.OwnerID]; ok {
				p.BombsUsed--
			}
		} else {
			remaining = append(remaining, b)
		}
	}
	e.State.Bombs = remaining
}

// explode processes a bomb explosion in the 4 cardinal directions.
// It can trigger chain reactions on other bombs.
func (e *Engine) explode(bomb *Bomb, detonated map[int]bool) {
	now := time.Now()
	fireExpiry := now.Add(e.Config.FireDuration)

	// Fire at bomb center
	e.State.Fires = append(e.State.Fires, Fire{
		Pos:       bomb.Pos,
		ExpiresAt: fireExpiry,
	})

	// Expand in 4 directions
	dirs := []Position{
		{X: 0, Y: -1}, // Up
		{X: 0, Y: 1},  // Down
		{X: -1, Y: 0}, // Left
		{X: 1, Y: 0},  // Right
	}

	for _, d := range dirs {
		for dist := 1; dist <= bomb.Range; dist++ {
			pos := Position{
				X: bomb.Pos.X + d.X*dist,
				Y: bomb.Pos.Y + d.Y*dist,
			}

			// Out of bounds
			if pos.X < 0 || pos.X >= e.State.Width ||
				pos.Y < 0 || pos.Y >= e.State.Height {
				break
			}

			tile := e.State.Board[pos.Y][pos.X]

			// Hard wall stops explosion completely
			if tile == HardWall {
				break
			}

			// Soft wall: destroy it, place fire, but stop further expansion
			if tile == SoftWall {
				e.State.Board[pos.Y][pos.X] = Empty
				e.State.Fires = append(e.State.Fires, Fire{
					Pos:       pos,
					ExpiresAt: fireExpiry,
				})
				break
			}

			// Place fire on empty tile
			e.State.Fires = append(e.State.Fires, Fire{
				Pos:       pos,
				ExpiresAt: fireExpiry,
			})

			// Chain reaction: if fire hits another bomb, detonate it immediately
			for i, otherBomb := range e.State.Bombs {
				if otherBomb.Pos == pos && !detonated[i] {
					detonated[i] = true
					e.explode(otherBomb, detonated)
				}
			}
		}
	}

	// Damage players caught in fire (including the bomb center)
	e.damagePlayersInFire()
}

// damagePlayersInFire kills any alive player standing on a fire tile.
func (e *Engine) damagePlayersInFire() {
	fireSet := make(map[Position]bool, len(e.State.Fires))
	for _, f := range e.State.Fires {
		fireSet[f.Pos] = true
	}

	for _, p := range e.State.Players {
		if p.Alive && fireSet[p.Pos] {
			p.Alive = false
		}
	}
}

// clearExpiredFires removes fire tiles that have expired.
func (e *Engine) clearExpiredFires() {
	now := time.Now()
	remaining := make([]Fire, 0, len(e.State.Fires))
	for _, f := range e.State.Fires {
		if now.Before(f.ExpiresAt) {
			remaining = append(remaining, f)
		}
	}
	e.State.Fires = remaining
}

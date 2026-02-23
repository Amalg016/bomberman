package game

// movePlayer attempts to move a player in the given direction.
// Movement is blocked by hard walls, soft walls, bombs, and board edges.
func (e *Engine) movePlayer(playerID string, dir Direction) {
	p, ok := e.State.Players[playerID]
	if !ok || !p.Alive {
		return
	}

	newPos := p.Pos
	switch dir {
	case DirUp:
		newPos.Y--
	case DirDown:
		newPos.Y++
	case DirLeft:
		newPos.X--
	case DirRight:
		newPos.X++
	}

	// Bounds check
	if newPos.X < 0 || newPos.X >= e.State.Width ||
		newPos.Y < 0 || newPos.Y >= e.State.Height {
		return
	}

	// Wall collision
	tile := e.State.Board[newPos.Y][newPos.X]
	if tile == HardWall || tile == SoftWall {
		return
	}

	// Bomb collision — players can't walk through bombs
	// (except the bomb they just placed, which is handled by standing on it)
	for _, b := range e.State.Bombs {
		if b.Pos == newPos {
			return
		}
	}

	p.Pos = newPos

	// Check if player walked into fire
	for _, f := range e.State.Fires {
		if f.Pos == newPos {
			p.Alive = false
			return
		}
	}
}

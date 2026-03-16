package game

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	// enemyMoveInterval controls how often enemies move (in ticks).
	// At 20 ticks/sec, 5 ticks = 0.25s per move — slightly slower than a player.
	enemyMoveInterval = 5

	// chaseChance is the probability an enemy will chase the nearest player
	// instead of wandering randomly (0.0–1.0).
	chaseChance = 0.65

	// bombFleeRadius is the Manhattan distance within which enemies try to avoid bombs.
	bombFleeRadius = 3
)

// spawnEnemies places enemies on empty tiles in the interior of the board.
// Avoids the 3x3 safe zones around player spawn corners.
func (e *Engine) spawnEnemies() {
	spawns := SpawnPositions(e.Config.Width, e.Config.Height)
	safeSet := makeSafeSet(spawns)

	// Collect all candidate positions (empty tiles not in safe zones)
	var candidates []Position
	for y := 1; y < e.State.Height-1; y++ {
		for x := 1; x < e.State.Width-1; x++ {
			pos := Position{X: x, Y: y}
			if e.State.Board[y][x] == Empty && !safeSet[pos] {
				candidates = append(candidates, pos)
			}
		}
	}

	// Shuffle and pick up to EnemyCount positions
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	count := e.Config.EnemyCount
	if count > len(candidates) {
		count = len(candidates)
	}

	for i := 0; i < count; i++ {
		enemy := &Enemy{
			ID:        fmt.Sprintf("enemy_%d", i),
			Pos:       candidates[i],
			Alive:     true,
			Dir:       Direction(rand.Intn(4)),
			MoveTimer: rand.Intn(enemyMoveInterval), // stagger start times
		}
		e.State.Enemies = append(e.State.Enemies, enemy)
	}
}

// tickEnemies updates all alive enemies: move them and check player kills.
func (e *Engine) tickEnemies() {
	// Pre-compute danger map once per tick for all enemies to use
	dangerSet := e.buildDangerSet()

	for _, enemy := range e.State.Enemies {
		if !enemy.Alive {
			continue
		}
		e.tickSingleEnemy(enemy, dangerSet)
	}

	// After all enemies moved, check if any enemy occupies the same tile as a player
	e.checkEnemyPlayerCollisions()
}

// buildDangerSet returns positions that enemies should avoid (fire tiles + bomb blast zones).
func (e *Engine) buildDangerSet() map[Position]bool {
	danger := make(map[Position]bool)

	// Current fire tiles are dangerous
	for _, f := range e.State.Fires {
		danger[f.Pos] = true
	}

	// Bomb blast zones: for each bomb, mark the cross pattern as dangerous
	for _, b := range e.State.Bombs {
		// Only worry about bombs that will explode soon (within 2 seconds)
		if time.Until(b.ExpiresAt) > 2*time.Second {
			continue
		}
		danger[b.Pos] = true
		dirs := []Position{
			{X: 0, Y: -1}, {X: 0, Y: 1},
			{X: -1, Y: 0}, {X: 1, Y: 0},
		}
		for _, d := range dirs {
			for dist := 1; dist <= b.Range; dist++ {
				pos := Position{
					X: b.Pos.X + d.X*dist,
					Y: b.Pos.Y + d.Y*dist,
				}
				if pos.X < 0 || pos.X >= e.State.Width ||
					pos.Y < 0 || pos.Y >= e.State.Height {
					break
				}
				tile := e.State.Board[pos.Y][pos.X]
				if tile == HardWall {
					break
				}
				danger[pos] = true
				if tile == SoftWall {
					break
				}
			}
		}
	}

	return danger
}

// tickSingleEnemy handles the AI for one enemy per tick.
//
// Behavior priority:
//  1. FLEE: If currently on or adjacent to a danger zone, move away from it.
//  2. CHASE: With chaseChance probability, move toward the nearest alive player.
//  3. WANDER: Otherwise, prefer current direction (momentum) or pick randomly.
func (e *Engine) tickSingleEnemy(enemy *Enemy, dangerSet map[Position]bool) {
	enemy.MoveTimer++
	if enemy.MoveTimer < enemyMoveInterval {
		return
	}
	enemy.MoveTimer = 0

	validDirs := e.getValidDirections(enemy)
	if len(validDirs) == 0 {
		return // completely stuck
	}

	// --- Priority 1: Flee from danger ---
	if dangerSet[enemy.Pos] {
		dir, ok := e.pickFleeDirection(enemy, validDirs, dangerSet)
		if ok {
			e.moveEnemy(enemy, dir)
			return
		}
		// Can't flee — fall through and try something
	}

	// Filter out directions that lead into danger
	safeDirs := make([]Direction, 0, len(validDirs))
	for _, d := range validDirs {
		target := applyDirection(enemy.Pos, d)
		if !dangerSet[target] {
			safeDirs = append(safeDirs, d)
		}
	}
	if len(safeDirs) == 0 {
		safeDirs = validDirs // no safe options, take any path
	}

	// --- Priority 2: Chase nearest player ---
	if rand.Float64() < chaseChance {
		dir, ok := e.pickChaseDirection(enemy, safeDirs)
		if ok {
			e.moveEnemy(enemy, dir)
			return
		}
	}

	// --- Priority 3: Wander with momentum ---
	dir := e.pickWanderDirection(enemy, safeDirs)
	e.moveEnemy(enemy, dir)
}

// getValidDirections returns all directions an enemy can physically move.
func (e *Engine) getValidDirections(enemy *Enemy) []Direction {
	allDirs := []Direction{DirUp, DirDown, DirLeft, DirRight}
	valid := make([]Direction, 0, 4)

	for _, dir := range allDirs {
		newPos := applyDirection(enemy.Pos, dir)

		// Bounds check
		if newPos.X < 0 || newPos.X >= e.State.Width ||
			newPos.Y < 0 || newPos.Y >= e.State.Height {
			continue
		}

		// Wall collision
		tile := e.State.Board[newPos.Y][newPos.X]
		if tile == HardWall || tile == SoftWall {
			continue
		}

		// Bomb collision
		blocked := false
		for _, b := range e.State.Bombs {
			if b.Pos == newPos {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		valid = append(valid, dir)
	}
	return valid
}

// pickFleeDirection finds the best direction to escape danger.
// It picks the valid direction that maximizes distance from dangerous tiles.
func (e *Engine) pickFleeDirection(enemy *Enemy, validDirs []Direction, dangerSet map[Position]bool) (Direction, bool) {
	var bestDir Direction
	bestScore := -1

	for _, dir := range validDirs {
		target := applyDirection(enemy.Pos, dir)
		if dangerSet[target] {
			continue // don't flee INTO danger
		}
		// Score by how far the nearest danger tile is from the target
		score := e.minDangerDistance(target, dangerSet)
		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}

	return bestDir, bestScore > -1
}

// minDangerDistance returns the minimum Manhattan distance from pos to any danger tile.
func (e *Engine) minDangerDistance(pos Position, dangerSet map[Position]bool) int {
	minDist := math.MaxInt32
	for dp := range dangerSet {
		dist := abs(pos.X-dp.X) + abs(pos.Y-dp.Y)
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

// pickChaseDirection finds the direction that moves the enemy closest to the nearest alive player.
func (e *Engine) pickChaseDirection(enemy *Enemy, dirs []Direction) (Direction, bool) {
	// Find nearest alive player
	var nearest *Player
	nearestDist := math.MaxInt32
	for _, p := range e.State.Players {
		if !p.Alive {
			continue
		}
		dist := abs(enemy.Pos.X-p.Pos.X) + abs(enemy.Pos.Y-p.Pos.Y)
		if dist < nearestDist {
			nearestDist = dist
			nearest = p
		}
	}
	if nearest == nil {
		return DirUp, false // no alive players
	}

	// Pick the direction that minimizes distance to that player
	bestDir := dirs[0]
	bestDist := math.MaxInt32
	for _, dir := range dirs {
		target := applyDirection(enemy.Pos, dir)
		dist := abs(target.X-nearest.Pos.X) + abs(target.Y-nearest.Pos.Y)
		if dist < bestDist {
			bestDist = dist
			bestDir = dir
		}
	}
	return bestDir, true
}

// pickWanderDirection picks a direction with momentum bias.
// 60% chance to keep going the same direction, otherwise pick randomly.
func (e *Engine) pickWanderDirection(enemy *Enemy, dirs []Direction) Direction {
	// Try to keep current direction (momentum) 60% of the time
	if rand.Float64() < 0.6 {
		for _, d := range dirs {
			if d == enemy.Dir {
				return d
			}
		}
	}
	// Random from available
	return dirs[rand.Intn(len(dirs))]
}

// moveEnemy applies a direction to the enemy's position.
func (e *Engine) moveEnemy(enemy *Enemy, dir Direction) {
	enemy.Pos = applyDirection(enemy.Pos, dir)
	enemy.Dir = dir
}

// applyDirection returns the new position after moving in dir from pos.
func applyDirection(pos Position, dir Direction) Position {
	switch dir {
	case DirUp:
		pos.Y--
	case DirDown:
		pos.Y++
	case DirLeft:
		pos.X--
	case DirRight:
		pos.X++
	}
	return pos
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// checkEnemyPlayerCollisions kills any alive player standing on the same tile as an alive enemy.
func (e *Engine) checkEnemyPlayerCollisions() {
	enemySet := make(map[Position]bool)
	for _, enemy := range e.State.Enemies {
		if enemy.Alive {
			enemySet[enemy.Pos] = true
		}
	}

	for _, p := range e.State.Players {
		if p.Alive && enemySet[p.Pos] {
			p.Alive = false
		}
	}
}

// damageEnemiesInFire kills any alive enemy standing on a fire tile.
func (e *Engine) damageEnemiesInFire() {
	fireSet := make(map[Position]bool, len(e.State.Fires))
	for _, f := range e.State.Fires {
		fireSet[f.Pos] = true
	}

	for _, enemy := range e.State.Enemies {
		if enemy.Alive && fireSet[enemy.Pos] {
			enemy.Alive = false
		}
	}
}

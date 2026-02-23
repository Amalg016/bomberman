package game

import (
	"testing"
)

func TestNewBoard(t *testing.T) {
	config := DefaultConfig()
	board := NewBoard(config)

	// Check dimensions
	if len(board) != config.Height {
		t.Fatalf("expected height %d, got %d", config.Height, len(board))
	}
	if len(board[0]) != config.Width {
		t.Fatalf("expected width %d, got %d", config.Width, len(board[0]))
	}

	// Check border walls
	for x := 0; x < config.Width; x++ {
		if board[0][x] != HardWall {
			t.Errorf("top border at (%d,0) should be HardWall", x)
		}
		if board[config.Height-1][x] != HardWall {
			t.Errorf("bottom border at (%d,%d) should be HardWall", x, config.Height-1)
		}
	}
	for y := 0; y < config.Height; y++ {
		if board[y][0] != HardWall {
			t.Errorf("left border at (0,%d) should be HardWall", y)
		}
		if board[y][config.Width-1] != HardWall {
			t.Errorf("right border at (%d,%d) should be HardWall", config.Width-1, y)
		}
	}

	// Check pillar pattern (even,even interior positions)
	for y := 2; y < config.Height-1; y += 2 {
		for x := 2; x < config.Width-1; x += 2 {
			if board[y][x] != HardWall {
				t.Errorf("pillar at (%d,%d) should be HardWall, got %d", x, y, board[y][x])
			}
		}
	}

	// Check spawn corners are clear
	spawns := SpawnPositions(config.Width, config.Height)
	for _, sp := range spawns {
		if board[sp.Y][sp.X] != Empty {
			t.Errorf("spawn position (%d,%d) should be Empty, got %d", sp.X, sp.Y, board[sp.Y][sp.X])
		}
	}
}

func TestMovePlayer(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0 // No soft walls for predictable testing
	engine := NewEngine(config)
	engine.AddPlayer("p1", "TestPlayer")
	engine.State.Status = StatusRunning

	p := engine.State.Players["p1"]
	startPos := p.Pos // Should be (1,1)

	if startPos.X != 1 || startPos.Y != 1 {
		t.Fatalf("expected start at (1,1), got (%d,%d)", startPos.X, startPos.Y)
	}

	// Move right to (2,1) — should succeed
	engine.movePlayer("p1", DirRight)
	if p.Pos.X != 2 || p.Pos.Y != 1 {
		t.Errorf("after move right: expected (2,1), got (%d,%d)", p.Pos.X, p.Pos.Y)
	}

	// Move down from (2,1) to (2,2) — should be BLOCKED (HardWall pillar at even,even)
	engine.movePlayer("p1", DirDown)
	if p.Pos.X != 2 || p.Pos.Y != 1 {
		t.Errorf("move down from (2,1) should be blocked by pillar at (2,2), got (%d,%d)", p.Pos.X, p.Pos.Y)
	}

	// Move right to (3,1) — should succeed
	engine.movePlayer("p1", DirRight)
	if p.Pos.X != 3 || p.Pos.Y != 1 {
		t.Errorf("after move right: expected (3,1), got (%d,%d)", p.Pos.X, p.Pos.Y)
	}

	// Move down from (3,1) to (3,2) — should succeed (odd X, even Y but X is odd)
	engine.movePlayer("p1", DirDown)
	if p.Pos.X != 3 || p.Pos.Y != 2 {
		t.Errorf("after move down: expected (3,2), got (%d,%d)", p.Pos.X, p.Pos.Y)
	}
}

func TestMovePlayerBlocked(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0
	engine := NewEngine(config)
	engine.AddPlayer("p1", "TestPlayer")
	engine.State.Status = StatusRunning

	p := engine.State.Players["p1"]
	// Player starts at (1,1)

	// Move up — should be blocked by top border wall
	engine.movePlayer("p1", DirUp)
	if p.Pos.X != 1 || p.Pos.Y != 1 {
		t.Errorf("move up from (1,1) should be blocked, got (%d,%d)", p.Pos.X, p.Pos.Y)
	}

	// Move left — should be blocked by left border wall
	engine.movePlayer("p1", DirLeft)
	if p.Pos.X != 1 || p.Pos.Y != 1 {
		t.Errorf("move left from (1,1) should be blocked, got (%d,%d)", p.Pos.X, p.Pos.Y)
	}

	// Move right to (2,1) — should succeed (odd row, even col but row is odd)
	engine.movePlayer("p1", DirRight)
	if p.Pos.X != 2 || p.Pos.Y != 1 {
		t.Errorf("move right from (1,1) should succeed, got (%d,%d)", p.Pos.X, p.Pos.Y)
	}

	// Move down from (2,1) to (2,2) — should be blocked (HardWall pillar at even,even)
	engine.movePlayer("p1", DirDown)
	if p.Pos.X != 2 || p.Pos.Y != 1 {
		t.Errorf("move down from (2,1) to (2,2) should be blocked by pillar, got (%d,%d)", p.Pos.X, p.Pos.Y)
	}
}

func TestPlaceBomb(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0
	engine := NewEngine(config)
	engine.AddPlayer("p1", "TestPlayer")
	engine.State.Status = StatusRunning

	// Place one bomb
	engine.placeBomb("p1")
	if len(engine.State.Bombs) != 1 {
		t.Fatalf("expected 1 bomb, got %d", len(engine.State.Bombs))
	}

	p := engine.State.Players["p1"]
	if p.BombsUsed != 1 {
		t.Errorf("expected BombsUsed=1, got %d", p.BombsUsed)
	}

	// Try to place another — should fail (BombMax=1)
	engine.placeBomb("p1")
	if len(engine.State.Bombs) != 1 {
		t.Errorf("should not place second bomb when at limit, got %d bombs", len(engine.State.Bombs))
	}
}

func TestExplosion(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0
	engine := NewEngine(config)
	engine.AddPlayer("p1", "TestPlayer")
	engine.State.Status = StatusRunning

	p := engine.State.Players["p1"]
	// Player starts at (1,1), bomb range is 2
	// Move player far enough away before placing bomb
	engine.movePlayer("p1", DirRight) // to (2,1)
	engine.movePlayer("p1", DirRight) // to (3,1)
	engine.movePlayer("p1", DirDown)  // to (3,2) — odd X so not a pillar
	engine.movePlayer("p1", DirDown)  // to (3,3)

	// Place bomb at safe distance from original spawn
	// Actually let's place at (1,1) by resetting player, placing, then moving
	p.Pos = Position{X: 1, Y: 1}
	engine.placeBomb("p1")
	// Move far enough away (range=2, so need X>3 or Y>3 from bomb at 1,1)
	p.Pos = Position{X: 5, Y: 5}

	// Manually trigger the bomb
	engine.State.Bombs[0].ExpiresAt = engine.State.Bombs[0].PlacedAt

	detonated := make(map[int]bool)
	detonated[0] = true
	engine.explode(engine.State.Bombs[0], detonated)

	// Should have fire tiles
	if len(engine.State.Fires) == 0 {
		t.Fatal("expected fire tiles after explosion")
	}

	// Player should still be alive (moved far away)
	if !p.Alive {
		t.Error("player should be alive after moving away from bomb")
	}
}

func TestPlayerDamage(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0
	engine := NewEngine(config)
	engine.AddPlayer("p1", "TestPlayer")
	engine.State.Status = StatusRunning

	p := engine.State.Players["p1"]
	// Player at (1,1), place bomb, DON'T move
	engine.placeBomb("p1")

	// Force detonate
	engine.State.Bombs[0].ExpiresAt = engine.State.Bombs[0].PlacedAt
	detonated := make(map[int]bool)
	detonated[0] = true
	engine.explode(engine.State.Bombs[0], detonated)

	// Player should be dead
	if p.Alive {
		t.Error("player standing on bomb should be killed by explosion")
	}
}

func TestSoftWallDestruction(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0
	engine := NewEngine(config)
	engine.AddPlayer("p1", "TestPlayer")
	engine.State.Status = StatusRunning

	// Manually place a soft wall next to player
	engine.State.Board[1][3] = SoftWall

	// Move right to (2,1) and place bomb
	engine.movePlayer("p1", DirRight) // (2,1)
	engine.placeBomb("p1")

	// Move away
	engine.movePlayer("p1", DirLeft) // (1,1)

	// Detonate
	engine.State.Bombs[0].ExpiresAt = engine.State.Bombs[0].PlacedAt
	detonated := make(map[int]bool)
	detonated[0] = true
	engine.explode(engine.State.Bombs[0], detonated)

	// Soft wall at (3,1) should be destroyed
	if engine.State.Board[1][3] != Empty {
		t.Errorf("soft wall at (3,1) should be destroyed, got %d", engine.State.Board[1][3])
	}
}

func TestAddPlayer(t *testing.T) {
	config := DefaultConfig()
	engine := NewEngine(config)

	// Add players
	if err := engine.AddPlayer("p1", "Alice"); err != nil {
		t.Fatalf("failed to add player 1: %v", err)
	}
	if err := engine.AddPlayer("p2", "Bob"); err != nil {
		t.Fatalf("failed to add player 2: %v", err)
	}

	if len(engine.State.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(engine.State.Players))
	}

	// Duplicate should fail
	if err := engine.AddPlayer("p1", "Alice2"); err == nil {
		t.Error("adding duplicate player should fail")
	}

	// Add up to max
	engine.AddPlayer("p3", "Charlie")
	engine.AddPlayer("p4", "Diana")
	if err := engine.AddPlayer("p5", "Eve"); err == nil {
		t.Error("adding player beyond max should fail")
	}
}

func TestWinCondition(t *testing.T) {
	config := DefaultConfig()
	config.SoftWallDensity = 0
	engine := NewEngine(config)
	engine.AddPlayer("p1", "Alice")
	engine.AddPlayer("p2", "Bob")
	engine.State.Status = StatusRunning

	// Kill p2
	engine.State.Players["p2"].Alive = false
	engine.checkWinCondition()

	if engine.State.Status != StatusOver {
		t.Error("game should be over when only 1 player alive")
	}
	if engine.State.Winner != "p1" {
		t.Errorf("winner should be p1, got %s", engine.State.Winner)
	}
}

# 💣 Go-Bomberman TUI

A high-performance, concurrent, grid-based multiplayer Bomberman game for the terminal, written in Go.

Players connect over a local network (Wi-Fi/Hotspot) and play in real-time via a colorful terminal UI.

## Features

- **Multiplayer** — Up to 4 players over local TCP
- **Server-Authoritative** — All game logic runs on the server to prevent cheating
- **Concurrent Bombs** — Bombs tick in the background using goroutines; chain reactions supported
- **Rich TUI** — Colored terminal interface powered by [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Cross-Platform** — Compile for Linux, macOS, and Windows

## Quick Start

### Host a Game (Server + Player)

```bash
go run ./cmd/server/ --port 9999 --name "Host"
```

The server prints your local IP addresses. Share one with your friends.

### Join a Game (Client)

```bash
go run ./cmd/client/ --addr 192.168.x.x:9999 --name "Alice"
```

### Controls

| Key | Action |
|-----|--------|
| `W` / `↑` | Move Up |
| `S` / `↓` | Move Down |
| `A` / `←` | Move Left |
| `D` / `→` | Move Right |
| `Space` | Place Bomb |
| `Enter` | Start Game (from lobby) |
| `Q` / `Esc` | Quit |

## Building

```bash
# Build both binaries
go build -o bomberman-server ./cmd/server/
go build -o bomberman-client ./cmd/client/

# Cross-compile for friends
GOOS=windows GOARCH=amd64 go build -o bomberman-client.exe ./cmd/client/
GOOS=darwin GOARCH=arm64 go build -o bomberman-client-mac ./cmd/client/
```

## Project Structure

```
go-bomberman/
├── cmd/
│   ├── server/          # Host entry point (server + embedded TUI)
│   └── client/          # Player entry point
├── internal/
│   ├── game/            # Core engine (types, board, movement, bombs)
│   ├── network/         # TCP protocol, server, client
│   └── ui/              # Bubbletea model + Lipgloss renderer
├── go.mod
└── README.md
```

## Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `9999` | TCP port to listen on |
| `--name` | `Host` | Host player name |
| `--width` | `15` | Board width (auto-corrected to odd) |
| `--height` | `13` | Board height (auto-corrected to odd) |
| `--max-players` | `4` | Maximum player count |

## How It Works

1. **Server** opens a TCP port and starts the game engine at 20 ticks/second
2. **Clients** connect, send a join message, and receive a player ID
3. Players send **actions** (move/bomb) to the server
4. Server processes actions, updates state, and **broadcasts** the full game state every tick
5. Clients render the received state using Bubbletea + Lipgloss

## License

MIT

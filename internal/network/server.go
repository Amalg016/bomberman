package network

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/amalg/go-bomberman/internal/game"
)

// Server hosts the game and manages client connections.
type Server struct {
	engine   *game.Engine
	addr     string
	listener net.Listener
	clients  map[string]*clientConn
	mu       sync.RWMutex
	done     chan struct{}
}

// clientConn represents a connected client.
type clientConn struct {
	conn     net.Conn
	playerID string
	mu       sync.Mutex
}

// NewServer creates a new game server.
func NewServer(addr string, config game.GameConfig) *Server {
	engine := game.NewEngine(config)

	s := &Server{
		engine:  engine,
		addr:    addr,
		clients: make(map[string]*clientConn),
		done:    make(chan struct{}),
	}

	// Set up the broadcast callback — receives a pre-copied state from the engine
	engine.OnTick(func(state game.GameState) {
		s.broadcastState(state)
	})

	return s
}

// Engine returns the underlying game engine.
func (s *Server) Engine() *game.Engine {
	return s.engine
}

// Start begins accepting connections and running the game loop.
func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Printf("[SERVER] Listening on %s", s.addr)

	// Print local IPs for convenience
	printLocalIPs(s.addr)

	// Start game engine in background
	go s.engine.Run()

	// Accept connections
	go s.acceptLoop()

	return nil
}

// Stop shuts down the server.
func (s *Server) Stop() {
	close(s.done)
	s.engine.Stop()
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.RLock()
	for _, c := range s.clients {
		c.conn.Close()
	}
	s.mu.RUnlock()
}

// StartGame starts the game from lobby to running.
func (s *Server) StartGame() error {
	return s.engine.StartGame()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				log.Printf("[SERVER] Accept error: %v", err)
				continue
			}
		}
		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	defer conn.Close()

	// Read join message
	env, err := Decode(conn)
	if err != nil {
		log.Printf("[SERVER] Failed to read join message: %v", err)
		return
	}

	if env.Type != MsgJoin {
		log.Printf("[SERVER] Expected join message, got %s", env.Type)
		Encode(conn, MsgError, ErrorMsg{Message: "expected join message"})
		return
	}

	var joinMsg JoinMsg
	if err := DecodePayload(env, &joinMsg); err != nil {
		log.Printf("[SERVER] Failed to decode join message: %v", err)
		return
	}

	// Generate player ID
	playerID := fmt.Sprintf("p%d", time.Now().UnixNano())

	// Add player to engine
	if err := s.engine.AddPlayer(playerID, joinMsg.Name); err != nil {
		Encode(conn, MsgError, ErrorMsg{Message: err.Error()})
		return
	}

	// Register client
	cc := &clientConn{
		conn:     conn,
		playerID: playerID,
	}
	s.mu.Lock()
	s.clients[playerID] = cc
	s.mu.Unlock()

	log.Printf("[SERVER] Player joined: %s (%s)", joinMsg.Name, playerID)

	// Send welcome message
	welcome := WelcomeMsg{
		PlayerID: playerID,
		Config:   s.engine.Config,
	}
	if err := Encode(conn, MsgWelcome, welcome); err != nil {
		log.Printf("[SERVER] Failed to send welcome: %v", err)
		s.removeClient(playerID)
		return
	}

	// Send initial state
	initialState := s.engine.GetStateCopy()
	s.sendStateTo(cc, initialState)

	// Read actions loop
	for {
		select {
		case <-s.done:
			return
		default:
		}

		env, err := Decode(conn)
		if err != nil {
			log.Printf("[SERVER] Player %s disconnected: %v", playerID, err)
			s.removeClient(playerID)
			return
		}

		switch env.Type {
		case MsgAction:
			var actionMsg ActionMsg
			if err := DecodePayload(env, &actionMsg); err != nil {
				log.Printf("[SERVER] Invalid action from %s: %v", playerID, err)
				continue
			}
			s.engine.EnqueueAction(game.Action{
				PlayerID: playerID,
				Type:     actionMsg.ActionType,
				Dir:      actionMsg.Direction,
			})
		case MsgStart:
			// Host requests game start
			if err := s.engine.StartGame(); err != nil {
				Encode(conn, MsgError, ErrorMsg{Message: err.Error()})
			}
		default:
			log.Printf("[SERVER] Unknown message type from %s: %s", playerID, env.Type)
		}
	}
}

func (s *Server) removeClient(playerID string) {
	s.mu.Lock()
	if cc, ok := s.clients[playerID]; ok {
		cc.conn.Close()
		delete(s.clients, playerID)
	}
	s.mu.Unlock()
	s.engine.RemovePlayer(playerID)
	log.Printf("[SERVER] Player removed: %s", playerID)
}

func (s *Server) broadcastState(state game.GameState) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cc := range s.clients {
		s.sendStateTo(cc, state)
	}
}

func (s *Server) sendStateTo(cc *clientConn, state game.GameState) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	msg := StateMsg{State: state}
	if err := Encode(cc.conn, MsgState, msg); err != nil {
		log.Printf("[SERVER] Failed to send state to %s: %v", cc.playerID, err)
	}
}

// printLocalIPs prints all local network interfaces for players to connect to.
func printLocalIPs(addr string) {
	_, port, _ := net.SplitHostPort(addr)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	log.Println("[SERVER] Players can connect using:")
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				log.Printf("[SERVER]   %s:%s", ipnet.IP.String(), port)
			}
		}
	}
}

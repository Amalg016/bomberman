package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// BroadcastPort is the UDP port used for room discovery.
	BroadcastPort = 9998
	// BroadcastInterval is how often hosts advertise their room.
	BroadcastInterval = 1 * time.Second
	// RoomExpiry is how long a room stays visible after its last broadcast.
	RoomExpiry = 4 * time.Second
)

// RoomInfo describes an available game room on the network.
type RoomInfo struct {
	RoomName    string `json:"room_name"`
	HostName    string `json:"host_name"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	GameAddr    string `json:"game_addr"` // TCP host:port to connect to
}

// --- Broadcaster ---

// Broadcaster periodically sends UDP broadcast packets with room info.
type Broadcaster struct {
	info RoomInfo
	done chan struct{}
	mu   sync.Mutex
}

// NewBroadcaster creates a new room broadcaster.
func NewBroadcaster(info RoomInfo) *Broadcaster {
	return &Broadcaster{
		info: info,
		done: make(chan struct{}),
	}
}

// UpdatePlayerCount updates the advertised player count.
func (b *Broadcaster) UpdatePlayerCount(count int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.info.PlayerCount = count
}

// Start begins broadcasting room info via UDP.
func (b *Broadcaster) Start() error {
	go b.broadcastLoop()
	return nil
}

// Stop stops the broadcaster.
func (b *Broadcaster) Stop() {
	select {
	case <-b.done:
	default:
		close(b.done)
	}
}

func (b *Broadcaster) broadcastLoop() {
	// Use ListenPacket (not DialUDP) so broadcast works on Linux.
	// DialUDP to 255.255.255.255 silently fails without SO_BROADCAST.
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		log.Printf("[DISCOVERY] Failed to create broadcast socket: %v", err)
		return
	}
	defer conn.Close()

	dst := &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: BroadcastPort,
	}

	ticker := time.NewTicker(BroadcastInterval)
	defer ticker.Stop()

	// Send immediately on start, then on tick
	b.sendBroadcast(conn, dst)

	for {
		select {
		case <-b.done:
			return
		case <-ticker.C:
			b.sendBroadcast(conn, dst)
		}
	}
}

func (b *Broadcaster) sendBroadcast(conn net.PacketConn, dst net.Addr) {
	b.mu.Lock()
	data, err := json.Marshal(b.info)
	b.mu.Unlock()
	if err != nil {
		return
	}

	// 1. Always send to loopback for same-machine discovery
	//    (255.255.255.255 broadcast is often dropped by Linux firewall)
	loopback := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: BroadcastPort}
	conn.WriteTo(data, loopback)

	// 2. Try global broadcast
	conn.WriteTo(data, dst)

	// 3. Also broadcast on each interface's specific broadcast address
	b.broadcastOnInterfaces(conn, data)
}

// broadcastOnInterfaces sends to each interface's broadcast address as a fallback.
func (b *Broadcaster) broadcastOnInterfaces(conn net.PacketConn, data []byte) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}

			// Calculate broadcast address: IP | ~Mask
			broadcast := make(net.IP, 4)
			ip4 := ipnet.IP.To4()
			mask := ipnet.Mask
			for i := range broadcast {
				broadcast[i] = ip4[i] | ^mask[i]
			}

			dst := &net.UDPAddr{IP: broadcast, Port: BroadcastPort}
			conn.WriteTo(data, dst)
		}
	}
}

// --- Listener ---

// discoveredRoom holds a room and when it was last seen.
type discoveredRoom struct {
	Info     RoomInfo
	LastSeen time.Time
}

// Listener listens for UDP broadcast room advertisements.
type Listener struct {
	rooms map[string]*discoveredRoom // keyed by GameAddr
	mu    sync.RWMutex
	conn  *net.UDPConn
	done  chan struct{}
}

// NewListener creates a new room listener.
func NewListener() *Listener {
	return &Listener{
		rooms: make(map[string]*discoveredRoom),
		done:  make(chan struct{}),
	}
}

// Start begins listening for room broadcasts.
func (l *Listener) Start() error {
	addr := &net.UDPAddr{
		Port: BroadcastPort,
		IP:   net.IPv4zero,
	}

	var err error
	l.conn, err = net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("listen UDP on port %d: %w (is another instance browsing?)", BroadcastPort, err)
	}

	go l.listenLoop()
	go l.cleanupLoop()

	return nil
}

// Stop stops the listener.
func (l *Listener) Stop() {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	if l.conn != nil {
		l.conn.Close()
	}
}

// Rooms returns a snapshot of currently visible rooms.
func (l *Listener) Rooms() []RoomInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	rooms := make([]RoomInfo, 0, len(l.rooms))
	for _, dr := range l.rooms {
		rooms = append(rooms, dr.Info)
	}
	return rooms
}

func (l *Listener) listenLoop() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-l.done:
			return
		default:
		}

		l.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var info RoomInfo
		if err := json.Unmarshal(buf[:n], &info); err != nil {
			continue
		}

		l.mu.Lock()
		l.rooms[info.GameAddr] = &discoveredRoom{
			Info:     info,
			LastSeen: time.Now(),
		}
		l.mu.Unlock()
	}
}

func (l *Listener) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.done:
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for addr, dr := range l.rooms {
				if now.Sub(dr.LastSeen) > RoomExpiry {
					delete(l.rooms, addr)
				}
			}
			l.mu.Unlock()
		}
	}
}

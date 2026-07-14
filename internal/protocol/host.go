package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"sync"
)

type Host struct {
	listener net.Listener
	rooms    map[string]*roomState
	mu       sync.RWMutex
}

type roomState struct {
	cfg      RoomConfig
	verifier string
	clients  map[*clientConn]struct{}
}

type clientConn struct {
	conn net.Conn
	nick string
	room string
	send chan ControlFrame
}

func StartHost(ctx context.Context, rooms []RoomConfig) (*Host, int, error) {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen: %w", err)
	}

	host := &Host{
		listener: ln,
		rooms:    make(map[string]*roomState, len(rooms)),
	}
	for _, cfg := range rooms {
		host.rooms[cfg.Name] = &roomState{
			cfg:      cfg,
			verifier: string(AuthVerifier(cfg.Name, cfg.Passcode)),
			clients:  make(map[*clientConn]struct{}),
		}
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
		host.closeAll()
	}()
	go host.acceptLoop()

	return host, ln.Addr().(*net.TCPAddr).Port, nil
}

func (h *Host) acceptLoop() {
	for {
		conn, err := h.listener.Accept()
		if err != nil {
			return
		}
		go h.handleConn(conn)
	}
}

func (h *Host) handleConn(conn net.Conn) {
	reader := bufio.NewReader(conn)

	frame, err := ReadFrame(reader)
	if err != nil || frame.Type != FrameTypeHello {
		_ = conn.Close()
		return
	}

	var hello HelloPayload
	if err := json.Unmarshal(frame.Payload, &hello); err != nil {
		_ = WriteFrame(conn, ControlFrame{Type: FrameTypeError, Room: frame.Room, Payload: []byte(`"invalid hello"`)})
		_ = conn.Close()
		return
	}

	room := h.room(frame.Room)
	if room == nil || string(hello.Verifier) != room.verifier {
		_ = WriteFrame(conn, ControlFrame{Type: FrameTypeError, Room: frame.Room, Payload: []byte(`"auth failed"`)})
		_ = conn.Close()
		return
	}

	client := &clientConn{
		conn: conn,
		nick: hello.Nickname,
		room: frame.Room,
		send: make(chan ControlFrame, 16),
	}
	h.addClient(room, client)
	defer h.removeClient(room, client)

	go writeLoop(client)
	h.broadcast(room, ControlFrame{Type: FrameTypeJoined, Room: room.cfg.Name, Sender: client.nick}, nil)

	for {
		msg, err := ReadFrame(reader)
		if err != nil {
			h.broadcast(room, ControlFrame{Type: FrameTypeLeft, Room: room.cfg.Name, Sender: client.nick}, client)
			return
		}
		if msg.Type != FrameTypeMsg || msg.Room != room.cfg.Name {
			continue
		}
		h.broadcast(room, msg, nil)
	}
}

func writeLoop(client *clientConn) {
	defer client.conn.Close()
	for frame := range client.send {
		if err := WriteFrame(client.conn, frame); err != nil {
			return
		}
	}
}

func (h *Host) room(name string) *roomState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[name]
}

func (h *Host) addClient(room *roomState, client *clientConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room.clients[client] = struct{}{}
}

func (h *Host) removeClient(room *roomState, client *clientConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := room.clients[client]; !ok {
		return
	}
	delete(room.clients, client)
	close(client.send)
}

func (h *Host) broadcast(room *roomState, frame ControlFrame, except *clientConn) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range room.clients {
		if client == except {
			continue
		}
		select {
		case client.send <- frame:
		default:
		}
	}
}

func (h *Host) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, room := range h.rooms {
		for client := range room.clients {
			_ = client.conn.Close()
			close(client.send)
			delete(room.clients, client)
		}
	}
}

func (h *Host) Rooms() []RoomConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rooms := make([]RoomConfig, 0, len(h.rooms))
	for _, room := range h.rooms {
		rooms = append(rooms, room.cfg)
	}
	slices.SortFunc(rooms, func(a, b RoomConfig) int {
		return cmpString(a.Name, b.Name)
	})
	return rooms
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

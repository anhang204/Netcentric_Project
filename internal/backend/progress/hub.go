package progress

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
)

type session struct {
	conn   net.Conn
	user   contract.Identity
	events chan contract.ProgressEvent
}

type Hub struct {
	mu       sync.Mutex
	sessions map[*session]struct{}
	closed   bool
	verify   func(string) (contract.Identity, error)
}

func New(verify func(string) (contract.Identity, error)) *Hub {
	return &Hub{sessions: make(map[*session]struct{}), verify: verify}
}

func (h *Hub) Serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		client := &session{conn: conn, events: make(chan contract.ProgressEvent, 16)}
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			conn.Close()
			return net.ErrClosed
		}
		h.sessions[client] = struct{}{}
		h.mu.Unlock()
		go h.read(client)
	}
}

func (h *Hub) read(client *session) {
	defer h.remove(client)
	client.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	frames := bufio.NewReaderSize(client.conn, 8192)
	line, err := frames.ReadSlice('\n')
	var auth contract.TCPAuth
	if err != nil || json.Unmarshal(line, &auth) != nil || auth.Type != "auth" {
		return
	}
	user, err := h.verify(auth.Token)
	if err != nil {
		client.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		json.NewEncoder(client.conn).Encode(contract.TCPAuth{Type: "auth_rejected"})
		return
	}
	client.conn.SetReadDeadline(user.ExpiresAt)
	h.mu.Lock()
	client.user = user
	h.mu.Unlock()
	go h.write(client)
	// After authentication the client only receives updates. Reading detects disconnects.
	frames.ReadByte()
}

func (h *Hub) write(client *session) {
	defer h.remove(client)
	encoder := json.NewEncoder(client.conn) // Encode appends a newline to each frame.
	client.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := encoder.Encode(contract.TCPAuth{Type: "auth_ok"}); err != nil {
		return
	}
	for event := range client.events {
		client.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := encoder.Encode(event); err != nil {
			return
		}
	}
}

func (h *Hub) Publish(origin contract.Identity, entry contract.LibraryEntry) {
	event := contract.ProgressEvent{
		Type: "progress_updated", MangaID: entry.MangaID, Chapter: entry.CurrentChapter,
		Status: entry.Status, UpdatedAt: entry.UpdatedAt, Version: entry.Version,
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for client := range h.sessions {
		if client.user.UserID != origin.UserID || client.user.SessionID == origin.SessionID || !client.user.ExpiresAt.After(now) {
			continue
		}
		select {
		case client.events <- event:
		default:
			// A stalled connection may reconnect. It must not block other sessions.
			h.removeLocked(client)
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for client := range h.sessions {
		h.removeLocked(client)
	}
}

func (h *Hub) remove(client *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(client)
}

func (h *Hub) removeLocked(client *session) {
	if _, ok := h.sessions[client]; ok {
		delete(h.sessions, client)
		close(client.events)
		client.conn.Close()
	}
}

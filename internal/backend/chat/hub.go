package chat

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gorilla/websocket"
)

type client struct {
	conn     *websocket.Conn
	username string
	out      chan contract.ChatMessage
}

type Hub struct {
	mu       sync.Mutex
	rooms    map[*client]string
	closed   bool
	upgrader websocket.Upgrader
}

func New() *Hub {
	return &Hub{rooms: make(map[*client]string), upgrader: websocket.Upgrader{HandshakeTimeout: 3 * time.Second}}
}

func (h *Hub) Handle(w http.ResponseWriter, r *http.Request, user contract.Identity) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(4096)
	conn.SetReadDeadline(user.ExpiresAt)
	c := &client{conn: conn, username: user.Username, out: make(chan contract.ChatMessage, 16)}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		conn.Close()
		return
	}
	h.rooms[c] = ""
	h.mu.Unlock()
	defer h.remove(c)
	go h.write(c)
	for {
		var msg contract.ChatMessage
		if err := conn.ReadJSON(&msg); err != nil || !user.ExpiresAt.After(time.Now()) {
			return
		}
		h.handleMessage(c, msg)
	}
}

func (h *Hub) handleMessage(c *client, msg contract.ChatMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, present := h.rooms[c]
	if !present {
		return
	}
	msg.Room = strings.TrimSpace(msg.Room)
	msg.Text = strings.TrimSpace(msg.Text)
	switch msg.Type {
	case "join":
		if msg.Room == "" || len(msg.Room) > 32 {
			h.send(c, contract.ChatMessage{Type: "error", Text: "Enter a room name of 1–32 bytes."})
			return
		}
		h.rooms[c] = msg.Room
		h.send(c, contract.ChatMessage{Type: "joined", Room: msg.Room})
	case "chat":
		if room == "" || msg.Room != room {
			h.send(c, contract.ChatMessage{Type: "error", Text: "Join that room before sending messages."})
			return
		}
		if msg.Text == "" || len(msg.Text) > 500 {
			h.send(c, contract.ChatMessage{Type: "error", Text: "Enter a message of 1–500 bytes."})
			return
		}
		msg.Sender = c.username
		for other, otherRoom := range h.rooms {
			if room == otherRoom {
				h.send(other, msg)
			}
		}
	default:
		h.send(c, contract.ChatMessage{Type: "error", Text: "Unknown chat message type."})
	}
}

// Called under the hub lock. One writer owns each WebSocket connection.
func (h *Hub) send(c *client, msg contract.ChatMessage) {
	select {
	case c.out <- msg:
	default:
		h.removeLocked(c)
	}
}

func (h *Hub) write(c *client) {
	defer h.remove(c)
	for msg := range c.out {
		c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := c.conn.WriteJSON(msg); err != nil {
			return
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for c := range h.rooms {
		h.removeLocked(c)
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(c)
}

func (h *Hub) removeLocked(c *client) {
	if _, present := h.rooms[c]; present {
		delete(h.rooms, c)
		close(c.out)
		c.conn.Close()
	}
}

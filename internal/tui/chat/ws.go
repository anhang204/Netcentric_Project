package chat

import (
	"context"
	"net/http"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gorilla/websocket"
)

func (s *Screen) run(ctx context.Context, cancel context.CancelFunc, out <-chan contract.ChatMessage) {
	defer cancel()
	defer s.queue(ctx, func() {
		s.room = ""
		s.feedback.SetText("Chat disconnected. Select Join room to try again.")
		if s.connection != nil {
			s.connection("Disconnected")
		}
	})
	header := http.Header{"Authorization": {"Bearer " + s.token}}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, Proxy: http.ProxyFromEnvironment}
	conn, response, err := dialer.DialContext(ctx, s.url, header)
	if err != nil {
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
		return
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	go writeMessages(ctx, conn, out)
	conn.SetReadLimit(4096)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		var msg contract.ChatMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Type == "joined" {
			conn.SetReadDeadline(time.Time{})
		}
		s.queue(ctx, func() { s.show(msg) })
	}
}

func writeMessages(ctx context.Context, conn *websocket.Conn, out <-chan contract.ChatMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-out:
			conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if conn.WriteJSON(msg) != nil {
				conn.Close()
				return
			}
		}
	}
}

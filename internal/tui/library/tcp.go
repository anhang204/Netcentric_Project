package library

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
)

// RunTCP runs in a background goroutine for the lifetime of the login session.
func (s *Screen) RunTCP(address string) {
	go s.reload()
	for s.ctx.Err() == nil {
		if s.connection != nil {
			s.queue(func() { s.connection("Connecting") })
		}
		err := s.receive(address)
		s.queue(func() {
			if s.connection != nil {
				s.connection("Disconnected")
			}
			if err != nil {
				s.feedback.SetText("Progress connection interrupted. Reconnecting…")
			}
		})
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Screen) receive(address string) error {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(s.ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(s.ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(contract.TCPAuth{Type: "auth", Token: s.token}); err != nil {
		return err
	}
	frames := bufio.NewReaderSize(conn, 8192)
	line, err := frames.ReadSlice('\n')
	var auth contract.TCPAuth
	if err != nil || json.Unmarshal(line, &auth) != nil || auth.Type != "auth_ok" {
		return fmt.Errorf("TCP authentication failed")
	}
	conn.SetDeadline(time.Time{})
	if s.connection != nil {
		s.queue(func() { s.connection("Connected") })
	}
	go s.reload()
	for {
		line, err := frames.ReadSlice('\n')
		if err != nil {
			return err
		}
		var event contract.ProgressEvent
		if json.Unmarshal(line, &event) != nil || event.Type != "progress_updated" {
			continue
		}
		s.queue(func() {
			entry := contract.LibraryEntry{
				MangaID: event.MangaID, CurrentChapter: event.Chapter, Status: event.Status,
				UpdatedAt: event.UpdatedAt, Version: event.Version,
			}
			if s.accept(entry) {
				s.drawList()
				s.feedback.SetText(fmt.Sprintf("Updated %s to chapter %d from another session.", entry.MangaID, entry.CurrentChapter))
			}
		})
	}
}

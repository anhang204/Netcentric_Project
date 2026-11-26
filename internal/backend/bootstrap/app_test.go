package bootstrap_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/backend/accounts"
	"github.com/anhang204/Netcentric_Project/internal/backend/bootstrap"
	"github.com/anhang204/Netcentric_Project/internal/backend/library"
	"github.com/anhang204/Netcentric_Project/internal/backend/store"
	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

type testCatalog struct{}

func (testCatalog) GetManga(_ context.Context, id string) (contract.Manga, error) {
	if id == "one-piece" {
		return contract.Manga{ID: id, Title: "One Piece", TotalChapters: 30}, nil
	}
	return contract.Manga{}, contract.ErrMangaNotFound
}

type fixture struct {
	t       *testing.T
	baseURL string
	tcpAddr string
	close   func()
}

func start(t *testing.T, path string, catalog library.Catalog) *fixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	tokens, err := accounts.NewTokens("synthetic-test-secret-with-32-bytes-only")
	if err != nil {
		t.Fatal(err)
	}
	app := bootstrap.New(db, tokens, catalog)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go app.Progress.Serve(listener)
	httpServer := httptest.NewServer(app.Router)
	closeServers := func() {
		listener.Close()
		app.Close()
		httpServer.Close()
		db.Close()
	}
	t.Cleanup(closeServers)
	return &fixture{t: t, baseURL: httpServer.URL, tcpAddr: listener.Addr().String(), close: closeServers}
}

func (f *fixture) request(method, path, token string, payload any, want int, output any) {
	f.t.Helper()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			f.t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, f.baseURL+path, body)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		data, _ := io.ReadAll(res.Body)
		f.t.Fatalf("%s %s returned %d, want %d: %s", method, path, res.StatusCode, want, data)
	}
	if output != nil {
		if err := json.NewDecoder(res.Body).Decode(output); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *fixture) register(username string) {
	f.t.Helper()
	f.request(http.MethodPost, "/api/auth/register", "", contract.Credentials{Username: username, Password: "demo-password"}, 201, nil)
}

func (f *fixture) login(username string) string {
	f.t.Helper()
	var login contract.LoginResponse
	f.request(http.MethodPost, "/api/auth/login", "", contract.Credentials{Username: username, Password: "demo-password"}, 200, &login)
	if login.Token == "" {
		f.t.Fatal("login returned an empty JWT")
	}
	return login.Token
}

func (f *fixture) add(token string) {
	f.t.Helper()
	f.request(http.MethodPost, "/api/library", token, contract.AddMangaRequest{MangaID: "one-piece"}, 201, nil)
}

func TestAccountsLibraryAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test data", "users.db")
	f := start(t, path, testCatalog{})
	f.request(http.MethodGet, "/api/health", "", nil, 200, nil)
	f.request(http.MethodPost, "/api/auth/register", "", contract.Credentials{}, 400, nil)
	f.register("alice")
	f.register("charlie")
	f.request(http.MethodPost, "/api/auth/register", "", contract.Credentials{Username: "alice", Password: "demo-password"}, 409, nil)
	f.request(http.MethodPost, "/api/auth/login", "", contract.Credentials{Username: "alice", Password: "wrong-password"}, 401, nil)
	f.request(http.MethodGet, "/api/library", "not-a-jwt", nil, 401, nil)
	f.request(http.MethodGet, "/api/library", "", nil, 401, nil)
	alice, charlie := f.login("alice"), f.login("charlie")
	f.add(alice)
	f.request(http.MethodPost, "/api/library", alice, contract.AddMangaRequest{MangaID: "one-piece"}, 409, nil)
	f.request(http.MethodPost, "/api/library", alice, contract.AddMangaRequest{MangaID: "missing"}, 404, nil)
	var isolated contract.LibraryList
	f.request(http.MethodGet, "/api/library", charlie, nil, 200, &isolated)
	if isolated.Items == nil || len(isolated.Items) != 0 {
		t.Fatalf("expected a JSON empty array, got %+v", isolated.Items)
	}
	const progressPath = "/api/library/one-piece/progress"
	f.request(http.MethodPut, progressPath, charlie, gin.H{"current_chapter": 2}, 404, nil)
	f.request(http.MethodPut, progressPath, charlie, gin.H{"user_id": 1, "current_chapter": 2}, 400, nil)
	f.request(http.MethodPut, progressPath, alice, gin.H{"status": "reading", "current_chapter": 0}, 200, nil)
	f.request(http.MethodPut, progressPath, alice, gin.H{"status": "completed", "current_chapter": 30}, 200, nil)
	for _, chapter := range []any{-1, 30.5, 31} {
		f.request(http.MethodPut, progressPath, alice, gin.H{"current_chapter": chapter}, 400, nil)
	}
	f.request(http.MethodPut, progressPath, alice, gin.H{"status": "unknown"}, 400, nil)
	f.close()

	// Simulate a backend restart while the catalog is temporarily unavailable.
	f = start(t, path, nil)
	alice = f.login("alice")
	var saved contract.LibraryList
	f.request(http.MethodGet, "/api/library", alice, nil, 200, &saved)
	if len(saved.Items) != 1 || saved.Items[0].CurrentChapter != 30 || saved.Items[0].Status != contract.Completed || saved.Items[0].Version != 3 {
		t.Fatalf("wrong persisted library: %+v", saved.Items)
	}
	f.request(http.MethodPut, progressPath, alice, gin.H{"current_chapter": 29}, 503, nil)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var hash string
	if err := db.QueryRow("SELECT password_hash FROM users WHERE username = 'alice'").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == "demo-password" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("demo-password")) != nil {
		t.Fatal("incorrect stored password hash")
	}
}

func (f *fixture) tcp(token string) (net.Conn, *bufio.Reader) {
	f.t.Helper()
	conn, err := net.DialTimeout("tcp", f.tcpAddr, 2*time.Second)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { conn.Close() })
	data, err := json.Marshal(contract.TCPAuth{Type: "auth", Token: token})
	if err != nil {
		f.t.Fatal(err)
	}
	// Deliberately write one line in two pieces.
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data[:6]); err != nil {
		f.t.Fatal(err)
	}
	if _, err := conn.Write(append(data[6:], '\n')); err != nil {
		f.t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	var ack contract.TCPAuth
	if err != nil || json.Unmarshal(line, &ack) != nil || ack.Type != "auth_ok" {
		f.t.Fatalf("missing TCP authentication acknowledgment: %s (%v)", line, err)
	}
	conn.SetDeadline(time.Time{})
	return conn, reader
}

func expectNoEvent(t *testing.T, conn net.Conn, reader *bufio.Reader) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(120 * time.Millisecond))
	line, err := reader.ReadBytes('\n')
	timeout, ok := err.(net.Error)
	if len(line) > 0 || !ok || !timeout.Timeout() {
		t.Fatalf("expected a read timeout with no event: %s (%v)", line, err)
	}
}

func TestProgressReachesOnlySiblingTCP(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), testCatalog{})
	f.register("alice")
	f.register("charlie")
	origin := f.login("alice")
	badFrame, err := net.DialTimeout("tcp", f.tcpAddr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer badFrame.Close()
	badFrame.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := badFrame.Write([]byte(`{"type":"auth","token":"` + origin + `"}`)); err != nil {
		t.Fatal(err)
	}
	if err := badFrame.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(badFrame).ReadByte(); err != io.EOF {
		t.Fatalf("incomplete TCP frame was not rejected: %v", err)
	}
	sibling, siblingRead := f.tcp(f.login("alice"))
	sender, senderRead := f.tcp(origin)
	other, otherRead := f.tcp(f.login("charlie"))
	f.add(origin)
	const path = "/api/library/one-piece/progress"
	f.request(http.MethodPut, path, origin, gin.H{"current_chapter": 7, "status": "reading"}, 200, nil)
	sibling.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := siblingRead.ReadBytes('\n')
	var event contract.ProgressEvent
	if err != nil || json.Unmarshal(line, &event) != nil || event.Type != "progress_updated" || event.MangaID != "one-piece" || event.Chapter != 7 || event.Version != 2 {
		t.Fatalf("wrong TCP event: %s (%v)", line, err)
	}
	expectNoEvent(t, sender, senderRead)
	expectNoEvent(t, other, otherRead)
	f.request(http.MethodPut, path, origin, gin.H{"current_chapter": 31}, 400, nil)
	expectNoEvent(t, sibling, siblingRead)
	sibling.Close()
	f.request(http.MethodPut, path, origin, gin.H{"current_chapter": 8}, 200, nil)
}

func (f *fixture) ws(token, room string) *websocket.Conn {
	f.t.Helper()
	url := "ws" + strings.TrimPrefix(f.baseURL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {"Bearer " + token}})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { conn.Close() })
	writeChat(f.t, conn, contract.ChatMessage{Type: "join", Room: room})
	if msg := readChat(f.t, conn); msg.Type != "joined" || msg.Room != room {
		f.t.Fatalf("wrong join acknowledgment: %+v", msg)
	}
	return conn
}

func writeChat(t *testing.T, conn *websocket.Conn, msg contract.ChatMessage) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
}

func readChat(t *testing.T, conn *websocket.Conn) contract.ChatMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg contract.ChatMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestWebSocketRoomsAndDisconnect(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), nil)
	for _, name := range []string{"alice", "bob", "charlie"} {
		f.register(name)
	}
	f.request(http.MethodGet, "/ws", "", nil, 401, nil)
	alice := f.ws(f.login("alice"), "general")
	bob := f.ws(f.login("bob"), "general")
	charlie := f.ws(f.login("charlie"), "other")
	writeChat(t, charlie, contract.ChatMessage{Type: "chat", Room: "general", Text: "wrong room"})
	if msg := readChat(t, charlie); msg.Type != "error" {
		t.Fatalf("expected room error, got %+v", msg)
	}
	writeChat(t, alice, contract.ChatMessage{Type: "chat", Room: "general", Text: "hello", Sender: "spoofed"})
	for _, conn := range []*websocket.Conn{alice, bob} {
		if msg := readChat(t, conn); msg.Type != "chat" || msg.Sender != "alice" || msg.Text != "hello" || msg.Room != "general" {
			t.Fatalf("wrong chat event: %+v", msg)
		}
	}
	writeChat(t, charlie, contract.ChatMessage{Type: "chat", Room: "other", Text: "local"})
	if msg := readChat(t, charlie); msg.Room != "other" || msg.Text != "local" {
		t.Fatalf("general content leaked into another room: %+v", msg)
	}
	bob.Close()
	writeChat(t, charlie, contract.ChatMessage{Type: "join", Room: "general"})
	if msg := readChat(t, charlie); msg.Type != "joined" {
		t.Fatalf("room did not accept a client after disconnect: %+v", msg)
	}
	writeChat(t, alice, contract.ChatMessage{Type: "chat", Room: "general", Text: "still here"})
	if msg := readChat(t, charlie); msg.Text != "still here" {
		t.Fatalf("chat stopped after Bob disconnected: %+v", msg)
	}
}

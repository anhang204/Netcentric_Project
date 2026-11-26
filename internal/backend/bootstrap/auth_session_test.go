package bootstrap_test

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

func tokenExpiringAt(t *testing.T, expires time.Time) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "1", "username": "alice", "sid": "test-session",
		"iss": "mangahub", "exp": expires.Unix(),
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestExpiredLoginCannotKeepSocketsOpen(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), nil)
	f.register("alice")
	expires := time.Now().Truncate(time.Second).Add(3 * time.Second)
	token := tokenExpiringAt(t, expires)
	f.request(http.MethodGet, "/api/library", token, nil, http.StatusOK, nil)
	tcp, reader := f.tcp(token)
	ws := f.ws(token, "general")
	readBy := expires.Add(time.Second)
	tcp.SetReadDeadline(readBy)
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		t.Errorf("expected backend to close expired TCP session, got %v", err)
	}
	ws.SetReadDeadline(readBy)
	var message any
	var closeError *websocket.CloseError
	if err := ws.ReadJSON(&message); !errors.As(err, &closeError) {
		t.Errorf("expected backend to close expired WebSocket, got %v", err)
	}
	f.request(http.MethodGet, "/api/library", token, nil, http.StatusUnauthorized, nil)
	f.request(http.MethodGet, "/ws", token, nil, http.StatusUnauthorized, nil)
	retry, err := net.DialTimeout("tcp", f.tcpAddr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer retry.Close()
	retry.SetDeadline(time.Now().Add(time.Second))
	if err := json.NewEncoder(retry).Encode(contract.TCPAuth{Type: "auth", Token: token}); err != nil {
		t.Fatal(err)
	}
	var rejection contract.TCPAuth
	if err := json.NewDecoder(retry).Decode(&rejection); err != nil || rejection.Type != "auth_rejected" {
		t.Fatalf("expected expired TCP token to be rejected: %+v (%v)", rejection, err)
	}
}

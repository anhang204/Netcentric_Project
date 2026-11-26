package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/backend/accounts"
	"github.com/anhang204/Netcentric_Project/internal/backend/bootstrap"
	"github.com/anhang204/Netcentric_Project/internal/backend/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	tokens, err := accounts.NewTokens(os.Getenv("JWT_SECRET"))
	if err != nil {
		return err
	}
	db, err := store.Open(setting("USERS_DB_PATH", "data/users.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	// Pass the catalog client's GetManga adapter here when integrating Vy's branch.
	app := bootstrap.New(db, tokens, nil)
	defer app.Close()
	httpListener, err := net.Listen("tcp", setting("API_ADDR", "127.0.0.1:8080"))
	if err != nil {
		return err
	}
	defer httpListener.Close()
	tcpListener, err := net.Listen("tcp", setting("TCP_ADDR", "127.0.0.1:9090"))
	if err != nil {
		return err
	}
	defer tcpListener.Close()

	server := &http.Server{Handler: app.Router, ReadHeaderTimeout: 5 * time.Second}
	stopped := make(chan error, 2)
	go func() { stopped <- server.Serve(httpListener) }()
	go func() { stopped <- app.Progress.Serve(tcpListener) }()
	log.Printf("HTTP and WebSocket at %s, progress TCP at %s", httpListener.Addr(), tcpListener.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	select {
	case <-ctx.Done():
	case err = <-stopped:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdown)
	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return shutdownErr
}

func setting(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

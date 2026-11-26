package bootstrap

import (
	"database/sql"
	"net/http"

	"github.com/anhang204/Netcentric_Project/internal/backend/accounts"
	"github.com/anhang204/Netcentric_Project/internal/backend/chat"
	"github.com/anhang204/Netcentric_Project/internal/backend/httpjson"
	"github.com/anhang204/Netcentric_Project/internal/backend/library"
	"github.com/anhang204/Netcentric_Project/internal/backend/progress"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router   *gin.Engine
	Progress *progress.Hub
	chat     *chat.Hub
}

func New(db *sql.DB, tokens *accounts.Tokens, catalog library.Catalog) *App {
	router := gin.New()
	router.Use(gin.Recovery())
	app := &App{Router: router, Progress: progress.New(tokens.Verify), chat: chat.New()}
	accountRoutes := accounts.Handler{DB: db, Tokens: tokens}
	libraryRoutes := library.Handler{DB: db, Catalog: catalog, Publish: app.Progress.Publish}

	router.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.POST("/api/auth/register", accountRoutes.Register)
	router.POST("/api/auth/login", accountRoutes.Login)
	router.GET("/api/library", tokens.RequireUser(), libraryRoutes.List)
	router.POST("/api/library", tokens.RequireUser(), libraryRoutes.Add)
	router.PUT("/api/library/:mangaId/progress", tokens.RequireUser(), libraryRoutes.Update)
	router.GET("/ws", tokens.RequireUser(), func(c *gin.Context) {
		app.chat.Handle(c.Writer, c.Request, accounts.User(c).Username)
	})
	router.NoRoute(func(c *gin.Context) {
		httpjson.Error(c, http.StatusNotFound, "route_not_found", "That endpoint does not exist.")
	})
	return app
}

func (a *App) Close() {
	a.Progress.Close()
	a.chat.Close()
}

package library

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/anhang204/Netcentric_Project/internal/backend/accounts"
	"github.com/anhang204/Netcentric_Project/internal/backend/httpjson"
	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gin-gonic/gin"
)

// Catalog is the small part of Vy's gRPC adapter needed by the library.
type Catalog interface {
	GetManga(ctx context.Context, id string) (contract.Manga, error)
}

type Handler struct {
	DB      *sql.DB
	Catalog Catalog
	Publish func(contract.Identity, contract.LibraryEntry)
}

func (h Handler) List(c *gin.Context) {
	entries, err := h.list(c.Request.Context(), accounts.User(c).UserID)
	if err != nil {
		libraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.LibraryList{Items: entries})
}

func (h Handler) Add(c *gin.Context) {
	var input contract.AddMangaRequest
	if err := httpjson.Read(c, &input); err != nil {
		httpjson.Error(c, http.StatusBadRequest, "invalid_input", "Enter a manga ID.")
		return
	}
	input.MangaID = strings.TrimSpace(input.MangaID)
	if _, ok := h.findManga(c, input.MangaID); !ok {
		return
	}
	entry, err := h.add(c.Request.Context(), accounts.User(c).UserID, input.MangaID)
	if errors.Is(err, errDuplicate) {
		httpjson.Error(c, http.StatusConflict, "duplicate_entry", "This manga is already in your library.")
		return
	}
	if err != nil {
		libraryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h Handler) Update(c *gin.Context) {
	id := strings.TrimSpace(c.Param("mangaId"))
	var input contract.UpdateProgressRequest
	err := httpjson.Read(c, &input)
	if err != nil || !validID(id) || (input.CurrentChapter == nil && input.Status == nil) {
		httpjson.Error(c, http.StatusBadRequest, "invalid_input", "Provide a chapter or a reading status.")
		return
	}
	if input.Status != nil {
		switch *input.Status {
		case contract.PlanToRead, contract.Reading, contract.Completed:
		default:
			httpjson.Error(c, http.StatusBadRequest, "invalid_status", "Choose plan-to-read, reading or completed.")
			return
		}
	}
	if input.CurrentChapter != nil {
		if *input.CurrentChapter < 0 {
			httpjson.Error(c, http.StatusBadRequest, "invalid_chapter", "Chapter must be zero or higher.")
			return
		}
		manga, ok := h.findManga(c, id)
		if !ok {
			return
		}
		if *input.CurrentChapter > manga.TotalChapters {
			httpjson.Error(c, http.StatusBadRequest, "invalid_chapter", "Chapter exceeds the catalog total.")
			return
		}
	}
	user := accounts.User(c)
	entry, err := h.update(c.Request.Context(), user.UserID, id, input)
	if errors.Is(err, sql.ErrNoRows) {
		httpjson.Error(c, http.StatusNotFound, "entry_not_found", "This manga is missing from your library.")
		return
	}
	if err != nil {
		libraryError(c, err)
		return
	}
	if h.Publish != nil {
		h.Publish(user, entry)
	}
	c.JSON(http.StatusOK, entry)
}

func validID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.Contains(id, "/")
}

func (h Handler) findManga(c *gin.Context, id string) (contract.Manga, bool) {
	if !validID(id) {
		httpjson.Error(c, http.StatusBadRequest, "invalid_id", "Enter a valid manga ID.")
		return contract.Manga{}, false
	}
	if h.Catalog == nil {
		httpjson.Error(c, http.StatusServiceUnavailable, "catalog_unavailable", "The manga catalog is unavailable.")
		return contract.Manga{}, false
	}
	manga, err := h.Catalog.GetManga(c.Request.Context(), id)
	switch {
	case err == nil:
		return manga, true
	case errors.Is(err, contract.ErrInvalidCatalogInput):
		httpjson.Error(c, http.StatusBadRequest, "invalid_id", "Enter a valid manga ID.")
	case errors.Is(err, contract.ErrMangaNotFound):
		httpjson.Error(c, http.StatusNotFound, "manga_not_found", "Manga was not found.")
	case errors.Is(err, contract.ErrCatalogUnavailable), errors.Is(err, context.DeadlineExceeded):
		httpjson.Error(c, http.StatusServiceUnavailable, "catalog_unavailable", "The manga catalog is unavailable.")
	default:
		libraryError(c, err)
	}
	return contract.Manga{}, false
}

func libraryError(c *gin.Context, err error) {
	log.Printf("library: %v", err)
	httpjson.Error(c, http.StatusInternalServerError, "library_error", "The library request failed.")
}

package library

import (
	"context"
	"errors"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
)

var errDuplicate = errors.New("manga is already in the library")

const entryColumns = "manga_id, status, current_chapter, updated_at, version"

func (h Handler) list(ctx context.Context, userID int64) ([]contract.LibraryEntry, error) {
	rows, err := h.DB.QueryContext(ctx, "SELECT "+entryColumns+" FROM user_library WHERE user_id = ? ORDER BY manga_id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]contract.LibraryEntry, 0)
	for rows.Next() {
		var entry contract.LibraryEntry
		if err := rows.Scan(&entry.MangaID, &entry.Status, &entry.CurrentChapter, &entry.UpdatedAt, &entry.Version); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (h Handler) add(ctx context.Context, userID int64, mangaID string) (contract.LibraryEntry, error) {
	entry := contract.LibraryEntry{
		MangaID: mangaID, Status: contract.PlanToRead, Version: 1, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	result, err := h.DB.ExecContext(ctx, `
		INSERT INTO user_library (user_id, manga_id, status, current_chapter, updated_at, version)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(user_id, manga_id) DO NOTHING`,
		userID, entry.MangaID, entry.Status, entry.CurrentChapter, entry.UpdatedAt, entry.Version)
	if err != nil {
		return entry, err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return entry, errDuplicate
	}
	return entry, err
}

func (h Handler) update(ctx context.Context, userID int64, mangaID string, input contract.UpdateProgressRequest) (contract.LibraryEntry, error) {
	var entry contract.LibraryEntry
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return entry, err
	}
	defer tx.Rollback()
	// The UPDATE and its new version are committed before a TCP event can be sent.
	row := tx.QueryRowContext(ctx, `
		UPDATE user_library
		SET current_chapter = COALESCE(?, current_chapter), status = COALESCE(?, status),
		    updated_at = ?, version = version + 1
		WHERE user_id = ? AND manga_id = ?
		RETURNING `+entryColumns,
		input.CurrentChapter, input.Status, time.Now().UTC().Format(time.RFC3339Nano), userID, mangaID)
	if err := row.Scan(&entry.MangaID, &entry.Status, &entry.CurrentChapter, &entry.UpdatedAt, &entry.Version); err != nil {
		return entry, err
	}
	return entry, tx.Commit()
}

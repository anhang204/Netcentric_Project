package contract

import "errors"

// Identity comes from a verified login token, never from a request body.
type Identity struct {
	UserID    int64
	Username  string
	SessionID string
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Manga struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	TotalChapters int      `json:"total_chapters"`
	Description   string   `json:"description"`
}

// The catalog client maps RPC failures to these errors for the HTTP layer.
var (
	ErrInvalidCatalogInput = errors.New("invalid catalog input")
	ErrMangaNotFound       = errors.New("manga not found")
	ErrCatalogUnavailable  = errors.New("catalog unavailable")
)

type LibraryEntry struct {
	MangaID        string `json:"manga_id"`
	Status         string `json:"status"`
	CurrentChapter int    `json:"current_chapter"`
	UpdatedAt      string `json:"updated_at"`
	Version        int64  `json:"version"`
}

type LibraryList struct {
	Items []LibraryEntry `json:"items"`
}

type AddMangaRequest struct {
	MangaID string `json:"manga_id"`
}

type UpdateProgressRequest struct {
	CurrentChapter *int    `json:"current_chapter,omitempty"`
	Status         *string `json:"status,omitempty"`
}

const (
	PlanToRead = "plan-to-read"
	Reading    = "reading"
	Completed  = "completed"
)

type TCPAuth struct {
	Type  string `json:"type"`
	Token string `json:"token,omitempty"`
}

type ProgressEvent struct {
	Type      string `json:"type"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	Version   int64  `json:"version"`
}

type ChatMessage struct {
	Type   string `json:"type"`
	Room   string `json:"room,omitempty"`
	Text   string `json:"text,omitempty"`
	Sender string `json:"sender,omitempty"`
}

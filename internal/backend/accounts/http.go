package accounts

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/backend/httpjson"
	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const identityKey = "mangahub.identity"

type Handler struct {
	DB     *sql.DB
	Tokens *Tokens
}

func credentials(c *gin.Context) (contract.Credentials, bool) {
	var input contract.Credentials
	if err := httpjson.Read(c, &input); err != nil {
		httpjson.Error(c, http.StatusBadRequest, "invalid_input", "Enter a username and password.")
		return input, false
	}
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" || len(input.Username) > 40 || len(input.Password) < 6 || len(input.Password) > 72 {
		httpjson.Error(c, http.StatusBadRequest, "invalid_input", "Use a username of 1–40 bytes and a password of 6–72 bytes.")
		return input, false
	}
	return input, true
}

func (h Handler) Register(c *gin.Context) {
	input, ok := credentials(c)
	if !ok {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		accountError(c, err)
		return
	}
	result, err := h.DB.ExecContext(c.Request.Context(), `
		INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)
		ON CONFLICT(username) DO NOTHING`, input.Username, string(hash), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		accountError(c, err)
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		accountError(c, err)
		return
	}
	if count == 0 {
		httpjson.Error(c, http.StatusConflict, "username_taken", "That username is already registered.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"username": input.Username})
}

func (h Handler) Login(c *gin.Context) {
	input, ok := credentials(c)
	if !ok {
		return
	}
	var userID int64
	var hash string
	err := h.DB.QueryRowContext(c.Request.Context(), "SELECT id, password_hash FROM users WHERE username = ?", input.Username).Scan(&userID, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		accountError(c, err)
		return
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		httpjson.Error(c, http.StatusUnauthorized, "bad_credentials", "Incorrect username or password.")
		return
	}
	encoded, err := h.Tokens.Issue(userID, input.Username)
	if err != nil {
		accountError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.LoginResponse{Token: encoded})
}

func (t *Tokens) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			identity, err := t.Verify(parts[1])
			if err == nil {
				c.Set(identityKey, identity)
				c.Next()
				return
			}
		}
		httpjson.Error(c, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
	}
}

// User is used by routes protected with RequireUser.
func User(c *gin.Context) contract.Identity {
	return c.MustGet(identityKey).(contract.Identity)
}

func accountError(c *gin.Context, err error) {
	log.Printf("accounts: %v", err)
	httpjson.Error(c, http.StatusInternalServerError, "account_error", "The account request failed.")
}

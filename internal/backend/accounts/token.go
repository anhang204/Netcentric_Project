package accounts

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/golang-jwt/jwt/v5"
)

type Tokens struct {
	secret []byte
}

type claims struct {
	Username  string `json:"username"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func NewTokens(secret string) (*Tokens, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must have at least 32 bytes")
	}
	return &Tokens{secret: []byte(secret)}, nil
}

func (t *Tokens) Issue(userID int64, username string) (string, error) {
	var sessionID [16]byte
	if _, err := rand.Read(sessionID[:]); err != nil {
		return "", err
	}
	now := time.Now()
	data := claims{
		Username:  username,
		SessionID: hex.EncodeToString(sessionID[:]),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mangahub",
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(8 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, data).SignedString(t.secret)
}

func (t *Tokens) Verify(encoded string) (contract.Identity, error) {
	var data claims
	token, err := jwt.ParseWithClaims(encoded, &data, func(_ *jwt.Token) (any, error) {
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("mangahub"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return contract.Identity{}, errors.New("invalid login token")
	}
	userID, err := strconv.ParseInt(data.Subject, 10, 64)
	if err != nil || userID < 1 || data.Username == "" || data.SessionID == "" {
		return contract.Identity{}, errors.New("invalid login identity")
	}
	return contract.Identity{
		UserID: userID, Username: data.Username, SessionID: data.SessionID, ExpiresAt: data.ExpiresAt.Time,
	}, nil
}

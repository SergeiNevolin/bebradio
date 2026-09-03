// Package auth issues and validates the two kinds of bearer token the service
// uses, and hashes passwords.
//
// A *user token* identifies an account and is what the browser stores after
// login. A *room-access token* proves only that its holder supplied the correct
// password for one specific room; it names no user and grants nothing anywhere
// else. Keeping them apart means an anonymous guest can be let into a
// password-protected room without ever being handed account credentials.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const roomAccessScope = "room_access"

// ErrInvalidToken is returned for a token that is malformed, expired, or signed
// with the wrong key. Callers must not distinguish between those cases when
// reporting to a client.
var ErrInvalidToken = errors.New("auth: invalid or expired token")

// Tokens issues and verifies the service's bearer tokens.
type Tokens struct {
	secret []byte
	ttl    time.Duration
}

// NewTokens returns a token issuer signing with secret and expiring tokens
// after ttl.
func NewTokens(secret string, ttl time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl}
}

// CreateUser issues a token identifying userID.
func (t *Tokens) CreateUser(userID string) (string, error) {
	return t.sign(jwt.MapClaims{
		"sub": userID,
		"exp": float64(time.Now().Add(t.ttl).UnixNano()) / float64(time.Second),
	})
}

// ParseUser returns the user id carried by a user token.
func (t *Tokens) ParseUser(token string) (string, error) {
	claims, err := t.parse(token)
	if err != nil {
		return "", err
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", ErrInvalidToken
	}
	// A room-access token must never stand in for a user token.
	if scope, _ := claims["scope"].(string); scope == roomAccessScope {
		return "", ErrInvalidToken
	}
	return sub, nil
}

// CreateRoomAccess issues a token granting entry to one password-protected
// room. It is handed out only once the password has been verified.
func (t *Tokens) CreateRoomAccess(roomID string) (string, error) {
	return t.sign(jwt.MapClaims{
		"room":  roomID,
		"scope": roomAccessScope,
		"exp":   float64(time.Now().Add(t.ttl).UnixNano()) / float64(time.Second),
	})
}

// VerifyRoomAccess reports whether token grants entry to roomID.
func (t *Tokens) VerifyRoomAccess(token, roomID string) bool {
	if token == "" {
		return false
	}
	claims, err := t.parse(token)
	if err != nil {
		return false
	}
	scope, _ := claims["scope"].(string)
	room, _ := claims["room"].(string)
	return scope == roomAccessScope && room == roomID
}

func (t *Tokens) sign(claims jwt.MapClaims) (string, error) {
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("auth: signing token: %w", err)
	}
	return signed, nil
}

func (t *Tokens) parse(token string) (jwt.MapClaims, error) {
	parsed, err := jwt.Parse(token, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", tok.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// Passwords hashes and checks passwords with bcrypt.
type Passwords struct {
	cost int
}

// NewPasswords returns a hasher at the given bcrypt cost.
func NewPasswords(cost int) *Passwords {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &Passwords{cost: cost}
}

// Hash returns the bcrypt hash of password.
func (p *Passwords) Hash(password string) (string, error) {
	// bcrypt silently truncates beyond 72 bytes and, since Go 1.20's x/crypto,
	// rejects the input outright. Reporting it is better than storing a hash
	// that ignores most of what the user typed.
	if len(password) > 72 {
		return "", errors.New("auth: password must be at most 72 bytes")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), p.cost)
	if err != nil {
		return "", fmt.Errorf("auth: hashing password: %w", err)
	}
	return string(hashed), nil
}

// Verify reports whether password matches hash. It is safe to call with an
// empty hash (an account or room with no password), and always returns false.
func (p *Passwords) Verify(password, hash string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

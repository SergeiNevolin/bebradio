// Package users handles registration, sign-in and profiles.
package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/leenzstra/bebradio/backend/internal/auth"
	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/ids"
	"github.com/leenzstra/bebradio/backend/internal/store"
)

// Errors returned by the service.
var (
	// ErrEmailTaken means an account already exists for that address.
	ErrEmailTaken = errors.New("users: email already registered")
	// ErrUsernameTaken means the display name is in use.
	ErrUsernameTaken = errors.New("users: username already taken")
	// ErrInvalidCredentials covers both an unknown address and a wrong
	// password. The two are deliberately indistinguishable, so the endpoint
	// cannot be used to discover which addresses have accounts.
	ErrInvalidCredentials = errors.New("users: invalid email or password")
	// ErrNotFound means no account has that identifier.
	ErrNotFound = errors.New("users: not found")
	// ErrInvalidInput means the submitted details are not usable.
	ErrInvalidInput = errors.New("users: invalid input")
)

// Service manages accounts.
type Service struct {
	store     store.Store
	tokens    *auth.Tokens
	passwords *auth.Passwords
}

// New returns a user service.
func New(st store.Store, tokens *auth.Tokens, passwords *auth.Passwords) *Service {
	return &Service{store: st, tokens: tokens, passwords: passwords}
}

// Register creates an account and returns it with a signed session token.
func (s *Service) Register(ctx context.Context, email, username, password string) (string, domain.User, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return "", domain.User{}, err
	}
	username, err = NormalizeUsername(username)
	if err != nil {
		return "", domain.User{}, err
	}
	if password == "" {
		return "", domain.User{}, fmt.Errorf("%w: password is required", ErrInvalidInput)
	}

	// Checking first gives a precise message; the unique constraint below is
	// what actually guarantees it, since two registrations can race here.
	if _, err := s.store.UserByEmail(ctx, email); err == nil {
		return "", domain.User{}, ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", domain.User{}, fmt.Errorf("users: checking email: %w", err)
	}
	if _, err := s.store.UserByUsername(ctx, username); err == nil {
		return "", domain.User{}, ErrUsernameTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", domain.User{}, fmt.Errorf("users: checking username: %w", err)
	}

	hash, err := s.passwords.Hash(password)
	if err != nil {
		return "", domain.User{}, fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}

	user := domain.User{
		ID:           ids.Short(),
		Email:        email,
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    domain.Now(),
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			// Lost the race against a concurrent registration. Report the more
			// likely of the two, rather than guessing with another round-trip.
			return "", domain.User{}, ErrEmailTaken
		}
		return "", domain.User{}, fmt.Errorf("users: creating account: %w", err)
	}

	token, err := s.tokens.CreateUser(user.ID)
	if err != nil {
		return "", domain.User{}, err
	}
	return token, user, nil
}

// Login verifies credentials and returns a signed session token.
func (s *Service) Login(ctx context.Context, email, password string) (string, domain.User, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))

	user, err := s.store.UserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", domain.User{}, ErrInvalidCredentials
		}
		return "", domain.User{}, fmt.Errorf("users: loading account: %w", err)
	}
	if !s.passwords.Verify(password, user.PasswordHash) {
		return "", domain.User{}, ErrInvalidCredentials
	}

	token, err := s.tokens.CreateUser(user.ID)
	if err != nil {
		return "", domain.User{}, err
	}
	return token, user, nil
}

// FromToken resolves a bearer token to the account it identifies.
func (s *Service) FromToken(ctx context.Context, token string) (domain.User, error) {
	userID, err := s.tokens.ParseUser(token)
	if err != nil {
		return domain.User{}, ErrNotFound
	}
	return s.ByID(ctx, userID)
}

// ByID returns one account.
func (s *Service) ByID(ctx context.Context, id string) (domain.User, error) {
	user, err := s.store.UserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("users: loading account: %w", err)
	}
	return user, nil
}

// UpdateProfile writes the editable parts of a profile. A nil field is left
// unchanged.
func (s *Service) UpdateProfile(ctx context.Context, id string, bio, avatarURL *string) (domain.User, error) {
	user, err := s.store.UpdateProfile(ctx, id, bio, avatarURL)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("users: updating profile: %w", err)
	}
	return user, nil
}

// NormalizeEmail lower-cases and trims an address, rejecting one that could not
// be an address at all.
//
// The check is deliberately shallow. Address syntax is far more permissive than
// most validators assume, and the only proof an address works is mail arriving
// at it; a stricter rule here would reject real addresses without making
// anything safer.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.IndexByte(email, '@')
	if at <= 0 || !strings.Contains(email[at+1:], ".") {
		return "", fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(email) > 255 {
		return "", fmt.Errorf("%w: email is too long", ErrInvalidInput)
	}
	return email, nil
}

// NormalizeUsername trims a display name and checks its length.
func NormalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if n := utf8.RuneCountInString(username); n < 2 || n > 30 {
		return "", fmt.Errorf("%w: username must be 2-30 characters", ErrInvalidInput)
	}
	return username, nil
}

package usecase

import (
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

type AuthUsecase struct {
	userRepo repository.UserRepository
	auth     AuthBridge
	log      *slog.Logger
}

type AuthBridge interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) bool
	CreateToken(userID string) (string, error)
	DecodeToken(token string) (string, error)
	CreateRoomToken(roomID string) (string, error)
	VerifyRoomToken(token string, roomID string) bool
}

func NewAuthUsecase(userRepo repository.UserRepository, auth AuthBridge, log *slog.Logger) *AuthUsecase {
	return &AuthUsecase{userRepo: userRepo, auth: auth, log: log}
}

func (uc *AuthUsecase) Register(email, username, password string) (*entity.User, string, error) {
	existing, err := uc.userRepo.FindByEmail(email)
	if err == nil && existing != nil {
		return nil, "", ErrEmailTaken
	}
	existing, err = uc.userRepo.FindByUsername(username)
	if err == nil && existing != nil {
		return nil, "", ErrUsernameTaken
	}

	hash, err := uc.auth.HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	user := &entity.User{
		ID:           generateID(8),
		Email:        email,
		Username:     username,
		PasswordHash: hash,
		Bio:          "",
		AvatarURL:    "",
		CreatedAt:    time.Now(),
	}

	if err := uc.userRepo.Create(user); err != nil {
		return nil, "", err
	}

	token, err := uc.auth.CreateToken(user.ID)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (uc *AuthUsecase) Login(email, password string) (*entity.User, string, error) {
	user, err := uc.userRepo.FindByEmail(email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if !uc.auth.VerifyPassword(password, user.PasswordHash) {
		return nil, "", ErrInvalidCredentials
	}
	token, err := uc.auth.CreateToken(user.ID)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (uc *AuthUsecase) GetUserByID(id string) (*entity.User, error) {
	return uc.userRepo.FindByID(id)
}

func (uc *AuthUsecase) DecodeToken(token string) (string, error) {
	return uc.auth.DecodeToken(token)
}

var (
	ErrEmailTaken         = &BusinessError{Code: 409, Message: "Email already registered"}
	ErrUsernameTaken      = &BusinessError{Code: 409, Message: "Username already taken"}
	ErrInvalidCredentials = &BusinessError{Code: 401, Message: "Invalid email or password"}
)

type BusinessError struct {
	Code    int
	Message string
}

func (e *BusinessError) Error() string { return e.Message }

func generateID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

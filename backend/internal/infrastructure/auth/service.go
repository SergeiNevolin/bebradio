package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrUserNotFound = errors.New("user not found")
)

// RefreshScope — клейм, отделяющий refresh-токен от access и room-токенов.
const RefreshScope = "refresh"

type Service struct {
	secretKey      []byte
	roomTokenHours int
	accessMinutes  int
	refreshDays    int
}

func New(secretKey string, roomTokenHours, accessMinutes, refreshDays int) *Service {
	return &Service{
		secretKey:      []byte(secretKey),
		roomTokenHours: roomTokenHours,
		accessMinutes:  accessMinutes,
		refreshDays:    refreshDays,
	}
}

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *Service) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CreateToken — короткоживущий access: валиден accessMinutes.
func (s *Service) CreateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Duration(s.accessMinutes) * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *Service) DecodeToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secretKey, nil
	})
	if err != nil {
		return "", ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", ErrInvalidToken
	}
	sub, _ := claims.GetSubject()
	if sub == "" {
		return "", ErrInvalidToken
	}
	return sub, nil
}

// CreateRefreshToken — долгоживущий токен для httpOnly-куки: scope=refresh + jti
// (jti хранится в Redis — отзыв/ротация, см. RefreshStore).
func (s *Service) CreateRefreshToken(userID string) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"scope": RefreshScope,
		"jti":   jti,
		"exp":   time.Now().Add(time.Duration(s.refreshDays) * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// DecodeRefreshToken → (userID, jti); принимает только scope=refresh.
func (s *Service) DecodeRefreshToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secretKey, nil
	})
	if err != nil || !token.Valid {
		return "", "", ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", ErrInvalidToken
	}
	if scope, _ := claims["scope"].(string); scope != RefreshScope {
		return "", "", ErrInvalidToken // access/room-токены в refresh не принимаем
	}
	sub, _ := claims.GetSubject()
	jti, _ := claims["jti"].(string)
	if sub == "" || jti == "" {
		return "", "", ErrInvalidToken
	}
	return sub, jti, nil
}

func newJTI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *Service) CreateRoomToken(roomID string) (string, error) {
	claims := jwt.MapClaims{
		"room":  roomID,
		"scope": "room_access",
		"exp":   time.Now().Add(time.Duration(s.roomTokenHours) * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *Service) VerifyRoomToken(tokenStr string, roomID string) bool {
	if tokenStr == "" {
		return false
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secretKey, nil
	})
	if err != nil {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return false
	}
	scope, _ := claims["scope"].(string)
	room, _ := claims["room"].(string)
	return scope == "room_access" && room == roomID
}

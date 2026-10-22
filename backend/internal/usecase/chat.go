package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/pkg/id"
	"github.com/redis/go-redis/v9"
)

type ChatUsecase struct {
	sessionRepo repository.SessionRepository
	log         *slog.Logger
	rdb         *redis.Client
}

func NewChatUsecase(roomRepo repository.RoomRepository, log *slog.Logger, rdb *redis.Client, sessionRepos ...repository.SessionRepository) *ChatUsecase {
	var sessionRepo repository.SessionRepository
	if len(sessionRepos) > 0 {
		sessionRepo = sessionRepos[0]
	} else if sr, ok := roomRepo.(repository.SessionRepository); ok {
		sessionRepo = sr
	}
	return &ChatUsecase{sessionRepo: sessionRepo, log: log, rdb: rdb}
}

func (uc *ChatUsecase) SendMessage(ctx context.Context, session any, userID, username, text string) *entity.ChatMessage {
	msg := &entity.ChatMessage{
		ID:        id.NewHex(8),
		UserID:    userID,
		Username:  username,
		Text:      text,
		CreatedAt: time.Now(),
	}
	ref := redisc.AsRef(session)

	redisc.AppendMessage(ctx, uc.rdb, ref, msg)
	if uc.sessionRepo != nil {
		if err := uc.sessionRepo.SaveMessage(ref, msg); err != nil {
			uc.log.Error("failed to save chat message", "session", ref.String(), "error", err)
		}
	}
	return msg
}

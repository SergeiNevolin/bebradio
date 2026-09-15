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
	roomRepo repository.RoomRepository
	log      *slog.Logger
	rdb      *redis.Client
}

func NewChatUsecase(roomRepo repository.RoomRepository, log *slog.Logger, rdb *redis.Client) *ChatUsecase {
	return &ChatUsecase{roomRepo: roomRepo, log: log, rdb: rdb}
}

func (uc *ChatUsecase) SendMessage(ctx context.Context, roomID, userID, username, text string) *entity.ChatMessage {
	msg := &entity.ChatMessage{
		ID:        id.NewHex(8),
		UserID:    userID,
		Username:  username,
		Text:      text,
		CreatedAt: time.Now(),
	}

	redisc.AppendMessage(ctx, uc.rdb, roomID, msg)

	if err := uc.roomRepo.SaveMessage(roomID, msg); err != nil {
		uc.log.Error("failed to save chat message", "room_id", roomID, "error", err)
	}
	return msg
}

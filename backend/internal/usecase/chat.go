package usecase

import (
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/pkg/id"
)

type ChatUsecase struct {
	roomRepo repository.RoomRepository
	log      *slog.Logger
}

func NewChatUsecase(roomRepo repository.RoomRepository, log *slog.Logger) *ChatUsecase {
	return &ChatUsecase{roomRepo: roomRepo, log: log}
}

func (uc *ChatUsecase) SendMessage(rm *entity.Room, userID, username, text string) *entity.ChatMessage {
	msg := &entity.ChatMessage{
		ID:        id.NewHex(8),
		UserID:    userID,
		Username:  username,
		Text:      text,
		CreatedAt: time.Now(),
	}

	rm.Mu.Lock()
	rm.Messages = append(rm.Messages, msg)
	if len(rm.Messages) > entity.MaxChatMessages {
		rm.Messages = rm.Messages[len(rm.Messages)-entity.MaxChatMessages:]
	}
	rm.Mu.Unlock()

	if err := uc.roomRepo.SaveMessage(rm.ID, msg); err != nil {
		uc.log.Error("failed to save chat message", "room_id", rm.ID, "error", err)
	}
	return msg
}

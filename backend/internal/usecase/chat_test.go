package usecase

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

var chatLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestSendMessage(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog, rdb)

	roomID := "X"
	rm := entity.NewRoom(roomID, "R", "O")
	roomRepo.Save(rm)

	msg := uc.SendMessage(ctx, roomID, "u1", "Alice", "Hello!")

	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if msg.UserID != "u1" {
		t.Errorf("expected user_id 'u1', got '%s'", msg.UserID)
	}
	if msg.Username != "Alice" {
		t.Errorf("expected username 'Alice', got '%s'", msg.Username)
	}
	if msg.Text != "Hello!" {
		t.Errorf("expected text 'Hello!', got '%s'", msg.Text)
	}
	if msg.ID == "" {
		t.Error("expected non-empty message ID")
	}
}

func TestSendMessageAppendsToRoom(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog, rdb)

	roomID := "X"
	rm := entity.NewRoom(roomID, "R", "O")
	roomRepo.Save(rm)

	uc.SendMessage(ctx, roomID, "u1", "Alice", "First")
	uc.SendMessage(ctx, roomID, "u2", "Bob", "Second")

	msgs, _ := redisc.GetMessages(ctx, rdb, roomID)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "First" {
		t.Errorf("expected first message 'First', got '%s'", msgs[0].Text)
	}
	if msgs[1].Text != "Second" {
		t.Errorf("expected second message 'Second', got '%s'", msgs[1].Text)
	}
}

func TestSendMessageTrimsToMax(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog, rdb)

	roomID := "X"
	rm := entity.NewRoom(roomID, "R", "O")
	roomRepo.Save(rm)

	for i := 0; i < entity.MaxChatMessages+10; i++ {
		uc.SendMessage(ctx, roomID, "u1", "Alice", "msg")
	}

	msgs, _ := redisc.GetMessages(ctx, rdb, roomID)
	if len(msgs) != entity.MaxChatMessages {
		t.Errorf("expected %d messages (max), got %d", entity.MaxChatMessages, len(msgs))
	}
}

func TestSendMessageSavesToRepo(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog, rdb)

	roomID := "X"
	rm := entity.NewRoom(roomID, "R", "O")
	roomRepo.Save(rm)

	uc.SendMessage(ctx, roomID, "u1", "Alice", "Persist me")

	if len(roomRepo.Messages[roomID]) != 1 {
		t.Errorf("expected 1 message saved to repo, got %d", len(roomRepo.Messages[roomID]))
	}
}

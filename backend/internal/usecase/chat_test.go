package usecase

import (
	"log/slog"
	"os"
	"testing"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

var chatLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestSendMessage(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog)

	rm := entity.NewRoom("X", "R", "O")
	msg := uc.SendMessage(rm, "u1", "Alice", "Hello!")

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
	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog)

	rm := entity.NewRoom("X", "R", "O")
	uc.SendMessage(rm, "u1", "Alice", "First")
	uc.SendMessage(rm, "u2", "Bob", "Second")

	if len(rm.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(rm.Messages))
	}
	if rm.Messages[0].Text != "First" {
		t.Errorf("expected first message 'First', got '%s'", rm.Messages[0].Text)
	}
	if rm.Messages[1].Text != "Second" {
		t.Errorf("expected second message 'Second', got '%s'", rm.Messages[1].Text)
	}
}

func TestSendMessageTrimsToMax(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog)

	rm := entity.NewRoom("X", "R", "O")
	// Add more than MaxChatMessages
	for i := 0; i < entity.MaxChatMessages+10; i++ {
		uc.SendMessage(rm, "u1", "Alice", "msg")
	}

	if len(rm.Messages) != entity.MaxChatMessages {
		t.Errorf("expected %d messages (max), got %d", entity.MaxChatMessages, len(rm.Messages))
	}
}

func TestSendMessageSavesToRepo(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	uc := NewChatUsecase(roomRepo, chatLog)

	rm := entity.NewRoom("X", "R", "O")
	uc.SendMessage(rm, "u1", "Alice", "Persist me")

	if len(roomRepo.Messages["X"]) != 1 {
		t.Errorf("expected 1 message saved to repo, got %d", len(roomRepo.Messages["X"]))
	}
}

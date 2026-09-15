package entity

import (
	"testing"
)

func TestChatMessageToDict(t *testing.T) {
	msg := &ChatMessage{
		ID:       "msg1",
		UserID:   "u1",
		Username: "Alice",
		Text:     "Hello, world!",
	}

	dict := msg.ToDict()
	if dict["id"] != "msg1" {
		t.Errorf("expected id 'msg1', got '%v'", dict["id"])
	}
	if dict["user_id"] != "u1" {
		t.Errorf("expected user_id 'u1', got '%v'", dict["user_id"])
	}
	if dict["username"] != "Alice" {
		t.Errorf("expected username 'Alice', got '%v'", dict["username"])
	}
	if dict["text"] != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got '%v'", dict["text"])
	}
	if dict["created_at"] == nil {
		t.Error("expected created_at to be set")
	}
}

func TestChatMessageToDictEmpty(t *testing.T) {
	msg := &ChatMessage{}
	dict := msg.ToDict()

	if dict["id"] != "" {
		t.Errorf("expected empty id, got '%v'", dict["id"])
	}
	if dict["text"] != "" {
		t.Errorf("expected empty text, got '%v'", dict["text"])
	}
}

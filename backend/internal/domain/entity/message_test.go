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

func TestTrackVoteValues(t *testing.T) {
	v1 := &TrackVote{UserID: "u1", TrackID: "t1", Vote: 1}
	v2 := &TrackVote{UserID: "u2", TrackID: "t1", Vote: -1}

	if v1.Vote != 1 {
		t.Errorf("expected vote 1, got %d", v1.Vote)
	}
	if v2.Vote != -1 {
		t.Errorf("expected vote -1, got %d", v2.Vote)
	}
}

package entity

import (
	"testing"
)

func TestNewRoom(t *testing.T) {
	rm := NewRoom("ABC123", "Test Room", "owner1")

	if rm.ID != "ABC123" {
		t.Errorf("expected ID 'ABC123', got '%s'", rm.ID)
	}
	if rm.Name != "Test Room" {
		t.Errorf("expected Name 'Test Room', got '%s'", rm.Name)
	}
	if rm.OwnerID != "owner1" {
		t.Errorf("expected OwnerID 'owner1', got '%s'", rm.OwnerID)
	}
	if !rm.AllowAnonymousAdd {
		t.Error("expected AllowAnonymousAdd true")
	}
	if rm.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

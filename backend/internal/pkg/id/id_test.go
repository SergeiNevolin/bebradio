package id

import (
	"regexp"
	"testing"
)

func TestNew(t *testing.T) {
	result := New(8)
	if len(result) != 8 {
		t.Errorf("expected length 8, got %d", len(result))
	}
}

func TestNewLength(t *testing.T) {
	for _, length := range []int{0, 1, 4, 6, 8, 16, 32} {
		result := New(length)
		if len(result) != length {
			t.Errorf("New(%d) returned length %d", length, len(result))
		}
	}
}

func TestNewCharacterSet(t *testing.T) {
	result := New(100)
	matched, _ := regexp.MatchString(`^[A-Z0-9]+$`, result)
	if !matched {
		t.Errorf("expected uppercase alphanumeric, got '%s'", result)
	}
}

func TestNewUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := New(8)
		if seen[id] {
			t.Errorf("duplicate ID generated: '%s'", id)
		}
		seen[id] = true
	}
}

func TestNewHex(t *testing.T) {
	result := NewHex(16)
	if len(result) != 16 {
		t.Errorf("expected length 16, got %d", len(result))
	}
	matched, _ := regexp.MatchString(`^[0-9a-f]+$`, result)
	if !matched {
		t.Errorf("expected hex string, got '%s'", result)
	}
}

func TestNewHexLength(t *testing.T) {
	for _, length := range []int{0, 1, 4, 8, 16, 32} {
		result := NewHex(length)
		if len(result) != length {
			t.Errorf("NewHex(%d) returned length %d", length, len(result))
		}
	}
}

func TestNewUUID(t *testing.T) {
	result := NewUUID()
	// NewUUID returns 32 hex chars (no dashes)
	if len(result) != 32 {
		t.Errorf("expected length 32, got %d", len(result))
	}
	matched, _ := regexp.MatchString(`^[0-9a-f]{32}$`, result)
	if !matched {
		t.Errorf("expected hex string, got '%s'", result)
	}
	// Version nibble should be 4
	if result[12] != '4' {
		t.Errorf("expected version '4' at position 12, got '%c'", result[12])
	}
	// Variant bits should be 8, 9, a, or b
	if result[16] != '8' && result[16] != '9' && result[16] != 'a' && result[16] != 'b' {
		t.Errorf("expected variant nibble [89ab] at position 16, got '%c'", result[16])
	}
}

func TestNewUUIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewUUID()
		if seen[id] {
			t.Errorf("duplicate UUID: '%s'", id)
		}
		seen[id] = true
	}
}

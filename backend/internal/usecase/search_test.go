package usecase

import (
	"testing"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

func TestSearchYouTube(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.SearchFn = func(query string, limit int) ([]map[string]any, error) {
		return []map[string]any{{"title": "Result 1"}}, nil
	}
	cfg := &config.Config{}
	uc := NewSearchUsecase(mediaClient, cfg, testLog2)

	results, err := uc.SearchYouTube("test query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchYouTubeLimitClamping(t *testing.T) {
	var capturedLimit int
	mediaClient := repository.NewMockMediaClient()
	mediaClient.SearchFn = func(query string, limit int) ([]map[string]any, error) {
		capturedLimit = limit
		return nil, nil
	}
	cfg := &config.Config{}
	uc := NewSearchUsecase(mediaClient, cfg, testLog2)

	// Limit > 20 should clamp to 5
	uc.SearchYouTube("test", 25)
	if capturedLimit != 5 {
		t.Errorf("expected limit 5 for input 25, got %d", capturedLimit)
	}

	// Limit <= 0 should clamp to 5
	uc.SearchYouTube("test", 0)
	if capturedLimit != 5 {
		t.Errorf("expected limit 5 for input 0, got %d", capturedLimit)
	}

	// Limit < 0 should clamp to 5
	uc.SearchYouTube("test", -1)
	if capturedLimit != 5 {
		t.Errorf("expected limit 5 for input -1, got %d", capturedLimit)
	}

	// Normal limit should pass through
	uc.SearchYouTube("test", 10)
	if capturedLimit != 10 {
		t.Errorf("expected limit 10, got %d", capturedLimit)
	}
}

package usecase

import (
	"log/slog"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

type SearchUsecase struct {
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
}

func NewSearchUsecase(mediaClient repository.MediaClient, config *config.Config, log *slog.Logger) *SearchUsecase {
	return &SearchUsecase{mediaClient: mediaClient, config: config, log: log}
}

func (uc *SearchUsecase) SearchYouTube(query string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	return uc.mediaClient.Search(query, limit)
}

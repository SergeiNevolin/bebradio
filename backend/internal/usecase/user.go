package usecase

import (
	"log/slog"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

type UserUsecase struct {
	userRepo repository.UserRepository
	log      *slog.Logger
}

func NewUserUsecase(userRepo repository.UserRepository, log *slog.Logger) *UserUsecase {
	return &UserUsecase{userRepo: userRepo, log: log}
}

func (uc *UserUsecase) GetProfile(userID string) (*entity.User, error) {
	return uc.userRepo.FindByID(userID)
}

func (uc *UserUsecase) UpdateProfile(userID string, bio, avatarURL *string) (*entity.User, error) {
	return uc.userRepo.UpdateProfile(userID, bio, avatarURL)
}

func (uc *UserUsecase) GetUser(userID string) (*entity.User, error) {
	return uc.userRepo.FindByID(userID)
}

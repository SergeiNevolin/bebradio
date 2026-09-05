package repository

import "github.com/bebradio/backend-go/internal/domain/entity"

type UserRepository interface {
	Create(user *entity.User) error
	FindByID(id string) (*entity.User, error)
	FindByEmail(email string) (*entity.User, error)
	FindByUsername(username string) (*entity.User, error)
	UpdateProfile(id string, bio, avatarURL *string) (*entity.User, error)
}

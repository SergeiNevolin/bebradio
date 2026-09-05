package postgres

import (
	"context"
	"fmt"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(user *entity.User) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO users (id, email, username, password_hash, bio, avatar_url, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID, user.Email, user.Username, user.PasswordHash, user.Bio, user.AvatarURL, user.CreatedAt,
	)
	return err
}

func (r *UserRepo) FindByID(id string) (*entity.User, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, created_at
		 FROM users WHERE id = $1`, id,
	)
	return scanUser(row)
}

func (r *UserRepo) FindByEmail(email string) (*entity.User, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, created_at
		 FROM users WHERE email = $1`, email,
	)
	return scanUser(row)
}

func (r *UserRepo) FindByUsername(username string) (*entity.User, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, created_at
		 FROM users WHERE username = $1`, username,
	)
	return scanUser(row)
}

func (r *UserRepo) UpdateProfile(id string, bio, avatarURL *string) (*entity.User, error) {
	if bio != nil {
		r.pool.Exec(context.Background(),
			`UPDATE users SET bio = $1 WHERE id = $2`, *bio, id,
		)
	}
	if avatarURL != nil {
		r.pool.Exec(context.Background(),
			`UPDATE users SET avatar_url = $1 WHERE id = $2`, *avatarURL, id,
		)
	}
	return r.FindByID(id)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*entity.User, error) {
	u := &entity.User{}
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Bio, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

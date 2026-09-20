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
		`INSERT INTO users (id, email, username, password_hash, bio, avatar_url, role, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.Email, user.Username, user.PasswordHash, user.Bio, user.AvatarURL, user.Role, user.CreatedAt,
	)
	return err
}

func (r *UserRepo) FindByID(id string) (*entity.User, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, role, created_at
		 FROM users WHERE id = $1`, id,
	)
	return scanUser(row)
}

func (r *UserRepo) FindByEmail(email string) (*entity.User, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, role, created_at
		 FROM users WHERE email = $1`, email,
	)
	return scanUser(row)
}

func (r *UserRepo) FindByUsername(username string) (*entity.User, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, role, created_at
		 FROM users WHERE username = $1`, username,
	)
	return scanUser(row)
}

func (r *UserRepo) UpdateProfile(id string, bio, avatarURL *string) (*entity.User, error) {
	if bio != nil {
		if _, err := r.pool.Exec(context.Background(),
			`UPDATE users SET bio = $1 WHERE id = $2`, *bio, id,
		); err != nil {
			return nil, fmt.Errorf("update bio: %w", err)
		}
	}
	if avatarURL != nil {
		if _, err := r.pool.Exec(context.Background(),
			`UPDATE users SET avatar_url = $1 WHERE id = $2`, *avatarURL, id,
		); err != nil {
			return nil, fmt.Errorf("update avatar_url: %w", err)
		}
	}
	return r.FindByID(id)
}

func (r *UserRepo) SetRole(id string, role string) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE users SET role = $1 WHERE id = $2`, role, id,
	)
	return err
}

func (r *UserRepo) SearchByUsername(prefix string, limit int) ([]*entity.User, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, email, username, password_hash, bio, avatar_url, role, created_at
		 FROM users WHERE username ILIKE $1 ORDER BY username LIMIT $2`,
		prefix+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*entity.User
	for rows.Next() {
		u := &entity.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Bio, &u.AvatarURL, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) HasAdmin() (bool, error) {
	var count int
	err := r.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM users WHERE role = 'admin'`,
	).Scan(&count)
	return count > 0, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*entity.User, error) {
	u := &entity.User{}
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Bio, &u.AvatarURL, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

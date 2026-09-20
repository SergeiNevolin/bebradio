package entity

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Bio          string    `json:"bio"`
	AvatarURL    string    `json:"avatar_url"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) PublicProfile() map[string]any {
	return map[string]any{
		"id":         u.ID,
		"username":   u.Username,
		"bio":        u.Bio,
		"avatar_url": u.AvatarURL,
		"created_at": u.CreatedAt.Unix(),
	}
}

func (u *User) ProfileWithEmail() map[string]any {
	p := u.PublicProfile()
	p["email"] = u.Email
	p["role"] = u.Role
	return p
}

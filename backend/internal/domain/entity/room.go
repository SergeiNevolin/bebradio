package entity

import (
	"time"
)

const MaxChatMessages = 100

// Room is a thin metadata wrapper. All live state (queue, playback, votes,
// presence, messages) lives in Redis and is accessed through the redisc package.
type Room struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	OwnerID           string    `json:"owner_id"`
	CreatedAt         time.Time `json:"created_at"`
	AllowAnonymousAdd bool      `json:"allow_anonymous_add"`
	IsPrivate         bool      `json:"is_private"`
	PasswordHash      *string   `json:"-"`
	AutoRadio         bool      `json:"auto_radio"`
}

type PresenceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewRoom(id, name, ownerID string) *Room {
	return &Room{
		ID:                id,
		Name:              name,
		OwnerID:           ownerID,
		AllowAnonymousAdd: true,
		CreatedAt:         time.Now(),
	}
}

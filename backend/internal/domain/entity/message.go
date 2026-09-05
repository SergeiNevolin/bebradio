package entity

import "time"

type ChatMessage struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *ChatMessage) ToDict() map[string]any {
	return map[string]any{
		"id":         m.ID,
		"user_id":    m.UserID,
		"username":   m.Username,
		"text":       m.Text,
		"created_at": m.CreatedAt.Unix(),
	}
}

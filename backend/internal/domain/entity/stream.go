package entity

import (
	"strings"
	"time"
)

// Stream is an admin-run broadcast. It shares the live queue/playback runtime
// with rooms but has no voting, no password, and its own feature set.
type Stream struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	OwnerID   string         `json:"owner_id"`
	CreatedAt time.Time      `json:"created_at"`
	Features  StreamFeatures `json:"features"`
}

type StreamFeatures struct {
	MashupRadio MashupRadioFeature `json:"mashup_radio"`
}

type MashupRadioFeature struct {
	Enabled bool         `json:"enabled"`
	Filter  StreamFilter `json:"filter"`
}

// StreamFilter narrows the library tracks a Mashup Radio refiller may choose.
type StreamFilter struct {
	Query  string `json:"q" redis:"q"`
	Title  string `json:"title" redis:"title"`
	Artist string `json:"artist" redis:"artist"`
	Owner  string `json:"owner" redis:"owner"`
}

func (f StreamFilter) Empty() bool {
	return strings.TrimSpace(f.Query) == "" &&
		strings.TrimSpace(f.Title) == "" &&
		strings.TrimSpace(f.Artist) == "" &&
		strings.TrimSpace(f.Owner) == ""
}

func NewStream(id, name, ownerID string) *Stream {
	return &Stream{
		ID:        id,
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Now(),
		Features: StreamFeatures{
			MashupRadio: MashupRadioFeature{Enabled: false},
		},
	}
}

func (s *Stream) Ref() SessionRef {
	return NewStreamRef(s.ID)
}

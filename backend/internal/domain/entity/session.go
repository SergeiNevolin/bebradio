package entity

import "strings"

type SessionKind string

const (
	SessionKindRoom   SessionKind = "room"
	SessionKindStream SessionKind = "stream"
)

// SessionRef identifies one live session. Rooms and streams share the same
// queue/playback/presence/chat runtime but are distinct domain objects.
type SessionRef struct {
	Kind SessionKind `json:"kind"`
	ID   string      `json:"id"`
}

func NewRoomRef(id string) SessionRef {
	return SessionRef{Kind: SessionKindRoom, ID: strings.ToUpper(id)}
}

func NewStreamRef(id string) SessionRef {
	return SessionRef{Kind: SessionKindStream, ID: strings.ToUpper(id)}
}

func (r SessionRef) IsStream() bool {
	return r.Kind == SessionKindStream
}

func (r SessionRef) Valid() bool {
	return r.ID != "" && (r.Kind == SessionKindRoom || r.Kind == SessionKindStream)
}

func (r SessionRef) String() string {
	return string(r.Kind) + ":" + r.ID
}

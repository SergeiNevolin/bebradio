package domain

// This file defines the JSON shapes the browser sees. They are kept separate
// from the in-memory state so the wire format can be reasoned about on its own,
// and so no internal field is ever serialised by accident -- a room's password
// hash, in particular, must never leave the process.

// TrackDTO is a queue track as sent to clients. The playable URL is included;
// the source URL is what the client uses for the blurred video backdrop.
type TrackDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail"`
	Duration  int    `json:"duration"`
	AddedBy   string `json:"added_by"`
	SourceURL string `json:"source_url"`
}

// QueueEntryDTO is a queue track with its vote tally attached.
type QueueEntryDTO struct {
	TrackDTO
	Likes    int `json:"likes"`
	Dislikes int `json:"dislikes"`
}

// ChatMessageDTO is one chat line as sent to clients.
type ChatMessageDTO struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	Text      string  `json:"text"`
	CreatedAt float64 `json:"created_at"`
}

// RoomDTO is the full room state pushed over the WebSocket and returned by the
// room endpoints.
type RoomDTO struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	OwnerID           string           `json:"owner_id"`
	Queue             []QueueEntryDTO  `json:"queue"`
	CurrentIndex      int              `json:"current_index"`
	IsPlaying         bool             `json:"is_playing"`
	Position          float64          `json:"position"`
	CurrentTrack      *TrackDTO        `json:"current_track"`
	UserCount         int              `json:"user_count"`
	Listeners         []Listener       `json:"listeners"`
	AllowAnonymousAdd bool             `json:"allow_anonymous_add"`
	IsPrivate         bool             `json:"is_private"`
	AutoRadio         bool             `json:"auto_radio"`
	RadioSearching    bool             `json:"radio_searching"`
	HasPassword       bool             `json:"has_password"`
	TrackVotes        VoteCount        `json:"track_votes"`
	SkipVoters        []string         `json:"skip_voters"`
	Messages          []ChatMessageDTO `json:"messages"`

	// Access carries a room-access token to a caller who has proved they may
	// enter a password-protected room. It is omitted otherwise.
	Access string `json:"access,omitempty"`
}

// LockedRoomDTO is the stripped payload returned for a password-protected room
// the caller has not unlocked. It deliberately carries nothing but the room's
// name, so a locked room can be shown in the join prompt without leaking its
// queue, owner or chat.
type LockedRoomDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	HasPassword bool   `json:"has_password"`
	Locked      bool   `json:"locked"`
}

// RoomSummaryDTO is one entry in the public room list.
type RoomSummaryDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	UserCount   int    `json:"user_count"`
	TrackCount  int    `json:"track_count"`
	IsPlaying   bool   `json:"is_playing"`
	HasPassword bool   `json:"has_password"`
}

// UserDTO is a user profile as sent to clients. Email is included only on the
// caller's own profile.
type UserDTO struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Email     string  `json:"email,omitempty"`
	Bio       string  `json:"bio"`
	AvatarURL string  `json:"avatar_url"`
	CreatedAt float64 `json:"created_at"`
}

// DTO renders a track for the wire.
func (t *Track) DTO() TrackDTO {
	return TrackDTO{
		ID:        t.ID,
		Title:     t.Title,
		Artist:    t.Artist,
		URL:       t.URL,
		Thumbnail: t.Thumbnail,
		Duration:  t.Duration,
		AddedBy:   t.AddedBy,
		SourceURL: t.SourceURL,
	}
}

// DTO renders a chat message for the wire.
func (m ChatMessage) DTO() ChatMessageDTO {
	return ChatMessageDTO{
		ID:        m.ID,
		UserID:    m.UserID,
		Username:  m.Username,
		Text:      m.Text,
		CreatedAt: m.CreatedAt,
	}
}

// DTO renders a user profile. Set includeEmail only for the user's own profile.
func (u *User) DTO(includeEmail bool) UserDTO {
	out := UserDTO{
		ID:        u.ID,
		Username:  u.Username,
		Bio:       u.Bio,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt,
	}
	if includeEmail {
		out.Email = u.Email
	}
	return out
}

// Snapshot renders the whole room for the wire. The caller must hold the room's
// lock; the returned value shares nothing with the room, so it is safe to
// serialise and broadcast after the lock is released.
//
// maxMessages caps how much chat backlog travels with the snapshot.
func (r *Room) Snapshot(maxMessages int) RoomDTO {
	queue := make([]QueueEntryDTO, 0, len(r.Queue))
	for _, t := range r.Queue {
		votes := r.TrackVotes(t.ID)
		queue = append(queue, QueueEntryDTO{
			TrackDTO: t.DTO(),
			Likes:    votes.Likes,
			Dislikes: votes.Dislikes,
		})
	}

	var current *TrackDTO
	trackVotes := VoteCount{}
	if t := r.CurrentTrack(); t != nil {
		dto := t.DTO()
		current = &dto
		trackVotes = r.TrackVotes(t.ID)
	}

	listeners := r.Listeners()
	userCount := len(listeners)
	if userCount == 0 {
		// Before anyone has announced their presence, fall back to the accounts
		// seen on the room's connections.
		userCount = r.distinctUsers()
	}

	skipVoters := make([]string, 0, len(r.SkipVotes))
	for id := range r.SkipVotes {
		skipVoters = append(skipVoters, id)
	}
	sortStrings(skipVoters)

	messages := r.Messages
	if maxMessages > 0 && len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}
	msgDTOs := make([]ChatMessageDTO, 0, len(messages))
	for _, m := range messages {
		msgDTOs = append(msgDTOs, m.DTO())
	}

	return RoomDTO{
		ID:                r.ID,
		Name:              r.Name,
		OwnerID:           r.OwnerID,
		Queue:             queue,
		CurrentIndex:      r.CurrentIndex,
		IsPlaying:         r.IsPlaying,
		Position:          r.CurrentPosition(),
		CurrentTrack:      current,
		UserCount:         userCount,
		Listeners:         listeners,
		AllowAnonymousAdd: r.AllowAnonymousAdd,
		IsPrivate:         r.IsPrivate,
		AutoRadio:         r.AutoRadio,
		RadioSearching:    r.RadioFilling,
		HasPassword:       r.PasswordHash != "",
		TrackVotes:        trackVotes,
		SkipVoters:        skipVoters,
		Messages:          msgDTOs,
	}
}

// LockedSnapshot renders the stripped payload for a room the caller may not
// enter yet.
func (r *Room) LockedSnapshot() LockedRoomDTO {
	return LockedRoomDTO{ID: r.ID, Name: r.Name, HasPassword: true, Locked: true}
}

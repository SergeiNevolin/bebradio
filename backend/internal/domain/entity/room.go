package entity

import (
	"sync"
	"time"
)

const MaxChatMessages = 100

type Room struct {
	Mu sync.RWMutex `json:"-"`

	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	OwnerID           string                    `json:"owner_id"`
	Queue             []*Track                  `json:"-"`
	CurrentIndex      int                       `json:"current_index"`
	IsPlaying         bool                      `json:"is_playing"`
	Position          float64                   `json:"-"`
	LastSyncAt        time.Time                 `json:"-"`
	CreatedAt         time.Time                 `json:"created_at"`
	AllowAnonymousAdd bool                      `json:"allow_anonymous_add"`
	IsPrivate         bool                      `json:"is_private"`
	PasswordHash      *string                   `json:"-"`
	AutoRadio         bool                      `json:"auto_radio"`
	Messages          []*ChatMessage            `json:"-"`
	Votes             []*TrackVote              `json:"-"`
	SkipVotes         map[string]bool           `json:"-"`
	Presence          map[string]PresenceInfo   `json:"-"`
	Users             map[string]string         `json:"-"`
	LastAdvanceAt     time.Time                 `json:"-"`
	RadioSeedURL      string                    `json:"-"`
	RadioSeen         map[string]bool           `json:"-"`
	RadioFilling      bool                      `json:"radio_searching"`
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
		Queue:             make([]*Track, 0),
		Messages:          make([]*ChatMessage, 0),
		Votes:             make([]*TrackVote, 0),
		SkipVotes:         make(map[string]bool),
		Presence:          make(map[string]PresenceInfo),
		Users:             make(map[string]string),
		RadioSeen:         make(map[string]bool),
		AllowAnonymousAdd: true,
		CreatedAt:         time.Now(),
	}
}

func (r *Room) CurrentTrack() *Track {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	return r.CurrentTrackUnlocked()
}

func (r *Room) CurrentTrackUnlocked() *Track {
	if len(r.Queue) == 0 || r.CurrentIndex < 0 || r.CurrentIndex >= len(r.Queue) {
		return nil
	}
	return r.Queue[r.CurrentIndex]
}

func (r *Room) GetCurrentPosition() float64 {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.IsPlaying {
		elapsed := time.Since(r.LastSyncAt).Seconds()
		return r.Position + elapsed
	}
	return r.Position
}

func (r *Room) GetTrackVotes(trackID string) (likes, dislikes int) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	for _, v := range r.Votes {
		if v.TrackID == trackID {
			if v.Vote == 1 {
				likes++
			} else if v.Vote == -1 {
				dislikes++
			}
		}
	}
	return
}

func (r *Room) Listeners() []map[string]any {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	seen := make(map[string]string)
	for _, info := range r.Presence {
		seen[info.ID] = info.Name
	}
	listeners := make([]map[string]any, 0, len(seen))
	for uid, name := range seen {
		listeners = append(listeners, map[string]any{"id": uid, "name": name})
	}
	return listeners
}

func (r *Room) ToDict() map[string]any {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	track := r.CurrentTrackUnlocked()

	voteTotals := make(map[string][2]int)
	for _, v := range r.Votes {
		t := voteTotals[v.TrackID]
		if v.Vote == 1 {
			t[0]++
		} else if v.Vote == -1 {
			t[1]++
		}
		voteTotals[v.TrackID] = t
	}

	var trackVotes [2]int
	if track != nil {
		trackVotes = voteTotals[track.ID]
	}

	queueWithVotes := make([]map[string]any, 0, len(r.Queue))
	for _, t := range r.Queue {
		entry := t.ToDict()
		tv := voteTotals[t.ID]
		entry["likes"] = tv[0]
		entry["dislikes"] = tv[1]
		queueWithVotes = append(queueWithVotes, entry)
	}

	listeners := r.ListenersUnlocked()
	userCount := len(listeners)
	if userCount == 0 {
		seen := make(map[string]string)
		for _, name := range r.Users {
			seen[name] = name
		}
		userCount = len(seen)
	}

	var currentTrackDict map[string]any
	if track != nil {
		currentTrackDict = track.ToDict()
		cv := voteTotals[track.ID]
		currentTrackDict["likes"] = cv[0]
		currentTrackDict["dislikes"] = cv[1]
	}

	skipVoters := make([]string, 0, len(r.SkipVotes))
	for uid := range r.SkipVotes {
		skipVoters = append(skipVoters, uid)
	}

	msgs := r.Messages
	if len(msgs) > MaxChatMessages {
		msgs = msgs[len(msgs)-MaxChatMessages:]
	}
	msgDicts := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		msgDicts = append(msgDicts, m.ToDict())
	}

	result := map[string]any{
		"id":                  r.ID,
		"name":                r.Name,
		"owner_id":            r.OwnerID,
		"queue":               queueWithVotes,
		"current_index":       r.CurrentIndex,
		"is_playing":          r.IsPlaying,
		"position":            r.PositionUnlocked(),
		"user_count":          userCount,
		"listeners":           listeners,
		"allow_anonymous_add": r.AllowAnonymousAdd,
		"is_private":          r.IsPrivate,
		"auto_radio":          r.AutoRadio,
		"radio_searching":     r.RadioFilling,
		"has_password":        r.PasswordHash != nil,
		"track_votes":         map[string]int{"likes": trackVotes[0], "dislikes": trackVotes[1]},
		"skip_voters":         skipVoters,
		"messages":            msgDicts,
	}

	if track != nil {
		result["current_track"] = currentTrackDict
	}

	return result
}

func (r *Room) PositionUnlocked() float64 {
	if r.IsPlaying {
		elapsed := time.Since(r.LastSyncAt).Seconds()
		return r.Position + elapsed
	}
	return r.Position
}

func (r *Room) ListenersUnlocked() []map[string]any {
	seen := make(map[string]string)
	for _, info := range r.Presence {
		seen[info.ID] = info.Name
	}
	listeners := make([]map[string]any, 0, len(seen))
	for uid, name := range seen {
		listeners = append(listeners, map[string]any{"id": uid, "name": name})
	}
	return listeners
}

package entity

type TrackVote struct {
	UserID  string `json:"user_id"`
	TrackID string `json:"track_id"`
	Vote    int    `json:"vote"` // 1 = like, -1 = dislike
}

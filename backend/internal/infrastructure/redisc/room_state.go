package redisc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

// Redis key layout for room {id}:
//
//	room:{id}:queue          LIST   — JSON-encoded Track objects in play order
//	room:{id}:state          HASH   — current_index, is_playing, position,
//	                                  last_sync_at, current_started_at,
//	                                  last_advance_at, radio_seed_url,
//	                                  radio_filling
//	room:{id}:votes          HASH   — track_id → JSON VoteEntry
//	room:{id}:skip_votes     SET    — user IDs who voted to skip
//	room:{id}:messages       LIST   — JSON-encoded ChatMessage (capped at 100)
//	room:{id}:presence       HASH   — conn_addr → JSON PresenceInfo
//	room:{id}:radio_seen     SET    — media IDs already seeded

func RoomKey(id, suffix string) string {
	return "room:" + id + ":" + suffix
}

// ---------- Types ----------

type PlaybackState struct {
	CurrentIndex     int
	IsPlaying        bool
	Position         float64
	LastSyncAt       time.Time
	CurrentStartedAt time.Time
	LastAdvanceAt    time.Time
	RadioSeedURL     string
	RadioFilling     bool
}

type VoteEntry struct {
	LikedBy  []string `json:"liked_by,omitempty"`
	Disliked []string `json:"disliked_by,omitempty"`
	// Likes is a legacy unattributed counter from before per-user likes.
	// Kept for reading old data; new votes go into LikedBy.
	Likes int `json:"likes,omitempty"`
}

// LikeCount returns the total like count (legacy base + per-user likes).
func (v *VoteEntry) LikeCount() int {
	if v == nil {
		return 0
	}
	return v.Likes + len(v.LikedBy)
}

// DislikeCount returns the total dislike count.
func (v *VoteEntry) DislikeCount() int {
	if v == nil {
		return 0
	}
	return len(v.Disliked)
}

// ApplyVote sets a user's vote with set semantics, idempotent per user:
// 1 = like, -1 = dislike, anything else = retract previous vote.
func (v *VoteEntry) ApplyVote(userID string, vote int) {
	v.LikedBy = removeString(v.LikedBy, userID)
	v.Disliked = removeString(v.Disliked, userID)
	switch vote {
	case 1:
		v.LikedBy = append(v.LikedBy, userID)
	case -1:
		v.Disliked = append(v.Disliked, userID)
	}
}

func removeString(slice []string, val string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			result = append(result, s)
		}
	}
	return result
}

// ---------- Queue ----------

func GetQueueLen(ctx context.Context, rdb *redis.Client, roomID string) (int64, error) {
	return rdb.LLen(ctx, RoomKey(roomID, "queue")).Result()
}

func GetQueue(ctx context.Context, rdb *redis.Client, roomID string) ([]*entity.Track, error) {
	items, err := rdb.LRange(ctx, RoomKey(roomID, "queue"), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange queue: %w", err)
	}
	tracks := make([]*entity.Track, 0, len(items))
	for _, raw := range items {
		t := &entity.Track{}
		if err := json.Unmarshal([]byte(raw), t); err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func SetQueue(ctx context.Context, rdb *redis.Client, roomID string, tracks []*entity.Track) error {
	pipe := rdb.Pipeline()
	key := RoomKey(roomID, "queue")
	pipe.Del(ctx, key)
	if len(tracks) > 0 {
		vals := make([]any, len(tracks))
		for i, t := range tracks {
			b, _ := json.Marshal(t)
			vals[i] = b
		}
		pipe.RPush(ctx, key, vals...)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func AppendTrack(ctx context.Context, rdb *redis.Client, roomID string, track *entity.Track) error {
	b, err := json.Marshal(track)
	if err != nil {
		return err
	}
	return rdb.RPush(ctx, RoomKey(roomID, "queue"), b).Err()
}

// AppendFreshTrack appends track unless it is already queued (same ID) or
// radio-seen (same media). Successfully appended tracks are marked seen.
// Returns true if appended.
//
// This closes the Refill-vs-skip race: Refill snapshots the queue and then
// does slow network I/O, so a skip landing mid-flight must not be undone by
// a stale append (the last-track-never-skips bug). The check-act window here
// holds no I/O, only fast Redis ops.
func AppendFreshTrack(ctx context.Context, rdb *redis.Client, roomID string, track *entity.Track) (bool, error) {
	if track.MediaID != "" {
		seen, err := IsRadioSeen(ctx, rdb, roomID, track.MediaID)
		if err != nil {
			return false, err
		}
		if seen {
			return false, nil
		}
	}
	queue, err := GetQueue(ctx, rdb, roomID)
	if err != nil {
		return false, err
	}
	for _, t := range queue {
		if t.ID == track.ID {
			return false, nil
		}
	}
	if err := AppendTrack(ctx, rdb, roomID, track); err != nil {
		return false, err
	}
	if track.MediaID != "" {
		if err := AddRadioSeen(ctx, rdb, roomID, track.MediaID); err != nil {
			return false, err
		}
	}
	return true, nil
}

// SetQueueAt replaces the track at index with an updated copy.
// Callers must verify index+ID against a fresh read first: the queue may
// have shifted (skip/remove) between their read and this write.
func SetQueueAt(ctx context.Context, rdb *redis.Client, roomID string, index int, track *entity.Track) error {
	b, err := json.Marshal(track)
	if err != nil {
		return err
	}
	return rdb.LSet(ctx, RoomKey(roomID, "queue"), int64(index), b).Err()
}

func RemoveAt(ctx context.Context, rdb *redis.Client, roomID string, index int) (int, error) {
	key := RoomKey(roomID, "queue")
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if index < 0 || int64(index) >= length {
		return int(length), nil
	}
	// Use LSET+LREM: set to a sentinel, then remove it.
	sentinel := fmt.Sprintf("__del__%d__%d", time.Now().UnixNano(), index)
	if err := rdb.LSet(ctx, key, int64(index), sentinel).Err(); err != nil {
		return 0, err
	}
	if err := rdb.LRem(ctx, key, 1, sentinel).Err(); err != nil {
		return 0, err
	}
	newLen, _ := rdb.LLen(ctx, key).Result()
	return int(newLen), nil
}

// ---------- Playback state ----------

func GetPlayback(ctx context.Context, rdb *redis.Client, roomID string) (*PlaybackState, error) {
	vals, err := rdb.HGetAll(ctx, RoomKey(roomID, "state")).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall state: %w", err)
	}
	if len(vals) == 0 {
		return nil, nil
	}
	ps := &PlaybackState{}
	if v, ok := vals["current_index"]; ok {
		ps.CurrentIndex, _ = strconv.Atoi(v)
	}
	if v, ok := vals["is_playing"]; ok {
		ps.IsPlaying = v == "1"
	}
	if v, ok := vals["position"]; ok {
		ps.Position, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := vals["last_sync_at"]; ok {
		ps.LastSyncAt, _ = time.Parse(time.RFC3339Nano, v)
	}
	if v, ok := vals["current_started_at"]; ok {
		ps.CurrentStartedAt, _ = time.Parse(time.RFC3339Nano, v)
	}
	if v, ok := vals["last_advance_at"]; ok {
		ps.LastAdvanceAt, _ = time.Parse(time.RFC3339Nano, v)
	}
	if v, ok := vals["radio_seed_url"]; ok {
		ps.RadioSeedURL = v
	}
	if v, ok := vals["radio_filling"]; ok {
		ps.RadioFilling = v == "1"
	}
	return ps, nil
}

func SetPlayback(ctx context.Context, rdb *redis.Client, roomID string, ps *PlaybackState) error {
	vals := map[string]any{
		"current_index":      strconv.Itoa(ps.CurrentIndex),
		"is_playing":         boolToInt(ps.IsPlaying),
		"position":           strconv.FormatFloat(ps.Position, 'f', -1, 64),
		"last_sync_at":       ps.LastSyncAt.Format(time.RFC3339Nano),
		"current_started_at": ps.CurrentStartedAt.Format(time.RFC3339Nano),
		"last_advance_at":    ps.LastAdvanceAt.Format(time.RFC3339Nano),
		"radio_seed_url":     ps.RadioSeedURL,
		"radio_filling":      boolToInt(ps.RadioFilling),
	}
	return rdb.HSet(ctx, RoomKey(roomID, "state"), vals).Err()
}

func SetPlaybackField(ctx context.Context, rdb *redis.Client, roomID, field, value string) error {
	return rdb.HSet(ctx, RoomKey(roomID, "state"), field, value).Err()
}

// ---------- Votes ----------

func GetVotes(ctx context.Context, rdb *redis.Client, roomID string) (map[string]*VoteEntry, error) {
	vals, err := rdb.HGetAll(ctx, RoomKey(roomID, "votes")).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall votes: %w", err)
	}
	result := make(map[string]*VoteEntry, len(vals))
	for trackID, raw := range vals {
		v := &VoteEntry{}
		if err := json.Unmarshal([]byte(raw), v); err != nil {
			continue
		}
		result[trackID] = v
	}
	return result, nil
}

func SetVote(ctx context.Context, rdb *redis.Client, roomID, trackID string, entry *VoteEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return rdb.HSet(ctx, RoomKey(roomID, "votes"), trackID, b).Err()
}

// DelVote removes the vote entry for a track that left the queue,
// so a re-added track with the same ID starts with a clean slate.
func DelVote(ctx context.Context, rdb *redis.Client, roomID, trackID string) error {
	return rdb.HDel(ctx, RoomKey(roomID, "votes"), trackID).Err()
}

// ---------- Skip votes ----------

// ToggleSkipVote toggles a user's skip vote. Returns the new count.
func ToggleSkipVote(ctx context.Context, rdb *redis.Client, roomID, userID string) (int64, error) {
	key := RoomKey(roomID, "skip_votes")
	added, err := rdb.SAdd(ctx, key, userID).Result()
	if err != nil {
		return 0, err
	}
	if added == 0 {
		// Member existed → remove it (toggle off).
		rdb.SRem(ctx, key, userID)
	}
	return rdb.SCard(ctx, key).Result()
}

func ResetSkipVotes(ctx context.Context, rdb *redis.Client, roomID string) error {
	return rdb.Del(ctx, RoomKey(roomID, "skip_votes")).Err()
}

func GetSkipVoteCount(ctx context.Context, rdb *redis.Client, roomID string) (int64, error) {
	return rdb.SCard(ctx, RoomKey(roomID, "skip_votes")).Result()
}

// ---------- Messages ----------

func AppendMessage(ctx context.Context, rdb *redis.Client, roomID string, msg *entity.ChatMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	key := RoomKey(roomID, "messages")
	if err := rdb.RPush(ctx, key, b).Err(); err != nil {
		return err
	}
	rdb.LTrim(ctx, key, -100, -1)
	return nil
}

func GetMessages(ctx context.Context, rdb *redis.Client, roomID string) ([]*entity.ChatMessage, error) {
	items, err := rdb.LRange(ctx, RoomKey(roomID, "messages"), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange messages: %w", err)
	}
	msgs := make([]*entity.ChatMessage, 0, len(items))
	for _, raw := range items {
		m := &entity.ChatMessage{}
		if err := json.Unmarshal([]byte(raw), m); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// ---------- Presence ----------

func SetPresence(ctx context.Context, rdb *redis.Client, roomID, connAddr string, info *entity.PresenceInfo) error {
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return rdb.HSet(ctx, RoomKey(roomID, "presence"), connAddr, b).Err()
}

func RemovePresence(ctx context.Context, rdb *redis.Client, roomID, connAddr string) error {
	return rdb.HDel(ctx, RoomKey(roomID, "presence"), connAddr).Err()
}

func GetPresence(ctx context.Context, rdb *redis.Client, roomID string) (map[string]*entity.PresenceInfo, error) {
	vals, err := rdb.HGetAll(ctx, RoomKey(roomID, "presence")).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall presence: %w", err)
	}
	result := make(map[string]*entity.PresenceInfo, len(vals))
	for addr, raw := range vals {
		info := &entity.PresenceInfo{}
		if err := json.Unmarshal([]byte(raw), info); err != nil {
			continue
		}
		result[addr] = info
	}
	return result, nil
}

func GetPresenceCount(ctx context.Context, rdb *redis.Client, roomID string) (int64, error) {
	return rdb.HLen(ctx, RoomKey(roomID, "presence")).Result()
}

// ---------- Radio seen ----------

func AddRadioSeen(ctx context.Context, rdb *redis.Client, roomID, mediaID string) error {
	return rdb.SAdd(ctx, RoomKey(roomID, "radio_seen"), mediaID).Err()
}

func IsRadioSeen(ctx context.Context, rdb *redis.Client, roomID, mediaID string) (bool, error) {
	return rdb.SIsMember(ctx, RoomKey(roomID, "radio_seen"), mediaID).Result()
}

// GetRadioSeen returns all media IDs the radio must not recommend again.
func GetRadioSeen(ctx context.Context, rdb *redis.Client, roomID string) (map[string]bool, error) {
	members, err := rdb.SMembers(ctx, RoomKey(roomID, "radio_seen")).Result()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		seen[m] = true
	}
	return seen, nil
}

// ---------- Cleanup ----------

func DeleteRoom(ctx context.Context, rdb *redis.Client, roomID string) error {
	iter := rdb.Scan(ctx, 0, RoomKey(roomID, "*"), 100).Iterator()
	for iter.Next(ctx) {
		rdb.Del(ctx, iter.Val())
	}
	return iter.Err()
}

// ---------- High-level helpers ----------

// BuildToDict assembles the full room state dict from Redis for broadcasting.
func BuildToDict(ctx context.Context, rdb *redis.Client, rm *entity.Room) map[string]any {
	tracks, _ := GetQueue(ctx, rdb, rm.ID)
	ps, _ := GetPlayback(ctx, rdb, rm.ID)
	votes, _ := GetVotes(ctx, rdb, rm.ID)
	presence, _ := GetPresence(ctx, rdb, rm.ID)
	messages, _ := GetMessages(ctx, rdb, rm.ID)

	if ps == nil {
		ps = &PlaybackState{}
	}
	if tracks == nil {
		tracks = []*entity.Track{}
	}
	if votes == nil {
		votes = make(map[string]*VoteEntry)
	}
	if presence == nil {
		presence = make(map[string]*entity.PresenceInfo)
	}
	if messages == nil {
		messages = []*entity.ChatMessage{}
	}

	voteTotals := make(map[string][2]int)
	for trackID, v := range votes {
		voteTotals[trackID] = [2]int{v.LikeCount(), v.DislikeCount()}
	}

	var trackVotes [2]int
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		trackVotes = voteTotals[tracks[ps.CurrentIndex].ID]
	}

	queueWithVotes := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		entry := t.ToDict()
		tv := voteTotals[t.ID]
		entry["likes"] = tv[0]
		entry["dislikes"] = tv[1]
		queueWithVotes = append(queueWithVotes, entry)
	}

	seen := make(map[string]string)
	for _, info := range presence {
		seen[info.ID] = info.Name
	}
	listeners := make([]map[string]any, 0, len(seen))
	for uid, name := range seen {
		listeners = append(listeners, map[string]any{"id": uid, "name": name})
	}

	position := ps.Position
	if ps.IsPlaying {
		position += time.Since(ps.LastSyncAt).Seconds()
	}

	var currentTrackDict map[string]any
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		currentTrackDict = tracks[ps.CurrentIndex].ToDict()
		cv := voteTotals[tracks[ps.CurrentIndex].ID]
		currentTrackDict["likes"] = cv[0]
		currentTrackDict["dislikes"] = cv[1]
	}

	msgs := messages
	if len(msgs) > entity.MaxChatMessages {
		msgs = msgs[len(msgs)-entity.MaxChatMessages:]
	}
	msgDicts := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		msgDicts = append(msgDicts, m.ToDict())
	}

	skipVoters, _ := rdb.SMembers(ctx, RoomKey(rm.ID, "skip_votes")).Result()

	result := map[string]any{
		"id":                  rm.ID,
		"name":                rm.Name,
		"owner_id":            rm.OwnerID,
		"queue":               queueWithVotes,
		"current_index":       ps.CurrentIndex,
		"is_playing":          ps.IsPlaying,
		"position":            position,
		"user_count":          len(listeners),
		"listeners":           listeners,
		"allow_anonymous_add": rm.AllowAnonymousAdd,
		"is_private":          rm.IsPrivate,
		"auto_radio":          rm.AutoRadio,
		"radio_searching":     ps.RadioFilling,
		"has_password":        rm.PasswordHash != nil,
		"track_votes":         map[string]int{"likes": trackVotes[0], "dislikes": trackVotes[1]},
		"skip_voters":         skipVoters,
		"messages":            msgDicts,
	}

	if currentTrackDict != nil {
		result["current_track"] = currentTrackDict
	}

	return result
}

// CurrentTrack loads the current track from Redis.
func CurrentTrack(ctx context.Context, rdb *redis.Client, roomID string) *entity.Track {
	tracks, _ := GetQueue(ctx, rdb, roomID)
	ps, _ := GetPlayback(ctx, rdb, roomID)
	if ps == nil || len(tracks) == 0 {
		return nil
	}
	idx := ps.CurrentIndex
	if idx < 0 || idx >= len(tracks) {
		return nil
	}
	return tracks[idx]
}

// GetCurrentPosition returns the wall-clock playback position.
func GetCurrentPosition(ctx context.Context, rdb *redis.Client, roomID string) float64 {
	ps, _ := GetPlayback(ctx, rdb, roomID)
	if ps == nil {
		return 0
	}
	if ps.IsPlaying {
		elapsed := time.Since(ps.LastSyncAt).Seconds()
		return ps.Position + elapsed
	}
	return ps.Position
}

// GetTrackVotes returns likes/dislikes for a track from Redis.
func GetTrackVotes(ctx context.Context, rdb *redis.Client, roomID, trackID string) (likes, dislikes int) {
	votes, _ := GetVotes(ctx, rdb, roomID)
	if v, ok := votes[trackID]; ok {
		return v.LikeCount(), v.DislikeCount()
	}
	return 0, 0
}

// ---------- Helpers ----------

func boolToInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

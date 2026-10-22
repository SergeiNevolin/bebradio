package redisc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

// Redis key layout for any session:
//
//	session:{kind}:{id}:queue          LIST   JSON-encoded Tracks
//	session:{kind}:{id}:state          HASH   playback fields
//	session:{kind}:{id}:votes          HASH   room-only track votes
//	session:{kind}:{id}:skip_votes     SET    room-only skip votes
//	session:{kind}:{id}:messages       LIST   JSON-encoded ChatMessage
//	session:{kind}:{id}:presence       HASH   conn_addr -> PresenceInfo
//	session:{kind}:{id}:radio_seen     SET    media IDs already seeded
//	session:{kind}:{id}:feature:mashup-radio HASH enabled + filter
//
// kind is "room" or "stream". String arguments are accepted for compatibility
// and normalized to room sessions.

func AsRef(v any) entity.SessionRef {
	switch x := v.(type) {
	case entity.SessionRef:
		ref := x
		if ref.Kind == "" {
			ref.Kind = entity.SessionKindRoom
		}
		ref.ID = strings.ToUpper(ref.ID)
		return ref
	case *entity.SessionRef:
		if x == nil {
			return entity.SessionRef{}
		}
		return AsRef(*x)
	case string:
		return entity.NewRoomRef(x)
	default:
		return entity.SessionRef{}
	}
}

func SessionKey(session any, suffix string) string {
	ref := AsRef(session)
	return "session:" + string(ref.Kind) + ":" + ref.ID + ":" + suffix
}

func RoomKey(id, suffix string) string {
	return SessionKey(entity.NewRoomRef(id), suffix)
}

func StreamKey(id, suffix string) string {
	return SessionKey(entity.NewStreamRef(id), suffix)
}

// SessionRefFromKey parses "session:{kind}:{id}:{suffix}".
func SessionRefFromKey(key string) (entity.SessionRef, string, bool) {
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 || parts[0] != "session" {
		return entity.SessionRef{}, "", false
	}
	kind := entity.SessionKind(parts[1])
	if kind != entity.SessionKindRoom && kind != entity.SessionKindStream {
		return entity.SessionRef{}, "", false
	}
	return entity.SessionRef{Kind: kind, ID: strings.ToUpper(parts[2])}, parts[3], true
}

// Playback hash field names, shared so writers and readers cannot drift.
const (
	FieldRadioFilling = "radio_filling"
	FieldRadioSeedURL = "radio_seed_url"
)

const FeatureMashupRadio = "feature:mashup-radio"

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
	Likes    int      `json:"likes,omitempty"`
}

func (v *VoteEntry) LikeCount() int {
	if v == nil {
		return 0
	}
	return v.Likes + len(v.LikedBy)
}

func (v *VoteEntry) DislikeCount() int {
	if v == nil {
		return 0
	}
	return len(v.Disliked)
}

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

func GetQueueLen(ctx context.Context, rdb *redis.Client, session any) (int64, error) {
	return rdb.LLen(ctx, SessionKey(session, "queue")).Result()
}

func GetQueue(ctx context.Context, rdb *redis.Client, session any) ([]*entity.Track, error) {
	items, err := rdb.LRange(ctx, SessionKey(session, "queue"), 0, -1).Result()
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

func SetQueue(ctx context.Context, rdb *redis.Client, session any, tracks []*entity.Track) error {
	pipe := rdb.Pipeline()
	key := SessionKey(session, "queue")
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

func AppendTrack(ctx context.Context, rdb *redis.Client, session any, track *entity.Track) error {
	b, err := json.Marshal(track)
	if err != nil {
		return err
	}
	return rdb.RPush(ctx, SessionKey(session, "queue"), b).Err()
}

func AppendFreshTrack(ctx context.Context, rdb *redis.Client, session any, track *entity.Track) (bool, error) {
	if track.MediaID != "" {
		seen, err := IsRadioSeen(ctx, rdb, session, track.MediaID)
		if err != nil {
			return false, err
		}
		if seen {
			return false, nil
		}
	}
	queue, err := GetQueue(ctx, rdb, session)
	if err != nil {
		return false, err
	}
	for _, t := range queue {
		if t.ID == track.ID {
			return false, nil
		}
	}
	if err := AppendTrack(ctx, rdb, session, track); err != nil {
		return false, err
	}
	if track.MediaID != "" {
		if err := AddRadioSeen(ctx, rdb, session, track.MediaID); err != nil {
			return false, err
		}
	}
	return true, nil
}

func SetQueueAt(ctx context.Context, rdb *redis.Client, session any, index int, track *entity.Track) error {
	b, err := json.Marshal(track)
	if err != nil {
		return err
	}
	return rdb.LSet(ctx, SessionKey(session, "queue"), int64(index), b).Err()
}

func RemoveAt(ctx context.Context, rdb *redis.Client, session any, index int) (int, error) {
	key := SessionKey(session, "queue")
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if index < 0 || int64(index) >= length {
		return int(length), nil
	}
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

func GetPlayback(ctx context.Context, rdb *redis.Client, session any) (*PlaybackState, error) {
	vals, err := rdb.HGetAll(ctx, SessionKey(session, "state")).Result()
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

func SetPlayback(ctx context.Context, rdb *redis.Client, session any, ps *PlaybackState) error {
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
	return rdb.HSet(ctx, SessionKey(session, "state"), vals).Err()
}

func SetPlaybackField(ctx context.Context, rdb *redis.Client, session any, field, value string) error {
	return rdb.HSet(ctx, SessionKey(session, "state"), field, value).Err()
}

// ---------- Votes (rooms only) ----------

func GetVotes(ctx context.Context, rdb *redis.Client, session any) (map[string]*VoteEntry, error) {
	vals, err := rdb.HGetAll(ctx, SessionKey(session, "votes")).Result()
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

func SetVote(ctx context.Context, rdb *redis.Client, session any, trackID string, entry *VoteEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return rdb.HSet(ctx, SessionKey(session, "votes"), trackID, b).Err()
}

func DelVote(ctx context.Context, rdb *redis.Client, session any, trackID string) error {
	return rdb.HDel(ctx, SessionKey(session, "votes"), trackID).Err()
}

// ---------- Skip votes (rooms only) ----------

func ToggleSkipVote(ctx context.Context, rdb *redis.Client, session any, userID string) (int64, error) {
	key := SessionKey(session, "skip_votes")
	added, err := rdb.SAdd(ctx, key, userID).Result()
	if err != nil {
		return 0, err
	}
	if added == 0 {
		rdb.SRem(ctx, key, userID)
	}
	return rdb.SCard(ctx, key).Result()
}

func ResetSkipVotes(ctx context.Context, rdb *redis.Client, session any) error {
	return rdb.Del(ctx, SessionKey(session, "skip_votes")).Err()
}

func GetSkipVoteCount(ctx context.Context, rdb *redis.Client, session any) (int64, error) {
	return rdb.SCard(ctx, SessionKey(session, "skip_votes")).Result()
}

// ---------- Messages ----------

func AppendMessage(ctx context.Context, rdb *redis.Client, session any, msg *entity.ChatMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	key := SessionKey(session, "messages")
	if err := rdb.RPush(ctx, key, b).Err(); err != nil {
		return err
	}
	rdb.LTrim(ctx, key, -100, -1)
	return nil
}

func GetMessages(ctx context.Context, rdb *redis.Client, session any) ([]*entity.ChatMessage, error) {
	items, err := rdb.LRange(ctx, SessionKey(session, "messages"), 0, -1).Result()
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

func SetPresence(ctx context.Context, rdb *redis.Client, session any, connAddr string, info *entity.PresenceInfo) error {
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return rdb.HSet(ctx, SessionKey(session, "presence"), connAddr, b).Err()
}

func RemovePresence(ctx context.Context, rdb *redis.Client, session any, connAddr string) error {
	return rdb.HDel(ctx, SessionKey(session, "presence"), connAddr).Err()
}

func GetPresence(ctx context.Context, rdb *redis.Client, session any) (map[string]*entity.PresenceInfo, error) {
	vals, err := rdb.HGetAll(ctx, SessionKey(session, "presence")).Result()
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

func GetPresenceCount(ctx context.Context, rdb *redis.Client, session any) (int64, error) {
	return rdb.HLen(ctx, SessionKey(session, "presence")).Result()
}

// ---------- Radio seen ----------

func AddRadioSeen(ctx context.Context, rdb *redis.Client, session any, mediaID string) error {
	return rdb.SAdd(ctx, SessionKey(session, "radio_seen"), mediaID).Err()
}

func IsRadioSeen(ctx context.Context, rdb *redis.Client, session any, mediaID string) (bool, error) {
	return rdb.SIsMember(ctx, SessionKey(session, "radio_seen"), mediaID).Result()
}

func GetRadioSeen(ctx context.Context, rdb *redis.Client, session any) (map[string]bool, error) {
	members, err := rdb.SMembers(ctx, SessionKey(session, "radio_seen")).Result()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		seen[m] = true
	}
	return seen, nil
}

func ResetRadioSeen(ctx context.Context, rdb *redis.Client, session any) error {
	return rdb.Del(ctx, SessionKey(session, "radio_seen")).Err()
}

// ---------- Idle tracking (rooms only) ----------

func GetIdleSince(ctx context.Context, rdb *redis.Client, roomID string) (time.Time, bool) {
	v, err := rdb.Get(ctx, RoomKey(roomID, "idle_since")).Result()
	if err != nil {
		return time.Time{}, false
	}
	ts, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(ts, 0), true
}

func SetIdleSince(ctx context.Context, rdb *redis.Client, roomID string, ts time.Time) error {
	return rdb.Set(ctx, RoomKey(roomID, "idle_since"), strconv.FormatInt(ts.Unix(), 10), 0).Err()
}

func ClearIdleSince(ctx context.Context, rdb *redis.Client, roomID string) error {
	return rdb.Del(ctx, RoomKey(roomID, "idle_since")).Err()
}

// ---------- Mashup Radio feature ----------

// StreamFilter is kept as an alias so existing callers can keep using the
// Redis package while filter semantics live in the domain.
type StreamFilter = entity.StreamFilter

func GetMashupRadioFeature(ctx context.Context, rdb *redis.Client, session any) (entity.MashupRadioFeature, error) {
	vals, err := rdb.HGetAll(ctx, SessionKey(session, FeatureMashupRadio)).Result()
	if err != nil {
		return entity.MashupRadioFeature{}, err
	}
	if len(vals) == 0 {
		return entity.MashupRadioFeature{}, nil
	}
	return entity.MashupRadioFeature{
		Enabled: vals["enabled"] == "1",
		Filter: entity.StreamFilter{
			Query:  vals["q"],
			Title:  vals["title"],
			Artist: vals["artist"],
			Owner:  vals["owner"],
		},
	}, nil
}

func SetMashupRadioFeature(ctx context.Context, rdb *redis.Client, session any, feature entity.MashupRadioFeature) error {
	return rdb.HSet(ctx, SessionKey(session, FeatureMashupRadio), map[string]any{
		"enabled": boolToInt(feature.Enabled),
		"q":       feature.Filter.Query,
		"title":   feature.Filter.Title,
		"artist":  feature.Filter.Artist,
		"owner":   feature.Filter.Owner,
	}).Err()
}

func GetStreamFilter(ctx context.Context, rdb *redis.Client, session any) (*entity.StreamFilter, error) {
	feature, err := GetMashupRadioFeature(ctx, rdb, session)
	if err != nil {
		return nil, err
	}
	return &feature.Filter, nil
}

func SetStreamFilter(ctx context.Context, rdb *redis.Client, session any, f *entity.StreamFilter) error {
	feature, err := GetMashupRadioFeature(ctx, rdb, session)
	if err != nil {
		return err
	}
	if f != nil {
		feature.Filter = *f
	}
	return SetMashupRadioFeature(ctx, rdb, session, feature)
}

// ---------- Cleanup ----------

func DeleteSession(ctx context.Context, rdb *redis.Client, session any) error {
	iter := rdb.Scan(ctx, 0, SessionKey(session, "*"), 100).Iterator()
	for iter.Next(ctx) {
		rdb.Del(ctx, iter.Val())
	}
	return iter.Err()
}

func DeleteRoom(ctx context.Context, rdb *redis.Client, roomID string) error {
	return DeleteSession(ctx, rdb, entity.NewRoomRef(roomID))
}

func DeleteStream(ctx context.Context, rdb *redis.Client, streamID string) error {
	return DeleteSession(ctx, rdb, entity.NewStreamRef(streamID))
}

// ---------- High-level helpers ----------

func BuildToDict(ctx context.Context, rdb *redis.Client, rm *entity.Room) map[string]any {
	ref := rm.Ref()
	tracks, _ := GetQueue(ctx, rdb, ref)
	ps, _ := GetPlayback(ctx, rdb, ref)
	votes, _ := GetVotes(ctx, rdb, ref)
	result := buildBaseDict(ctx, rdb, ref, rm.Name, rm.OwnerID, tracks, ps)
	if votes == nil {
		votes = make(map[string]*VoteEntry)
	}
	voteTotals := make(map[string][2]int)
	for trackID, v := range votes {
		voteTotals[trackID] = [2]int{v.LikeCount(), v.DislikeCount()}
	}
	queue := result["queue"].([]map[string]any)
	for _, item := range queue {
		tv := voteTotals[item["id"].(string)]
		item["likes"] = tv[0]
		item["dislikes"] = tv[1]
	}
	var trackVotes [2]int
	if ps != nil && ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		trackVotes = voteTotals[tracks[ps.CurrentIndex].ID]
	}
	if current, ok := result["current_track"].(map[string]any); ok {
		current["likes"] = trackVotes[0]
		current["dislikes"] = trackVotes[1]
	}
	skipVoters, _ := rdb.SMembers(ctx, SessionKey(ref, "skip_votes")).Result()
	result["allow_anonymous_add"] = rm.AllowAnonymousAdd
	result["is_private"] = rm.IsPrivate
	result["auto_radio"] = rm.AutoRadio
	result["has_password"] = rm.PasswordHash != nil
	result["track_votes"] = map[string]int{"likes": trackVotes[0], "dislikes": trackVotes[1]}
	result["skip_voters"] = skipVoters
	return result
}

func BuildStreamToDict(ctx context.Context, rdb *redis.Client, st *entity.Stream) map[string]any {
	ref := st.Ref()
	tracks, _ := GetQueue(ctx, rdb, ref)
	ps, _ := GetPlayback(ctx, rdb, ref)
	result := buildBaseDict(ctx, rdb, ref, st.Name, st.OwnerID, tracks, ps)
	result["allow_anonymous_add"] = false
	result["is_private"] = false
	result["auto_radio"] = false
	result["has_password"] = false
	result["track_votes"] = map[string]int{"likes": 0, "dislikes": 0}
	result["skip_voters"] = []string{}
	result["features"] = st.Features
	result["mashup_radio"] = st.Features.MashupRadio.Enabled
	result["stream_filter"] = st.Features.MashupRadio.Filter
	return result
}

func buildBaseDict(ctx context.Context, rdb *redis.Client, ref entity.SessionRef, name, ownerID string, tracks []*entity.Track, ps *PlaybackState) map[string]any {
	if ps == nil {
		ps = &PlaybackState{}
	}
	if tracks == nil {
		tracks = []*entity.Track{}
	}
	presence, _ := GetPresence(ctx, rdb, ref)
	messages, _ := GetMessages(ctx, rdb, ref)
	if presence == nil {
		presence = make(map[string]*entity.PresenceInfo)
	}
	if messages == nil {
		messages = []*entity.ChatMessage{}
	}

	seen := make(map[string]string)
	for _, info := range presence {
		seen[info.ID] = info.Name
	}
	listeners := make([]map[string]any, 0, len(seen))
	for uid, n := range seen {
		listeners = append(listeners, map[string]any{"id": uid, "name": n})
	}

	queue := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		queue = append(queue, t.ToDict())
	}

	position := ps.Position
	if ps.IsPlaying {
		position += time.Since(ps.LastSyncAt).Seconds()
	}

	result := map[string]any{
		"kind":            ref.Kind,
		"id":              ref.ID,
		"name":            name,
		"owner_id":        ownerID,
		"queue":           queue,
		"current_index":   ps.CurrentIndex,
		"is_playing":      ps.IsPlaying,
		"position":        position,
		"user_count":      len(listeners),
		"listeners":       listeners,
		"radio_searching": ps.RadioFilling,
		"messages":        messageDicts(messages),
	}
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		result["current_track"] = tracks[ps.CurrentIndex].ToDict()
	}
	return result
}

func messageDicts(messages []*entity.ChatMessage) []map[string]any {
	if len(messages) > entity.MaxChatMessages {
		messages = messages[len(messages)-entity.MaxChatMessages:]
	}
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		out = append(out, m.ToDict())
	}
	return out
}

func CurrentTrack(ctx context.Context, rdb *redis.Client, session any) *entity.Track {
	tracks, _ := GetQueue(ctx, rdb, session)
	ps, _ := GetPlayback(ctx, rdb, session)
	if ps == nil || len(tracks) == 0 {
		return nil
	}
	idx := ps.CurrentIndex
	if idx < 0 || idx >= len(tracks) {
		return nil
	}
	return tracks[idx]
}

func GetCurrentPosition(ctx context.Context, rdb *redis.Client, session any) float64 {
	ps, _ := GetPlayback(ctx, rdb, session)
	if ps == nil {
		return 0
	}
	if ps.IsPlaying {
		return ps.Position + time.Since(ps.LastSyncAt).Seconds()
	}
	return ps.Position
}

func GetTrackVotes(ctx context.Context, rdb *redis.Client, session any, trackID string) (likes, dislikes int) {
	votes, _ := GetVotes(ctx, rdb, session)
	if v, ok := votes[trackID]; ok {
		return v.LikeCount(), v.DislikeCount()
	}
	return 0, 0
}

func boolToInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

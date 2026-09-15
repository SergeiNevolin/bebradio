package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/pkg/id"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var reactionEmojis = map[string]bool{
	"❤️": true, "🔥": true, "😂": true, "👍": true,
	"🎉": true, "😮": true, "🙌": true, "💃": true,
}

type Handler struct {
	manager  *ConnectionManager
	room     *usecase.RoomUsecase
	playback *usecase.PlaybackUsecase
	chat     *usecase.ChatUsecase
	radio    *usecase.RadioUsecase
	media    *usecase.MediaUsecase
	config   *config.Config
	rdb      *redis.Client
	log      *slog.Logger
}

func NewHandler(
	manager *ConnectionManager,
	room *usecase.RoomUsecase,
	playback *usecase.PlaybackUsecase,
	chat *usecase.ChatUsecase,
	radio *usecase.RadioUsecase,
	media *usecase.MediaUsecase,
	config *config.Config,
	rdb *redis.Client,
	log *slog.Logger,
) *Handler {
	return &Handler{
		manager:  manager,
		room:     room,
		playback: playback,
		chat:     chat,
		radio:    radio,
		media:    media,
		config:   config,
		rdb:      rdb,
		log:      log,
	}
}

func (h *Handler) HandleWebSocket(conn *websocket.Conn, roomID, access string) {
	roomID = toUpper(roomID)
	ctx := context.Background()

	rm, err := h.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		h.sendError(conn, "Room not found")
		conn.Close()
		return
	}

	if rm.PasswordHash != nil {
		if !h.room.HasRoomAccess(rm, "", access) {
			h.sendError(conn, "Password required")
			conn.Close()
			return
		}
	}

	h.manager.Connect(roomID, conn)
	h.manager.SendJSON(roomID, conn, redisc.BuildToDict(ctx, h.rdb, rm))

	defer func() {
		h.manager.Disconnect(roomID, conn)
		redisc.RemovePresence(ctx, h.rdb, roomID, conn.RemoteAddr().String())
		h.manager.Broadcast(roomID, redisc.BuildToDict(ctx, h.rdb, rm))
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg map[string]any
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		action, _ := msg["action"].(string)

		switch action {
		case "hello":
			h.handleHello(ctx, rm, conn, msg, roomID)
		case "reaction":
			h.handleReaction(ctx, rm, conn, msg, roomID)
		case "next":
			h.handleNext(ctx, rm, roomID)
		case "prev":
			h.handlePrev(ctx, roomID)
		case "jump":
			h.handleJump(ctx, msg, roomID)
		case "seek", "sync":
			h.handleSeek(ctx, msg, roomID)
		case "chat":
			h.handleChat(ctx, rm, conn, msg, roomID)
		case "vote":
			h.handleVote(ctx, rm, conn, msg, roomID)
		case "skip_vote":
			h.handleSkipVote(ctx, rm, conn, roomID)
		}

		if action != "sync" {
			h.manager.Broadcast(roomID, redisc.BuildToDict(ctx, h.rdb, rm))
		}

		if h.radio.NeedsRefill(ctx, roomID, rm.AutoRadio) {
			go h.backgroundRefill(ctx, rm, roomID)
		}
	}
}

func (h *Handler) handleHello(ctx context.Context, rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	userID, _ := msg["user_id"].(string)
	username, _ := msg["username"].(string)
	if username == "" {
		username = "Anonymous"
	}
	if len(username) > 30 {
		username = username[:30]
	}

	id := userID
	if id == "" {
		id = "anon:" + conn.RemoteAddr().String()
	}
	info := &entity.PresenceInfo{ID: id, Name: username}
	redisc.SetPresence(ctx, h.rdb, roomID, conn.RemoteAddr().String(), info)

	h.manager.BindUser(roomID, conn, id)
}

func (h *Handler) handleReaction(ctx context.Context, rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	emoji, _ := msg["emoji"].(string)
	if !reactionEmojis[emoji] {
		return
	}

	username := "Anonymous"
	presence, _ := redisc.GetPresence(ctx, h.rdb, roomID)
	if info, ok := presence[conn.RemoteAddr().String()]; ok && info.Name != "" {
		username = info.Name
	}

	h.manager.Broadcast(roomID, map[string]any{
		"type":     "reaction",
		"id":       id.NewHex(8),
		"emoji":    emoji,
		"username": username,
	})
}

func (h *Handler) handleNext(ctx context.Context, rm *entity.Room, roomID string) {
	changed := h.playback.GoNext(ctx, roomID)
	if changed {
		h.media.EnsureRoomMedia(ctx, roomID)
		if err := h.room.SaveTracks(ctx, rm); err != nil {
			h.log.Error("save tracks after next failed", "room_id", roomID, "error", err)
		}
	}
}

func (h *Handler) handlePrev(ctx context.Context, roomID string) {
	h.playback.GoPrev(ctx, roomID)
}

func (h *Handler) handleJump(ctx context.Context, msg map[string]any, roomID string) {
	if index, ok := msg["index"].(float64); ok {
		h.playback.JumpTo(ctx, roomID, int(index))
	}
}

func (h *Handler) handleSeek(ctx context.Context, msg map[string]any, roomID string) {
	if pos, ok := msg["position"].(float64); ok {
		h.playback.SeekTo(ctx, roomID, pos)
	}
}

func (h *Handler) handleChat(ctx context.Context, rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	text, _ := msg["text"].(string)
	text = trimSpace(text)
	if text == "" {
		return
	}

	userID := h.manager.GetUserID(roomID, conn)
	username := "Anonymous"
	presence, _ := redisc.GetPresence(ctx, h.rdb, roomID)
	if info, ok := presence[conn.RemoteAddr().String()]; ok && info.Name != "" {
		username = info.Name
	}

	chatMsg := h.chat.SendMessage(ctx, roomID, userID, username, text)
	h.manager.Broadcast(roomID, map[string]any{
		"type":    "chat",
		"message": chatMsg.ToDict(),
	})
}

func (h *Handler) handleVote(ctx context.Context, rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	userID := h.manager.GetUserID(roomID, conn)
	trackID, _ := msg["track_id"].(string)
	voteVal, _ := msg["vote"].(float64)

	if userID == "" || trackID == "" {
		return
	}

	// Update vote in Redis.
	votes, _ := redisc.GetVotes(ctx, h.rdb, roomID)
	ve, ok := votes[trackID]
	if !ok {
		ve = &redisc.VoteEntry{}
	}

	// Remove previous vote by this user.
	ve.Disliked = removeStr(ve.Disliked, userID)
	// Note: likes count doesn't track per-user in the current VoteEntry model.
	// For simplicity, we track likes as a count (not per-user) — this is a known limitation.

	if voteVal == -1 {
		ve.Disliked = append(ve.Disliked, userID)
	} else if voteVal == 1 {
		ve.Likes++
	}

	redisc.SetVote(ctx, h.rdb, roomID, trackID, ve)

	// Check auto-skip on dislike.
	if voteVal == -1 {
		likes := ve.Likes
		dislikes := len(ve.Disliked)
		if dislikes > likes {
			h.playback.GoNext(ctx, roomID)
			redisc.ResetSkipVotes(ctx, h.rdb, roomID)
		}
	}
}

func (h *Handler) handleSkipVote(ctx context.Context, rm *entity.Room, conn *websocket.Conn, roomID string) {
	userID := h.manager.GetUserID(roomID, conn)
	if userID == "" {
		return
	}

	skipCount, _ := redisc.ToggleSkipVote(ctx, h.rdb, roomID, userID)

	listeners, _ := redisc.GetPresenceCount(ctx, h.rdb, roomID)
	if listeners < 2 {
		listeners = 2
	}

	if skipCount*2 > listeners {
		h.playback.GoNext(ctx, roomID)
		redisc.ResetSkipVotes(ctx, h.rdb, roomID)
	}
}

func (h *Handler) backgroundRefill(ctx context.Context, rm *entity.Room, roomID string) {
	h.manager.Broadcast(roomID, redisc.BuildToDict(ctx, h.rdb, rm))

	tracks, err := h.radio.Refill(ctx, roomID)
	if err != nil {
		h.log.Error("radio refill failed", "room_id", roomID, "error", err)
		return
	}

	if len(tracks) > 0 {
		for _, t := range tracks {
			redisc.AppendTrack(ctx, h.rdb, roomID, t)
		}
		ps, _ := redisc.GetPlayback(ctx, h.rdb, roomID)
		if ps != nil && !ps.IsPlaying {
			ps.IsPlaying = true
			ps.Position = 0
			ps.LastSyncAt = time.Now()
			redisc.SetPlayback(ctx, h.rdb, roomID, ps)
		}

		if err := h.room.SaveTracks(ctx, rm); err != nil {
			h.log.Error("save tracks after refill failed", "room_id", roomID, "error", err)
		}
	}

	h.manager.Broadcast(roomID, redisc.BuildToDict(ctx, h.rdb, rm))
}

func (h *Handler) sendError(conn *websocket.Conn, message string) {
	if err := conn.WriteJSON(map[string]any{"error": message}); err != nil {
		h.log.Warn("failed to send error to client", "error", err)
	}
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func removeStr(slice []string, val string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			result = append(result, s)
		}
	}
	return result
}

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

func (h *Handler) HandleWebSocket(conn *websocket.Conn, roomID, access, userID string) {
	roomID = toUpper(roomID)
	ctx := context.Background()

	rm, err := h.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		h.sendError(conn, "Room not found")
		conn.Close()
		return
	}

	if rm.PasswordHash != nil {
		if !h.room.HasRoomAccess(rm, userID, access) {
			h.sendError(conn, "Password required")
			conn.Close()
			return
		}
	}

	h.manager.Connect(roomID, conn)
	// Bind the JWT-authenticated id up front: votes, skip votes and chat
	// identity key off the bound id, and the client-supplied "user_id" in
	// hello must never override it (otherwise anyone could vote/skip/chat
	// as anyone else). Anonymous connections bind on hello instead.
	if userID != "" {
		h.manager.BindUser(roomID, conn, userID)
	}
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
			h.handleNext(ctx, rm, conn, roomID)
		case "prev":
			h.handlePrev(ctx, rm, conn, roomID)
		case "jump":
			h.handleJump(ctx, rm, conn, msg, roomID)
		case "seek":
			h.handleSeek(ctx, rm, conn, msg, roomID)
		case "sync":
			// Intentionally ignored: the server wall-clock is authoritative
			// for playback position, so client reports must never move it.
			// Blindly adopting them let any laggy client drag the whole
			// room backward (the classic "rewinds on bad network").
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
		if rm.IsStream && h.radio.NeedsMashupRefill(ctx, roomID, rm) {
			go h.backgroundMashupRefill(ctx, rm, roomID)
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

	// A connection bound at upgrade time (JWT) keeps its id: the client
	// must not be able to re-bind to someone else's id via hello.
	id := resolvePresenceID(h.manager.GetUserID(roomID, conn), userID, conn.RemoteAddr().String())
	if h.manager.GetUserID(roomID, conn) == "" {
		h.manager.BindUser(roomID, conn, id)
	}
	info := &entity.PresenceInfo{ID: id, Name: username}
	redisc.SetPresence(ctx, h.rdb, roomID, conn.RemoteAddr().String(), info)
}

// resolvePresenceID picks the stable identity for a connection: the
// JWT-bound id wins, then the client-supplied one (guests), then an
// anonymous id derived from the connection. Pure function, unit-tested.
func resolvePresenceID(boundID, msgUserID, remoteAddr string) string {
	if boundID != "" {
		return boundID
	}
	if msgUserID != "" {
		return msgUserID
	}
	return "anon:" + remoteAddr
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

func (h *Handler) handleNext(ctx context.Context, rm *entity.Room, conn *websocket.Conn, roomID string) {
	// Streams are admin-driven: listeners must not skip tracks manually.
	// Auto-advance (client onEnded at track end + server tick) still works:
	// the tick bypasses this gate.
	if rm.IsStream && !h.room.IsController(rm, h.manager.GetUserID(roomID, conn)) {
		return
	}
	changed := h.playback.GoNext(ctx, roomID)
	if changed {
		h.media.EnsureRoomMedia(ctx, roomID)
		if err := h.room.SaveTracks(ctx, rm); err != nil {
			h.log.Error("save tracks after next failed", "room_id", roomID, "error", err)
		}
	}
}

func (h *Handler) handlePrev(ctx context.Context, rm *entity.Room, conn *websocket.Conn, roomID string) {
	if rm.IsStream && !h.room.IsController(rm, h.manager.GetUserID(roomID, conn)) {
		return
	}
	h.playback.GoPrev(ctx, roomID)
}

func (h *Handler) handleJump(ctx context.Context, rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	if rm.IsStream && !h.room.IsController(rm, h.manager.GetUserID(roomID, conn)) {
		return
	}
	if index, ok := msg["index"].(float64); ok {
		h.playback.JumpTo(ctx, roomID, int(index))
	}
}

func (h *Handler) handleSeek(ctx context.Context, rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	// Explicit repositioning moves playback for everyone at once, so it is
	// controller-only in every room kind (unlike passive sync reports,
	// which are ignored entirely).
	if !h.room.IsController(rm, h.manager.GetUserID(roomID, conn)) {
		return
	}
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
	// Streams have no voting: listeners only listen.
	if rm.IsStream {
		return
	}
	userID := h.manager.GetUserID(roomID, conn)
	trackID, _ := msg["track_id"].(string)
	voteVal, _ := msg["vote"].(float64)

	if h.playback.RegisterVote(ctx, roomID, userID, trackID, int(voteVal)) {
		if err := h.room.SaveTracks(ctx, rm); err != nil {
			h.log.Error("save tracks after auto-skip failed", "room_id", roomID, "error", err)
		}
	}
}

func (h *Handler) handleSkipVote(ctx context.Context, rm *entity.Room, conn *websocket.Conn, roomID string) {
	// Streams have no voting: listeners only listen.
	if rm.IsStream {
		return
	}
	userID := h.manager.GetUserID(roomID, conn)
	if userID == "" {
		return
	}

	skipCount, _ := redisc.ToggleSkipVote(ctx, h.rdb, roomID, userID)
	listeners, _ := redisc.GetPresenceCount(ctx, h.rdb, roomID)

	if skipThresholdMet(skipCount, listeners) {
		if h.playback.GoNext(ctx, roomID) {
			if err := h.room.SaveTracks(ctx, rm); err != nil {
				h.log.Error("save tracks after skip-vote failed", "room_id", roomID, "error", err)
			}
		}
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

	// AppendFreshTrack re-validates each pick against the live queue: picks
	// went stale if a skip landed while Refill was doing network I/O.
	appended := 0
	for _, t := range tracks {
		if ok, _ := redisc.AppendFreshTrack(ctx, h.rdb, roomID, t); ok {
			appended++
		}
	}

	if appended > 0 {
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

func (h *Handler) backgroundMashupRefill(ctx context.Context, rm *entity.Room, roomID string) {
	h.manager.Broadcast(roomID, redisc.BuildToDict(ctx, h.rdb, rm))

	picks, err := h.radio.RefillMashups(ctx, roomID, rm)
	if err != nil {
		h.log.Error("mashup refill failed", "room_id", roomID, "error", err)
		return
	}
	appended := 0
	for _, t := range picks {
		if ok, _ := redisc.AppendFreshTrack(ctx, h.rdb, roomID, t); ok {
			appended++
			if t.ID != "" {
				redisc.AddRadioSeen(ctx, h.rdb, roomID, t.ID)
			}
		}
	}
	if appended > 0 {
		ps, _ := redisc.GetPlayback(ctx, h.rdb, roomID)
		if ps != nil && !ps.IsPlaying {
			ps.IsPlaying = true
			ps.Position = 0
			ps.LastSyncAt = time.Now()
			ps.CurrentStartedAt = time.Now()
			redisc.SetPlayback(ctx, h.rdb, roomID, ps)
		}
		if err := h.room.SaveTracks(ctx, rm); err != nil {
			h.log.Error("save tracks after mashup refill failed", "room_id", roomID, "error", err)
		}
	}
	h.manager.Broadcast(roomID, redisc.BuildToDict(ctx, h.rdb, rm))
}

func (h *Handler) sendError(conn *websocket.Conn, message string) {
	if err := conn.WriteJSON(map[string]any{"error": message}); err != nil {
		h.log.Warn("failed to send error to client", "error", err)
	}
}

// skipThresholdMet reports strict-majority consensus to skip the current
// track. No listener floor: a solo listener's vote (1*2 > 1) skips.
func skipThresholdMet(skipCount, listeners int64) bool {
	return skipCount*2 > listeners
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

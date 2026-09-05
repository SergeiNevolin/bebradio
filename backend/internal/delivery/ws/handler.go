package ws

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/pkg/id"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/gorilla/websocket"
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
		log:      log,
	}
}

func (h *Handler) HandleWebSocket(conn *websocket.Conn, roomID, access string) {
	roomID = toUpper(roomID)

	rm, err := h.room.GetOrLoadRoom(roomID)
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
	h.manager.SendJSON(roomID, conn, rm.ToDict())

	defer func() {
		h.manager.Disconnect(roomID, conn)
		rm.Mu.Lock()
		// Clean up presence for this connection
		addr := conn.RemoteAddr().String()
		delete(rm.Presence, addr)
		rm.Mu.Unlock()
		h.manager.Broadcast(roomID, rm.ToDict())
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
		userID, _ := msg["user_id"].(string)

		rm.Mu.Lock()
		if userID != "" {
			rm.Users[conn.RemoteAddr().String()] = userID
		}
		rm.Mu.Unlock()

		switch action {
		case "hello":
			h.handleHello(rm, conn, msg, roomID)
		case "reaction":
			h.handleReaction(rm, msg, roomID)
		case "next":
			h.handleNext(rm, roomID)
		case "prev":
			h.handlePrev(rm, roomID)
		case "jump":
			h.handleJump(rm, msg, roomID)
		case "seek", "sync":
			h.handleSeek(rm, msg)
		case "chat":
			h.handleChat(rm, msg, roomID)
		case "vote":
			h.handleVote(rm, msg, roomID)
		case "skip_vote":
			h.handleSkipVote(rm, msg, roomID)
		case "clear_skip_votes":
			rm.Mu.Lock()
			rm.SkipVotes = make(map[string]bool)
			rm.Mu.Unlock()
		}

		h.manager.Broadcast(roomID, rm.ToDict())

		if h.radio.NeedsRefill(rm) {
			go h.backgroundRefill(rm, roomID)
		}
	}
}

func (h *Handler) handleHello(rm *entity.Room, conn *websocket.Conn, msg map[string]any, roomID string) {
	userID, _ := msg["user_id"].(string)
	username, _ := msg["username"].(string)
	if username == "" {
		username = "Anonymous"
	}
	if len(username) > 30 {
		username = username[:30]
	}

	rm.Mu.Lock()
	id := userID
	if id == "" {
		id = "anon:" + conn.RemoteAddr().String()
	}
	rm.Presence[conn.RemoteAddr().String()] = entity.PresenceInfo{
		ID:   id,
		Name: username,
	}
	rm.Mu.Unlock()
}

func (h *Handler) handleReaction(rm *entity.Room, msg map[string]any, roomID string) {
	emoji, _ := msg["emoji"].(string)
	username, _ := msg["username"].(string)
	if !reactionEmojis[emoji] {
		return
	}
	if username == "" {
		username = "Anonymous"
	}

	h.manager.Broadcast(roomID, map[string]any{
		"type":     "reaction",
		"id":       id.NewHex(8),
		"emoji":    emoji,
		"username": username,
	})
}

func (h *Handler) handleNext(rm *entity.Room, roomID string) {
	changed := h.playback.GoNext(rm)
	if changed {
		h.media.EnsureRoomMedia(rm)
		if err := h.room.SaveTracks(rm); err != nil {
			h.log.Error("save tracks after next failed", "room_id", roomID, "error", err)
		}
	}
}

func (h *Handler) handlePrev(rm *entity.Room, roomID string) {
	h.playback.GoPrev(rm)
}

func (h *Handler) handleJump(rm *entity.Room, msg map[string]any, roomID string) {
	if index, ok := msg["index"].(float64); ok {
		h.playback.JumpTo(rm, int(index))
	}
}

func (h *Handler) handleSeek(rm *entity.Room, msg map[string]any) {
	if pos, ok := msg["position"].(float64); ok {
		h.playback.SeekTo(rm, pos)
	}
}

func (h *Handler) handleChat(rm *entity.Room, msg map[string]any, roomID string) {
	text, _ := msg["text"].(string)
	userID, _ := msg["user_id"].(string)
	username, _ := msg["username"].(string)

	text = trimSpace(text)
	if text == "" {
		return
	}
	if username == "" {
		username = "Anonymous"
	}

	chatMsg := h.chat.SendMessage(rm, userID, username, text)
	h.manager.Broadcast(roomID, map[string]any{
		"type":    "chat",
		"message": chatMsg.ToDict(),
	})
}

func (h *Handler) handleVote(rm *entity.Room, msg map[string]any, roomID string) {
	userID, _ := msg["user_id"].(string)
	trackID, _ := msg["track_id"].(string)
	voteVal, _ := msg["vote"].(float64)

	if userID == "" || trackID == "" {
		return
	}

	rm.Mu.Lock()
	newVotes := make([]*entity.TrackVote, 0)
	for _, v := range rm.Votes {
		if !(v.UserID == userID && v.TrackID == trackID) {
			newVotes = append(newVotes, v)
		}
	}
	rm.Votes = newVotes

	if voteVal == 1 || voteVal == -1 {
		rm.Votes = append(rm.Votes, &entity.TrackVote{
			UserID:  userID,
			TrackID: trackID,
			Vote:    int(voteVal),
		})
	}
	rm.Mu.Unlock()

	if err := h.room.SaveVotes(rm); err != nil {
		h.log.Error("save votes after vote failed", "room_id", roomID, "error", err)
	}
	if err := h.room.SaveTracks(rm); err != nil {
		h.log.Error("save tracks after vote failed", "room_id", roomID, "error", err)
	}

	currentTrack := rm.CurrentTrack()
	if currentTrack != nil && currentTrack.ID == trackID {
		likes, dislikes := rm.GetTrackVotes(trackID)
		if dislikes > likes {
			h.playback.GoNext(rm)
			rm.Mu.Lock()
			rm.SkipVotes = make(map[string]bool)
			rm.Mu.Unlock()
		}
	}
}

func (h *Handler) handleSkipVote(rm *entity.Room, msg map[string]any, roomID string) {
	userID, _ := msg["user_id"].(string)
	if userID == "" {
		return
	}

	rm.Mu.Lock()
	if rm.SkipVotes[userID] {
		delete(rm.SkipVotes, userID)
	} else {
		rm.SkipVotes[userID] = true
	}

	listeners := h.manager.GetCount(roomID)
	if listeners < 2 {
		listeners = 2
	}
	skipCount := len(rm.SkipVotes)
	rm.Mu.Unlock()

	if skipCount >= listeners/2 {
		h.playback.GoNext(rm)
		rm.Mu.Lock()
		rm.SkipVotes = make(map[string]bool)
		rm.Mu.Unlock()
	}
}

func (h *Handler) backgroundRefill(rm *entity.Room, roomID string) {
	h.manager.Broadcast(roomID, rm.ToDict())

	tracks, err := h.radio.Refill(rm)
	if err != nil {
		h.log.Error("radio refill failed", "room_id", roomID, "error", err)
		return
	}

	if len(tracks) > 0 {
		rm.Mu.Lock()
		rm.Queue = append(rm.Queue, tracks...)
		if !rm.IsPlaying {
			rm.IsPlaying = true
			rm.Position = 0
			rm.LastSyncAt = time.Now()
		}
		rm.Mu.Unlock()

		if err := h.room.SaveTracks(rm); err != nil {
			h.log.Error("save tracks after refill failed", "room_id", roomID, "error", err)
		}
	}

	h.manager.Broadcast(roomID, rm.ToDict())
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

package repository

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

var ErrNotFound = errors.New("not found")

type MockUserRepo struct {
	Users    map[string]*entity.User
	CreateFn func(user *entity.User) error
}

func NewMockUserRepo() *MockUserRepo {
	return &MockUserRepo{
		Users: make(map[string]*entity.User),
	}
}

func (m *MockUserRepo) Create(user *entity.User) error {
	if m.CreateFn != nil {
		return m.CreateFn(user)
	}
	m.Users[user.ID] = user
	return nil
}

func (m *MockUserRepo) FindByID(id string) (*entity.User, error) {
	if u, ok := m.Users[id]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (m *MockUserRepo) FindByEmail(email string) (*entity.User, error) {
	for _, u := range m.Users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockUserRepo) FindByUsername(username string) (*entity.User, error) {
	for _, u := range m.Users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockUserRepo) UpdateProfile(id string, bio, avatarURL *string) (*entity.User, error) {
	u, ok := m.Users[id]
	if !ok {
		return nil, ErrNotFound
	}
	if bio != nil {
		u.Bio = *bio
	}
	if avatarURL != nil {
		u.AvatarURL = *avatarURL
	}
	return u, nil
}

func (m *MockUserRepo) SetRole(id string, role string) error {
	u, ok := m.Users[id]
	if !ok {
		return ErrNotFound
	}
	u.Role = role
	return nil
}

func (m *MockUserRepo) SearchByUsername(prefix string, limit int) ([]*entity.User, error) {
	var result []*entity.User
	for _, u := range m.Users {
		if len(result) >= limit {
			break
		}
		if len(u.Username) >= len(prefix) && u.Username[:len(prefix)] == prefix {
			result = append(result, u)
		}
	}
	return result, nil
}

func (m *MockUserRepo) HasAdmin() (bool, error) {
	for _, u := range m.Users {
		if u.Role == "admin" {
			return true, nil
		}
	}
	return false, nil
}

type MockRoomRepo struct {
	Rooms    map[string]*entity.Room
	Messages map[string][]*entity.ChatMessage
	Tracks   map[string][]*entity.Track
}

func NewMockRoomRepo() *MockRoomRepo {
	return &MockRoomRepo{
		Rooms:    make(map[string]*entity.Room),
		Messages: make(map[string][]*entity.ChatMessage),
		Tracks:   make(map[string][]*entity.Track),
	}
}

func (m *MockRoomRepo) Save(room *entity.Room) error {
	m.Rooms[room.ID] = room
	return nil
}

func (m *MockRoomRepo) FindByID(id string) (*entity.Room, error) {
	if r, ok := m.Rooms[id]; ok {
		return r, nil
	}
	return nil, ErrNotFound
}

func (m *MockRoomRepo) Delete(id string) error {
	delete(m.Rooms, id)
	delete(m.Messages, id)
	delete(m.Tracks, id)
	return nil
}

func (m *MockRoomRepo) ListPublic() ([]map[string]any, error) {
	var result []map[string]any
	for _, r := range m.Rooms {
		if !r.IsPrivate {
			result = append(result, map[string]any{
				"id":         r.ID,
				"name":       r.Name,
				"auto_radio": r.AutoRadio,
			})
		}
	}
	return result, nil
}

func (m *MockRoomRepo) SaveTracks(room *entity.Room) error {
	return nil
}

func (m *MockRoomRepo) LoadTracks(roomID string) ([]*entity.Track, error) {
	return m.Tracks[roomID], nil
}

func (m *MockRoomRepo) SaveMessage(roomID string, msg *entity.ChatMessage) error {
	m.Messages[roomID] = append(m.Messages[roomID], msg)
	return nil
}

func (m *MockRoomRepo) LoadMessages(roomID string) ([]*entity.ChatMessage, error) {
	return m.Messages[roomID], nil
}

func (m *MockRoomRepo) SaveTracksFromSlice(roomID string, tracks []*entity.Track) error {
	m.Tracks[roomID] = tracks
	return nil
}

func (m *MockRoomRepo) RecordVisit(userID, roomID string) error {
	return nil
}

func (m *MockRoomRepo) RecentRooms(userID string, limit int) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

type MockMediaClient struct {
	SearchFn            func(query string, limit int) ([]map[string]any, error)
	ResolveFn           func(url string) (map[string]any, error)
	EnsureFn            func(items []map[string]any) ([]string, error)
	RelatedFn           func(sourceURL string, limit int) ([]string, error)
	MediaCaptionsFn     func(mediaID, lang string) (map[string]any, error)
	ContentFn           func(mediaID, rangeHeader string) (int64, string, []byte, error)
	DownloadFn          func(sourceURL, mediaID string) (map[string]any, error)
	UpdateRefsFn        func(mediaIDs []string) error
	UploadTrackFn       func(filename string, body io.Reader) (map[string]any, error)
	UploadTrackCoverFn  func(mediaID, filename string, body io.Reader) error
	TrackUploadStatusFn func(mediaID string) (map[string]any, error)
	DeleteTrackFn       func(mediaID string) error
}

func NewMockMediaClient() *MockMediaClient {
	return &MockMediaClient{}
}

var mockUploadCounter int64

func (m *MockMediaClient) UploadTrack(filename string, body io.Reader) (map[string]any, error) {
	if m.UploadTrackFn != nil {
		return m.UploadTrackFn(filename, body)
	}
	n := atomic.AddInt64(&mockUploadCounter, 1)
	return map[string]any{
		"id":       fmt.Sprintf("mockid%02d", n),
		"media_id": fmt.Sprintf("mockmedia%02d", n),
		"status":   "processing",
	}, nil
}

func (m *MockMediaClient) Search(query string, limit int) ([]map[string]any, error) {
	if m.SearchFn != nil {
		return m.SearchFn(query, limit)
	}
	return nil, nil
}

func (m *MockMediaClient) Resolve(url string) (map[string]any, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(url)
	}
	return nil, nil
}

func (m *MockMediaClient) Ensure(items []map[string]any) ([]string, error) {
	if m.EnsureFn != nil {
		return m.EnsureFn(items)
	}
	return nil, nil
}

func (m *MockMediaClient) Related(sourceURL string, limit int) ([]string, error) {
	if m.RelatedFn != nil {
		return m.RelatedFn(sourceURL, limit)
	}
	return nil, nil
}

func (m *MockMediaClient) MediaCaptions(mediaID, lang string) (map[string]any, error) {
	if m.MediaCaptionsFn != nil {
		return m.MediaCaptionsFn(mediaID, lang)
	}
	return map[string]any{"lang": "", "auto": false, "cues": []any{}}, nil
}

func (m *MockMediaClient) Content(mediaID, rangeHeader string) (int64, string, []byte, error) {
	if m.ContentFn != nil {
		return m.ContentFn(mediaID, rangeHeader)
	}
	return 200, "audio/mpeg", nil, nil
}

func (m *MockMediaClient) Download(sourceURL, mediaID string) (map[string]any, error) {
	if m.DownloadFn != nil {
		return m.DownloadFn(sourceURL, mediaID)
	}
	return nil, nil
}

func (m *MockMediaClient) UpdateReferences(mediaIDs []string) error {
	if m.UpdateRefsFn != nil {
		return m.UpdateRefsFn(mediaIDs)
	}
	return nil
}

func (m *MockMediaClient) UploadTrackCover(mediaID, filename string, body io.Reader) error {
	if m.UploadTrackCoverFn != nil {
		return m.UploadTrackCoverFn(mediaID, filename, body)
	}
	return nil
}

func (m *MockMediaClient) TrackUploadStatus(mediaID string) (map[string]any, error) {
	if m.TrackUploadStatusFn != nil {
		return m.TrackUploadStatusFn(mediaID)
	}
	return map[string]any{"status": "ready"}, nil
}

func (m *MockMediaClient) DeleteTrack(mediaID string) error {
	if m.DeleteTrackFn != nil {
		return m.DeleteTrackFn(mediaID)
	}
	return nil
}

type MockTrackRepo struct {
	Items     map[string]*entity.Track
	Likes     map[string]map[string]bool // trackID -> set of userID
	CoverSet  map[string]bool
	CreateErr error
}

func NewMockTrackRepo() *MockTrackRepo {
	return &MockTrackRepo{
		Items:    make(map[string]*entity.Track),
		Likes:    make(map[string]map[string]bool),
		CoverSet: make(map[string]bool),
	}
}

func (m *MockTrackRepo) liked(trackID, viewerID string) bool {
	return viewerID != "" && m.Likes[trackID][viewerID]
}

func (m *MockTrackRepo) Create(track *entity.Track) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	cp := *track
	m.Items[track.ID] = &cp
	return nil
}

func (m *MockTrackRepo) FindByID(id string) (*entity.Track, error) {
	if v, ok := m.Items[id]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}

func (m *MockTrackRepo) Delete(id string) error {
	delete(m.Items, id)
	return nil
}

func (m *MockTrackRepo) UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error {
	if v, ok := m.Items[id]; ok {
		v.Status = status
		v.Error = errMsg
		v.Duration = duration
		v.SizeBytes = sizeBytes
		v.HasCover = hasCover
	}
	return nil
}

func (m *MockTrackRepo) Detail(id, viewerID string) (*entity.Track, error) {
	v, ok := m.Items[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *v
	cp.Liked = m.liked(id, viewerID)
	return &cp, nil
}

func (m *MockTrackRepo) SetCoverUploaded(id string) error {
	m.CoverSet[id] = true
	if v, ok := m.Items[id]; ok {
		v.HasCover = true
	}
	return nil
}

func (m *MockTrackRepo) List(query, sort string, limit, offset int, viewerID string) ([]*entity.Track, error) {
	out := make([]*entity.Track, 0)
	for id, v := range m.Items {
		cp := *v
		cp.Liked = m.liked(id, viewerID)
		out = append(out, &cp)
	}
	return out, nil
}

func (m *MockTrackRepo) ListByOwner(ownerID string) ([]*entity.Track, error) {
	out := make([]*entity.Track, 0)
	for id, v := range m.Items {
		if v.OwnerID == ownerID {
			cp := *v
			cp.Liked = m.liked(id, ownerID)
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *MockTrackRepo) ListLikedByUser(userID string, limit, offset int) ([]*entity.Track, error) {
	out := make([]*entity.Track, 0)
	for id, v := range m.Items {
		if m.Likes[id][userID] {
			cp := *v
			cp.Liked = true
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *MockTrackRepo) Like(trackID, userID string) (int, error) {
	if _, ok := m.Items[trackID]; !ok {
		return 0, ErrNotFound
	}
	if m.Likes[trackID] == nil {
		m.Likes[trackID] = make(map[string]bool)
	}
	m.Likes[trackID][userID] = true
	return m.recount(trackID), nil
}

func (m *MockTrackRepo) Unlike(trackID, userID string) (int, error) {
	if _, ok := m.Items[trackID]; !ok {
		return 0, ErrNotFound
	}
	delete(m.Likes[trackID], userID)
	return m.recount(trackID), nil
}

func (m *MockTrackRepo) recount(trackID string) int {
	n := len(m.Likes[trackID])
	if v, ok := m.Items[trackID]; ok {
		v.Likes = n
	}
	return n
}

func (m *MockTrackRepo) CountByOwner(ownerID string) (int, error) {
	n := 0
	for _, v := range m.Items {
		if v.OwnerID == ownerID {
			n++
		}
	}
	return n, nil
}

func (m *MockTrackRepo) ListProcessing() ([]*entity.Track, error) {
	out := make([]*entity.Track, 0)
	for _, v := range m.Items {
		if v.Status == "processing" {
			out = append(out, v)
		}
	}
	return out, nil
}

type MockAuthBridge struct {
	HashPasswordFn      func(password string) (string, error)
	VerifyPasswordFn    func(password, hash string) bool
	CreateTokenFn       func(userID string) (string, error)
	DecodeTokenFn       func(token string) (string, error)
	CreateRoomTokenFn   func(roomID string) (string, error)
	VerifyRoomTokenFn   func(token string, roomID string) bool
}

func NewMockAuthBridge() *MockAuthBridge {
	return &MockAuthBridge{
		HashPasswordFn: func(p string) (string, error) { return "hashed_" + p, nil },
		VerifyPasswordFn: func(p, h string) bool { return h == "hashed_"+p },
		CreateTokenFn: func(uid string) (string, error) { return "token_" + uid, nil },
		DecodeTokenFn: func(t string) (string, error) { return t, nil },
		CreateRoomTokenFn: func(rid string) (string, error) { return "room_token_" + rid, nil },
		VerifyRoomTokenFn: func(t, rid string) bool { return t == "room_token_"+rid },
	}
}

func (m *MockAuthBridge) HashPassword(password string) (string, error) {
	if m.HashPasswordFn != nil {
		return m.HashPasswordFn(password)
	}
	return "", nil
}

func (m *MockAuthBridge) VerifyPassword(password, hash string) bool {
	if m.VerifyPasswordFn != nil {
		return m.VerifyPasswordFn(password, hash)
	}
	return false
}

func (m *MockAuthBridge) CreateToken(userID string) (string, error) {
	if m.CreateTokenFn != nil {
		return m.CreateTokenFn(userID)
	}
	return "", nil
}

func (m *MockAuthBridge) DecodeToken(token string) (string, error) {
	if m.DecodeTokenFn != nil {
		return m.DecodeTokenFn(token)
	}
	return "", nil
}

func (m *MockAuthBridge) CreateRoomToken(roomID string) (string, error) {
	if m.CreateRoomTokenFn != nil {
		return m.CreateRoomTokenFn(roomID)
	}
	return "", nil
}

func (m *MockAuthBridge) VerifyRoomToken(token string, roomID string) bool {
	if m.VerifyRoomTokenFn != nil {
		return m.VerifyRoomTokenFn(token, roomID)
	}
	return false
}


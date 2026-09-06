package repository

import (
	"errors"
	"io"

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

type MockRoomRepo struct {
	Rooms     map[string]*entity.Room
	Messages  map[string][]*entity.ChatMessage
	Votes     map[string][]*entity.TrackVote
	Tracks    map[string][]*entity.Track
}

func NewMockRoomRepo() *MockRoomRepo {
	return &MockRoomRepo{
		Rooms:    make(map[string]*entity.Room),
		Messages: make(map[string][]*entity.ChatMessage),
		Votes:    make(map[string][]*entity.TrackVote),
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
	delete(m.Votes, id)
	delete(m.Tracks, id)
	return nil
}

func (m *MockRoomRepo) ListPublic() ([]map[string]any, error) {
	var result []map[string]any
	for _, r := range m.Rooms {
		if !r.IsPrivate {
			result = append(result, map[string]any{
				"id":   r.ID,
				"name": r.Name,
			})
		}
	}
	return result, nil
}

func (m *MockRoomRepo) SaveTracks(room *entity.Room) error {
	m.Tracks[room.ID] = room.Queue
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

func (m *MockRoomRepo) SaveVotes(room *entity.Room) error {
	m.Votes[room.ID] = room.Votes
	return nil
}

func (m *MockRoomRepo) LoadVotes(roomID string) ([]*entity.TrackVote, error) {
	return m.Votes[roomID], nil
}

func (m *MockRoomRepo) RecordVisit(userID, roomID string) error {
	return nil
}

func (m *MockRoomRepo) RecentRooms(userID string, limit int) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

type MockMediaClient struct {
	SearchFn    func(query string, limit int) ([]map[string]any, error)
	ResolveFn   func(url string) (map[string]any, error)
	EnsureFn    func(items []map[string]any) ([]string, error)
	RelatedFn   func(sourceURL string, limit int) ([]string, error)
	CaptionsFn  func(sourceURL, lang string) (map[string]any, error)
	ContentFn   func(mediaID, rangeHeader string) (int64, string, []byte, error)
	DownloadFn  func(sourceURL, mediaID string) (map[string]any, error)
	UpdateRefsFn func(mediaIDs []string) error
	UploadMashupFn func(mediaID, filename string, body io.Reader) error
	UploadMashupCoverFn func(mediaID, filename string, body io.Reader) error
	MashupStatusFn func(mediaID string) (map[string]any, error)
	DeleteMashupFn func(mediaID string) error
}

func NewMockMediaClient() *MockMediaClient {
	return &MockMediaClient{}
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

func (m *MockMediaClient) Captions(sourceURL, lang string) (map[string]any, error) {
	if m.CaptionsFn != nil {
		return m.CaptionsFn(sourceURL, lang)
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

func (m *MockMediaClient) UploadMashup(mediaID, filename string, body io.Reader) error {
	if m.UploadMashupFn != nil {
		return m.UploadMashupFn(mediaID, filename, body)
	}
	return nil
}

func (m *MockMediaClient) UploadMashupCover(mediaID, filename string, body io.Reader) error {
	if m.UploadMashupCoverFn != nil {
		return m.UploadMashupCoverFn(mediaID, filename, body)
	}
	return nil
}

func (m *MockMediaClient) MashupStatus(mediaID string) (map[string]any, error) {
	if m.MashupStatusFn != nil {
		return m.MashupStatusFn(mediaID)
	}
	return map[string]any{"status": "ready"}, nil
}

func (m *MockMediaClient) DeleteMashup(mediaID string) error {
	if m.DeleteMashupFn != nil {
		return m.DeleteMashupFn(mediaID)
	}
	return nil
}

type MockMashupRepo struct {
	Items     map[string]*entity.Mashup
	Likes     map[string]map[string]bool // mashupID -> set of userID
	CoverSet  map[string]bool
	CreateErr error
}

func NewMockMashupRepo() *MockMashupRepo {
	return &MockMashupRepo{
		Items:    make(map[string]*entity.Mashup),
		Likes:    make(map[string]map[string]bool),
		CoverSet: make(map[string]bool),
	}
}

func (m *MockMashupRepo) liked(mashupID, viewerID string) bool {
	return viewerID != "" && m.Likes[mashupID][viewerID]
}

func (m *MockMashupRepo) Create(mashup *entity.Mashup) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	cp := *mashup
	m.Items[mashup.ID] = &cp
	return nil
}

func (m *MockMashupRepo) FindByID(id string) (*entity.Mashup, error) {
	if v, ok := m.Items[id]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}

func (m *MockMashupRepo) Delete(id string) error {
	delete(m.Items, id)
	return nil
}

func (m *MockMashupRepo) UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error {
	if v, ok := m.Items[id]; ok {
		v.Status = status
		v.Error = errMsg
		v.Duration = duration
		v.SizeBytes = sizeBytes
		v.HasCover = hasCover
	}
	return nil
}

func (m *MockMashupRepo) Detail(id, viewerID string) (*entity.Mashup, error) {
	v, ok := m.Items[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *v
	cp.Liked = m.liked(id, viewerID)
	return &cp, nil
}

func (m *MockMashupRepo) SetCoverUploaded(id string) error {
	m.CoverSet[id] = true
	if v, ok := m.Items[id]; ok {
		v.HasCover = true
	}
	return nil
}

func (m *MockMashupRepo) List(query, sort string, limit, offset int, viewerID string) ([]*entity.Mashup, error) {
	out := make([]*entity.Mashup, 0)
	for id, v := range m.Items {
		cp := *v
		cp.Liked = m.liked(id, viewerID)
		out = append(out, &cp)
	}
	return out, nil
}

func (m *MockMashupRepo) ListByOwner(ownerID string) ([]*entity.Mashup, error) {
	out := make([]*entity.Mashup, 0)
	for id, v := range m.Items {
		if v.OwnerID == ownerID {
			cp := *v
			cp.Liked = m.liked(id, ownerID)
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *MockMashupRepo) ListLikedByUser(userID string, limit, offset int) ([]*entity.Mashup, error) {
	out := make([]*entity.Mashup, 0)
	for id, v := range m.Items {
		if m.Likes[id][userID] {
			cp := *v
			cp.Liked = true
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *MockMashupRepo) Like(mashupID, userID string) (int, error) {
	if _, ok := m.Items[mashupID]; !ok {
		return 0, ErrNotFound
	}
	if m.Likes[mashupID] == nil {
		m.Likes[mashupID] = make(map[string]bool)
	}
	m.Likes[mashupID][userID] = true
	return m.recount(mashupID), nil
}

func (m *MockMashupRepo) Unlike(mashupID, userID string) (int, error) {
	if _, ok := m.Items[mashupID]; !ok {
		return 0, ErrNotFound
	}
	delete(m.Likes[mashupID], userID)
	return m.recount(mashupID), nil
}

func (m *MockMashupRepo) recount(mashupID string) int {
	n := len(m.Likes[mashupID])
	if v, ok := m.Items[mashupID]; ok {
		v.Likes = n
	}
	return n
}

func (m *MockMashupRepo) CountByOwner(ownerID string) (int, error) {
	n := 0
	for _, v := range m.Items {
		if v.OwnerID == ownerID {
			n++
		}
	}
	return n, nil
}

func (m *MockMashupRepo) ListProcessing() ([]*entity.Mashup, error) {
	out := make([]*entity.Mashup, 0)
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

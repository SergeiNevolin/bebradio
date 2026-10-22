package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/pkg/id"
	"github.com/redis/go-redis/v9"
)

var ErrStreamAdminOnly = &BusinessError{Code: 403, Message: "Only admins can manage streams"}

type StreamUsecase struct {
	streamRepo  repository.StreamRepository
	sessionRepo repository.SessionRepository
	userRepo    repository.UserRepository
	log         *slog.Logger
	rdb         *redis.Client
}

func NewStreamUsecase(streamRepo repository.StreamRepository, sessionRepo repository.SessionRepository, userRepo repository.UserRepository, log *slog.Logger, rdb *redis.Client) *StreamUsecase {
	return &StreamUsecase{
		streamRepo:  streamRepo,
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
		log:         log,
		rdb:         rdb,
	}
}

func (uc *StreamUsecase) CreateStream(ctx context.Context, name string, user *entity.User) (*entity.Stream, error) {
	if user == nil || !user.IsAdmin() {
		return nil, ErrStreamAdminOnly
	}
	st := entity.NewStream(id.New(6), name, user.ID)
	if err := uc.streamRepo.Save(st); err != nil {
		return nil, err
	}
	redisc.SetMashupRadioFeature(ctx, uc.rdb, st.Ref(), st.Features.MashupRadio)
	uc.ensurePlayback(ctx, st.Ref())
	return st, nil
}

func (uc *StreamUsecase) GetOrLoadStream(ctx context.Context, streamID string) (*entity.Stream, error) {
	streamID = toUpper(streamID)
	st, err := uc.streamRepo.FindByID(streamID)
	if err != nil {
		return nil, err
	}
	ref := st.Ref()
	uc.ensurePlayback(ctx, ref)

	featureKey := redisc.SessionKey(ref, redisc.FeatureMashupRadio)
	if n, _ := uc.rdb.Exists(ctx, featureKey).Result(); n == 0 {
		redisc.SetMashupRadioFeature(ctx, uc.rdb, ref, st.Features.MashupRadio)
	}

	acquired, _ := uc.rdb.SetNX(ctx, redisc.SessionKey(ref, "hydrated"), "1", 0).Result()
	if acquired && uc.sessionRepo != nil {
		tracks, _ := uc.sessionRepo.LoadTracks(ref)
		if len(tracks) > 0 {
			redisc.SetQueue(ctx, uc.rdb, ref, tracks)
		}
		messages, _ := uc.sessionRepo.LoadMessages(ref)
		for _, m := range messages {
			redisc.AppendMessage(ctx, uc.rdb, ref, m)
		}
	}
	return st, nil
}

func (uc *StreamUsecase) ensurePlayback(ctx context.Context, ref entity.SessionRef) {
	if ps, _ := redisc.GetPlayback(ctx, uc.rdb, ref); ps == nil {
		redisc.SetPlayback(ctx, uc.rdb, ref, &redisc.PlaybackState{})
	}
}

func (uc *StreamUsecase) ListPublicStreams(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	streams, err := uc.streamRepo.ListPublic()
	if err != nil {
		return nil, err
	}
	type cmds struct {
		presence *redis.IntCmd
		qlen     *redis.IntCmd
		playing  *redis.StringCmd
	}
	pipe := uc.rdb.Pipeline()
	c := make([]cmds, len(streams))
	for i, s := range streams {
		streamID, _ := s["id"].(string)
		ref := entity.NewStreamRef(streamID)
		c[i] = cmds{
			presence: pipe.HLen(ctx, redisc.SessionKey(ref, "presence")),
			qlen:     pipe.LLen(ctx, redisc.SessionKey(ref, "queue")),
			playing:  pipe.HGet(ctx, redisc.SessionKey(ref, "state"), "is_playing"),
		}
	}
	_, _ = pipe.Exec(ctx)
	for i, s := range streams {
		n, _ := c[i].presence.Result()
		s["user_count"] = int(n)
		q, _ := c[i].qlen.Result()
		s["track_count"] = int(q)
		v, _ := c[i].playing.Result()
		s["is_playing"] = v == "1"
	}
	sort.Slice(streams, func(i, j int) bool {
		a, _ := streams[i]["user_count"].(int)
		b, _ := streams[j]["user_count"].(int)
		return a > b
	})
	if offset < 0 {
		offset = 0
	}
	if offset >= len(streams) {
		return []map[string]any{}, nil
	}
	streams = streams[offset:]
	if limit >= 0 && len(streams) > limit {
		streams = streams[:limit]
	}
	return streams, nil
}

func (uc *StreamUsecase) IsController(st *entity.Stream, userID string) bool {
	if userID == "" {
		return false
	}
	if userID == st.OwnerID {
		return true
	}
	user, err := uc.userRepo.FindByID(userID)
	return err == nil && user.IsAdmin()
}

func (uc *StreamUsecase) UpdateMashupRadio(ctx context.Context, st *entity.Stream, enabled *bool, filter *entity.StreamFilter) error {
	if enabled != nil {
		st.Features.MashupRadio.Enabled = *enabled
	}
	if filter != nil {
		st.Features.MashupRadio.Filter = *filter
	}
	if err := uc.streamRepo.Save(st); err != nil {
		return err
	}
	return redisc.SetMashupRadioFeature(ctx, uc.rdb, st.Ref(), st.Features.MashupRadio)
}

func (uc *StreamUsecase) DeleteStream(ctx context.Context, st *entity.Stream) error {
	redisc.DeleteStream(ctx, uc.rdb, st.ID)
	return uc.streamRepo.Delete(st.ID)
}

func (uc *StreamUsecase) SaveTracks(ctx context.Context, st *entity.Stream) error {
	if uc.sessionRepo == nil {
		return nil
	}
	tracks, err := redisc.GetQueue(ctx, uc.rdb, st.Ref())
	if err != nil {
		return fmt.Errorf("get stream queue from redis: %w", err)
	}
	return uc.sessionRepo.SaveTracks(st.Ref(), tracks)
}

func (uc *StreamUsecase) BuildToDict(ctx context.Context, st *entity.Stream) map[string]any {
	return redisc.BuildStreamToDict(ctx, uc.rdb, st)
}

package usecase

// Business rules for the room queue lifecycle, pinned by tests:
//
//  1. Skipping a track removes its per-track state: its votes are deleted
//     (a re-added track with the same ID starts clean) and skip votes reset.
//  2. A finished/skipped track is never recommended by radio again.
//  3. Switching tracks (next/prev/jump) resets skip votes: they were cast
//     against the previous current track.

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

func TestSkipClearsVotesAndSkipVotes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{
		{ID: "t1", Title: "Song 1", MediaID: "m1", SourceURL: "https://x/watch?v=1"},
		{ID: "t2", Title: "Song 2", MediaID: "m2"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	// Votes and skip votes cast against t1.
	redisc.SetVote(ctx, rdb, roomID, "t1", &redisc.VoteEntry{
		LikedBy: []string{"u1"}, Disliked: []string{"u2"},
	})
	redisc.ToggleSkipVote(ctx, rdb, roomID, "u3")

	if !playback.GoNext(ctx, roomID) {
		t.Fatal("expected GoNext to succeed")
	}

	// t1 is gone from the queue.
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 1 || q[0].ID != "t2" {
		t.Fatalf("expected queue [t2], got %+v", q)
	}

	// t1's votes are deleted (no leak onto future same-ID tracks).
	votes, _ := redisc.GetVotes(ctx, rdb, roomID)
	if _, ok := votes["t1"]; ok {
		t.Error("expected vote entry for skipped track t1 to be deleted")
	}

	// Skip votes reset for the new current track.
	if n, _ := redisc.GetSkipVoteCount(ctx, rdb, roomID); n != 0 {
		t.Errorf("expected skip votes reset, got %d", n)
	}

	// The new current track starts with zero votes.
	if likes, dislikes := redisc.GetTrackVotes(ctx, rdb, roomID, "t2"); likes != 0 || dislikes != 0 {
		t.Errorf("expected clean votes for t2, got likes=%d dislikes=%d", likes, dislikes)
	}
}

func TestSkippedTrackNeverRecommendedAgain(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{
		{ID: "t1", Title: "Song 1", MediaID: "m1", SourceURL: "https://x/watch?v=seed"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	if !playback.GoNext(ctx, roomID) {
		t.Fatal("expected GoNext to succeed")
	}

	// Radio suggests the just-skipped track (same media) plus a fresh one.
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://x/watch?v=seed", "https://x/watch?v=fresh"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		if url == "https://x/watch?v=seed" {
			return map[string]any{
				"id": "t1", "media_id": "m1", "title": "Song 1",
				"source_url": url, "duration": float64(200),
			}, nil
		}
		return map[string]any{
			"id": "t9", "media_id": "m9", "title": "Fresh",
			"source_url": url, "duration": float64(200),
		}, nil
	}
	radio := NewRadioUsecase(mediaClient, testRadioConfig(), radioLog, rdb)

	picked, err := radio.Refill(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected refill error: %v", err)
	}
	if len(picked) != 1 {
		t.Fatalf("expected 1 picked track (skipped one filtered), got %d", len(picked))
	}
	if picked[0].MediaID != "m9" {
		t.Errorf("expected fresh track m9, got %s", picked[0].MediaID)
	}
}

func TestReaddedTrackStartsWithCleanVotes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{
		{ID: "t1", Title: "Song 1", MediaID: "m1", SourceURL: "https://x/watch?v=1"},
		{ID: "t2", Title: "Song 2", MediaID: "m2"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	// t1 gets disliked → auto-skip path equivalent: GoNext.
	ve := &redisc.VoteEntry{}
	ve.ApplyVote("u1", -1)
	redisc.SetVote(ctx, rdb, roomID, "t1", ve)
	playback.GoNext(ctx, roomID)

	// Same track comes back (user re-add or radio) and is appended.
	redisc.AppendTrack(ctx, rdb, roomID, &entity.Track{ID: "t1", Title: "Song 1", MediaID: "m1"})

	if likes, dislikes := redisc.GetTrackVotes(ctx, rdb, roomID, "t1"); likes != 0 || dislikes != 0 {
		t.Errorf("re-added track must start clean, got likes=%d dislikes=%d", likes, dislikes)
	}
}

func TestJumpResetsSkipVotes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	redisc.ToggleSkipVote(ctx, rdb, roomID, "u1")

	if !playback.JumpTo(ctx, roomID, 2) {
		t.Fatal("expected JumpTo to succeed")
	}
	if n, _ := redisc.GetSkipVoteCount(ctx, rdb, roomID); n != 0 {
		t.Errorf("expected skip votes reset after jump, got %d", n)
	}
}

func TestPrevResetsSkipVotes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 1})

	redisc.ToggleSkipVote(ctx, rdb, roomID, "u1")

	if !playback.GoPrev(ctx, roomID) {
		t.Fatal("expected GoPrev to succeed")
	}
	if n, _ := redisc.GetSkipVoteCount(ctx, rdb, roomID); n != 0 {
		t.Errorf("expected skip votes reset after prev, got %d", n)
	}
}

// Votes for tracks absent from the queue are ignored: they come from stale
// clients and previously created leaked entries plus phantom auto-skips.
func TestRegisterVoteIgnoresAbsentTrack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	// Stale client votes for the already-removed track: must be a no-op,
	// must NOT create an entry, must NOT skip the current track.
	if playback.RegisterVote(ctx, roomID, "u1", "ghost", -1) {
		t.Error("expected no skip for absent track")
	}
	votes, _ := redisc.GetVotes(ctx, rdb, roomID)
	if _, ok := votes["ghost"]; ok {
		t.Error("expected no vote entry for absent track")
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 2 || q[0].ID != "t1" {
		t.Errorf("current track must not be skipped by stale vote, queue=%+v", q)
	}
}

// Dislike auto-skip fires only for the current track: votes on a queued
// track must never skip whatever is playing.
func TestRegisterVoteNoSkipForQueuedTrack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	if playback.RegisterVote(ctx, roomID, "u1", "t2", -1) {
		t.Error("expected no skip: t2 is queued, not current")
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 2 || q[0].ID != "t1" {
		t.Errorf("current track t1 must keep playing, queue=%+v", q)
	}
	// The vote itself is still recorded on t2.
	if _, dislikes := redisc.GetTrackVotes(ctx, rdb, roomID, "t2"); dislikes != 1 {
		t.Errorf("expected 1 dislike recorded on t2, got %d", dislikes)
	}
}

// Disliking the current track past the threshold skips it.
func TestRegisterVoteSkipsCurrentTrack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	if !playback.RegisterVote(ctx, roomID, "u1", "t1", -1) {
		t.Fatal("expected skip for disliked current track")
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 1 || q[0].ID != "t2" {
		t.Errorf("expected queue [t2] after auto-skip, got %+v", q)
	}
}

// A track that becomes current starts with a clean slate, even if votes
// were cast on it while it was queued.
func TestBecomingCurrentClearsVotes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	// Pre-vote on the queued track (no skip: not current).
	playback.RegisterVote(ctx, roomID, "u1", "t2", -1)

	if !playback.JumpTo(ctx, roomID, 1) {
		t.Fatal("expected JumpTo to succeed")
	}
	if likes, dislikes := redisc.GetTrackVotes(ctx, rdb, roomID, "t2"); likes != 0 || dislikes != 0 {
		t.Errorf("new current track must start clean, got likes=%d dislikes=%d", likes, dislikes)
	}
}

// EnsureRoomMedia must not resurrect a track skipped while media resolution
// was in flight (Ensure() does network I/O). Regression: the old code wrote
// back its stale pre-read snapshot with a full-queue SetQueue.
func TestEnsureRoomMediaDoesNotResurrectSkippedTrack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	tracks := []*entity.Track{
		{ID: "t1", Title: "Song 1", MediaID: "m1", SourceURL: "https://x/watch?v=1"},
		{ID: "t2", Title: "Song 2", MediaID: "m2", SourceURL: "https://x/watch?v=2"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		// A concurrent skip lands while Ensure() is "in flight".
		redisc.RemoveAt(ctx, rdb, roomID, 0)
		return []string{"m1"}, nil
	}
	mediaUC := NewMediaUsecase(mediaClient, testConfig(), testLog2, rdb)

	mediaUC.EnsureRoomMedia(ctx, roomID)

	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 1 || q[0].ID != "t2" {
		t.Errorf("skipped track resurrected by EnsureRoomMedia, queue=%+v", q)
	}
}

// Positive path: resolved URLs are stamped onto the tracks in place.
func TestEnsureRoomMediaStampsResolvedURL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	tracks := []*entity.Track{
		{ID: "t1", Title: "Song 1", MediaID: "m1", SourceURL: "https://x/watch?v=1"},
		{ID: "t2", Title: "Song 2", MediaID: "m2", SourceURL: "https://x/watch?v=2"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		return []string{"m1"}, nil
	}
	mediaUC := NewMediaUsecase(mediaClient, testConfig(), testLog2, rdb)

	if !mediaUC.EnsureRoomMedia(ctx, roomID) {
		t.Fatal("expected changed=true")
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 2 {
		t.Fatalf("queue must be untouched in length, got %+v", q)
	}
	if q[0].URL == "" || q[0].LocalPath == "" {
		t.Errorf("expected resolved URL on t1, got %+v", q[0])
	}
	if q[1].URL != "" {
		t.Errorf("t2 was not ready, must stay unresolved, got %+v", q[1])
	}
}

// The last-track-never-skips race: Refill snapshots the queue, then
// concurrent activity lands mid-flight (a skip marking media seen, a second
// refill appending the same pick). The stale picks must be dropped at append
// time instead of duplicating/resurrecting tracks.
func TestStaleRefillPicksDroppedAtAppend(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	tracks := []*entity.Track{
		{ID: "t1", Title: "Last", MediaID: "m1", SourceURL: "https://x/watch?v=seed"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0, IsPlaying: true})

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://x/watch?v=fresh"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		// Concurrent activity lands mid-refill: the pick's media gets marked
		// seen (a skip elsewhere) AND appended (an overlapping refill).
		redisc.AddRadioSeen(ctx, rdb, roomID, "mx")
		redisc.AppendTrack(ctx, rdb, roomID, &entity.Track{ID: "rx", MediaID: "mx"})
		return map[string]any{
			"id": "rx", "media_id": "mx", "title": "Fresh",
			"source_url": url, "duration": float64(200),
		}, nil
	}
	radio := NewRadioUsecase(mediaClient, testRadioConfig(), radioLog, rdb)

	// Refill picks rx (its snapshot predates the concurrent activity).
	picked, err := radio.Refill(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected refill error: %v", err)
	}
	if len(picked) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(picked))
	}

	// Append path (backgroundRefill) must drop the stale pick twice over:
	// same ID already queued AND media already seen.
	appended := 0
	for _, tr := range picked {
		if ok, _ := redisc.AppendFreshTrack(ctx, rdb, roomID, tr); ok {
			appended++
		}
	}
	if appended != 0 {
		t.Errorf("expected stale pick dropped, appended %d", appended)
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 2 {
		t.Errorf("expected queue [t1 rx] without duplicates, got %+v", q)
	}
}

// GoNext marks the finished track's media seen so radio never re-picks it.
func TestGoNextMarksRadioSeen(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{
		{ID: "t1", MediaID: "m1", SourceURL: "https://x/watch?v=1"},
		{ID: "t2", MediaID: "m2"},
	}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	if !playback.GoNext(ctx, roomID) {
		t.Fatal("expected GoNext to succeed")
	}
	if seen, _ := redisc.IsRadioSeen(ctx, rdb, roomID, "m1"); !seen {
		t.Error("expected finished track media marked seen")
	}
}

// ForceNext bypasses the duplicate-suppression window for explicit votes.
func TestForceNextBypassesDedup(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	playback := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	setupPlaybackRoom(t, rdb, roomID, tracks, &redisc.PlaybackState{CurrentIndex: 0})

	if !playback.GoNext(ctx, roomID) {
		t.Fatal("expected first GoNext to succeed")
	}
	if playback.GoNext(ctx, roomID) {
		t.Error("expected deduped GoNext to be suppressed")
	}
	if !playback.ForceNext(ctx, roomID) {
		t.Error("expected ForceNext to bypass suppression")
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 1 || q[0].ID != "t3" {
		t.Errorf("expected queue [t3], got %+v", q)
	}
}

// AppendFreshTrack unit rules: dup IDs and seen media are dropped,
// fresh tracks append and get marked seen.
func TestAppendFreshTrack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	setupPlaybackRoom(t, rdb, roomID,
		[]*entity.Track{{ID: "t1", MediaID: "m1"}},
		&redisc.PlaybackState{CurrentIndex: 0})

	if ok, _ := redisc.AppendFreshTrack(ctx, rdb, roomID,
		&entity.Track{ID: "t1", MediaID: "m9"}); ok {
		t.Error("expected dup ID dropped")
	}
	redisc.AddRadioSeen(ctx, rdb, roomID, "m1")
	if ok, _ := redisc.AppendFreshTrack(ctx, rdb, roomID,
		&entity.Track{ID: "t9", MediaID: "m1"}); ok {
		t.Error("expected seen media dropped")
	}
	if ok, _ := redisc.AppendFreshTrack(ctx, rdb, roomID,
		&entity.Track{ID: "t3", MediaID: "m3"}); !ok {
		t.Error("expected fresh track appended")
	}
	if seen, _ := redisc.IsRadioSeen(ctx, rdb, roomID, "m3"); !seen {
		t.Error("expected appended track marked seen")
	}
	q, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(q) != 2 || q[1].ID != "t3" {
		t.Errorf("expected queue [t1 t3], got %+v", q)
	}
}

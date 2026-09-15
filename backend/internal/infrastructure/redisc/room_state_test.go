package redisc

import "testing"

func TestApplyVoteLikeIsIdempotentPerUser(t *testing.T) {
	ve := &VoteEntry{}
	ve.ApplyVote("u1", 1)
	ve.ApplyVote("u1", 1)
	ve.ApplyVote("u1", 1)
	if got := ve.LikeCount(); got != 1 {
		t.Errorf("one user liking 3 times: expected 1 like, got %d", got)
	}
}

func TestApplyVoteRetract(t *testing.T) {
	ve := &VoteEntry{}
	ve.ApplyVote("u1", 1)
	ve.ApplyVote("u1", 0)
	if got := ve.LikeCount(); got != 0 {
		t.Errorf("after retract: expected 0 likes, got %d", got)
	}
	if got := ve.DislikeCount(); got != 0 {
		t.Errorf("after retract: expected 0 dislikes, got %d", got)
	}
}

func TestApplyVoteSwitchSides(t *testing.T) {
	ve := &VoteEntry{}
	ve.ApplyVote("u1", 1)
	ve.ApplyVote("u1", -1)
	if got := ve.LikeCount(); got != 0 {
		t.Errorf("after switch to dislike: expected 0 likes, got %d", got)
	}
	if got := ve.DislikeCount(); got != 1 {
		t.Errorf("after switch to dislike: expected 1 dislike, got %d", got)
	}

	ve.ApplyVote("u1", 1)
	if got := ve.LikeCount(); got != 1 {
		t.Errorf("after switch back to like: expected 1 like, got %d", got)
	}
	if got := ve.DislikeCount(); got != 0 {
		t.Errorf("after switch back to like: expected 0 dislikes, got %d", got)
	}
}

func TestApplyVoteMultipleUsers(t *testing.T) {
	ve := &VoteEntry{}
	ve.ApplyVote("u1", 1)
	ve.ApplyVote("u2", 1)
	ve.ApplyVote("u3", -1)
	if got := ve.LikeCount(); got != 2 {
		t.Errorf("expected 2 likes, got %d", got)
	}
	if got := ve.DislikeCount(); got != 1 {
		t.Errorf("expected 1 dislike, got %d", got)
	}
}

func TestApplyVoteDislikeIsIdempotentPerUser(t *testing.T) {
	ve := &VoteEntry{}
	ve.ApplyVote("u1", -1)
	ve.ApplyVote("u1", -1)
	if got := ve.DislikeCount(); got != 1 {
		t.Errorf("one user disliking twice: expected 1 dislike, got %d", got)
	}
}

func TestLikeCountIncludesLegacyBase(t *testing.T) {
	ve := &VoteEntry{Likes: 3}
	ve.ApplyVote("u1", 1)
	if got := ve.LikeCount(); got != 4 {
		t.Errorf("expected legacy 3 + 1 per-user = 4, got %d", got)
	}
}

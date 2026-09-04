import time
import pytest
import models


def test_room_to_dict_empty_queue():
    r = models.Room()
    d = r.to_dict()
    assert d["current_track"] is None
    assert d["queue"] == []
    assert d["current_index"] == 0


def test_room_to_dict_multiple_tracks():
    r = models.Room()
    r.queue = [
        models.Track(id="t1", title="A"),
        models.Track(id="t2", title="B"),
        models.Track(id="t3", title="C"),
    ]
    r.current_index = 1
    d = r.to_dict()
    assert d["current_track"]["id"] == "t2"
    assert len(d["queue"]) == 3


def test_room_current_track_index_boundary():
    r = models.Room()
    r.queue = [models.Track(id="a")]
    r.current_index = 1
    assert r.current_track() is None
    r.current_index = -1
    assert r.current_track() is None


def test_room_listeners_empty():
    r = models.Room()
    assert r.listeners() == []


def test_room_listeners_multiple():
    r = models.Room()
    r.presence = {
        "ws1": {"id": "u1", "name": "Alice"},
        "ws2": {"id": "u2", "name": "Bob"},
    }
    listeners = r.listeners()
    assert len(listeners) == 2
    ids = {l["id"] for l in listeners}
    assert ids == {"u1", "u2"}


def test_room_skip_votes_is_set():
    r = models.Room()
    assert isinstance(r.skip_votes, set)
    r.skip_votes.add("u1")
    assert "u1" in r.skip_votes


def test_chat_message_to_dict_has_id():
    msg = models.ChatMessage(user_id="u1", username="A", text="hi")
    d = msg.to_dict()
    assert "id" in d
    assert len(d["id"]) > 0


def test_track_to_dict_has_all_fields():
    t = models.Track(
        id="abc", title="Song", artist="Band", url="/api/media/abc",
        thumbnail="http://t", duration=200, added_by="User",
    )
    d = t.to_dict()
    assert d["id"] == "abc"
    assert d["title"] == "Song"
    assert d["artist"] == "Band"
    assert d["duration"] == 200


def test_track_to_dict_without_optional_fields():
    t = models.Track(id="x")
    d = t.to_dict()
    assert d["url"] == ""
    assert d["thumbnail"] == ""
    assert d["duration"] == 0


def test_track_vote_values():
    v = models.TrackVote(user_id="u1", track_id="t1", vote=1)
    assert v.vote == 1
    v2 = models.TrackVote(user_id="u2", track_id="t1", vote=-1)
    assert v2.vote == -1


def test_room_get_track_votes_mixed():
    r = models.Room()
    r.votes.append(models.TrackVote(user_id="u1", track_id="t1", vote=1))
    r.votes.append(models.TrackVote(user_id="u2", track_id="t1", vote=1))
    r.votes.append(models.TrackVote(user_id="u3", track_id="t1", vote=-1))
    r.votes.append(models.TrackVote(user_id="u4", track_id="t2", vote=1))
    v = r.get_track_votes("t1")
    assert v["likes"] == 2
    assert v["dislikes"] == 1
    v2 = r.get_track_votes("t2")
    assert v2["likes"] == 1
    assert v2["dislikes"] == 0


def test_room_get_current_position_playing():
    r = models.Room()
    r.position = 10.0
    r.is_playing = True
    r.last_sync_at = time.time() - 2.0
    pos = r.get_current_position()
    assert 11.5 <= pos <= 12.5


def test_room_get_current_position_paused():
    r = models.Room()
    r.position = 42.0
    r.is_playing = False
    assert r.get_current_position() == 42.0


def test_room_to_dict_includes_user_count():
    r = models.Room()
    d = r.to_dict()
    assert d["user_count"] == 0
    r.presence = {"ws1": {"id": "u1", "name": "A"}}
    d = r.to_dict()
    assert d["user_count"] == 1


def test_room_password_hash_not_in_dict():
    r = models.Room(password_hash="secret")
    d = r.to_dict()
    assert "password_hash" not in d


def test_room_has_password_flag():
    r = models.Room()
    assert r.to_dict()["has_password"] is False
    r.password_hash = "x"
    assert r.to_dict()["has_password"] is True

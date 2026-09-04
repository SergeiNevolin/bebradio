import time
import pytest
import models


def test_track_defaults():
    t = models.Track()
    assert len(t.id) == 8
    assert t.title == ""
    assert t.added_by == "Anonymous"
    assert t.added_at > 0


def test_track_to_dict():
    t = models.Track(id="abc123", title="Song", artist="Artist", url="http://x", duration=200, added_by="Bob")
    d = t.to_dict()
    assert d["id"] == "abc123"
    assert d["title"] == "Song"
    assert d["artist"] == "Artist"
    assert d["duration"] == 200
    assert d["added_by"] == "Bob"


def test_track_local_path_default():
    t = models.Track()
    assert t.local_path == ""


def test_track_to_dict_includes_url():
    t = models.Track(id="x", url="/api/media/x")
    d = t.to_dict()
    assert d["url"] == "/api/media/x"


def test_room_defaults():
    r = models.Room()
    assert len(r.id) == 6
    assert r.id == r.id.upper()
    assert r.queue == []
    assert r.is_playing is False


def test_room_current_track_empty():
    r = models.Room()
    assert r.current_track() is None


def test_room_current_track():
    r = models.Room()
    t = models.Track(id="abc")
    r.queue.append(t)
    assert r.current_track().id == "abc"


def test_room_current_track_out_of_bounds():
    r = models.Room()
    r.queue.append(models.Track())
    r.current_index = 5
    assert r.current_track() is None


def test_room_to_dict():
    r = models.Room(name="Test")
    r.queue.append(models.Track(id="t1", title="Song"))
    d = r.to_dict()
    assert d["name"] == "Test"
    assert len(d["queue"]) == 1
    assert d["current_track"]["id"] == "t1"
    assert d["user_count"] == 0


def test_room_to_dict_empty():
    r = models.Room()
    d = r.to_dict()
    assert d["current_track"] is None
    assert d["queue"] == []


def test_room_get_current_position_paused():
    r = models.Room()
    r.position = 10.0
    r.is_playing = False
    assert r.get_current_position() == 10.0


def test_room_get_current_position_playing():
    r = models.Room()
    r.position = 10.0
    r.is_playing = True
    r.last_sync_at = time.time() - 5.0
    pos = r.get_current_position()
    assert 14.5 <= pos <= 15.5


def test_room_to_dict_position_playing():
    r = models.Room()
    r.position = 20.0
    r.is_playing = True
    r.last_sync_at = time.time() - 3.0
    d = r.to_dict()
    assert 22.5 <= d["position"] <= 23.5


def test_room_to_dict_position_paused():
    r = models.Room()
    r.position = 20.0
    r.is_playing = False
    d = r.to_dict()
    assert d["position"] == 20.0


def test_chat_message_model():
    msg = models.ChatMessage(user_id="u1", username="Alice", text="Hello!")
    d = msg.to_dict()
    assert d["user_id"] == "u1"
    assert d["username"] == "Alice"
    assert d["text"] == "Hello!"
    assert "id" in d
    assert "created_at" in d


def test_room_messages_default_empty():
    r = models.Room()
    assert r.messages == []


def test_track_vote_model():
    v = models.TrackVote(user_id="u1", track_id="t1", vote=1)
    assert v.user_id == "u1"
    assert v.vote == 1


def test_room_votes_default_empty():
    r = models.Room()
    assert r.votes == []


def test_get_track_votes():
    r = models.Room()
    r.votes.append(models.TrackVote(user_id="u1", track_id="t1", vote=1))
    r.votes.append(models.TrackVote(user_id="u2", track_id="t1", vote=1))
    r.votes.append(models.TrackVote(user_id="u3", track_id="t1", vote=-1))
    votes = r.get_track_votes("t1")
    assert votes["likes"] == 2
    assert votes["dislikes"] == 1


def test_get_track_votes_empty():
    r = models.Room()
    votes = r.get_track_votes("nonexistent")
    assert votes["likes"] == 0
    assert votes["dislikes"] == 0


def test_skip_votes_default_empty():
    r = models.Room()
    assert r.skip_votes == set()


def test_room_to_dict_includes_skip_voters():
    r = models.Room()
    r.skip_votes.add("u1")
    r.skip_votes.add("u2")
    d = r.to_dict()
    assert "skip_voters" in d
    assert len(d["skip_voters"]) == 2


def test_room_to_dict_has_password_flag():
    r = models.Room()
    assert r.to_dict()["has_password"] is False
    r.password_hash = "hashed"
    assert r.to_dict()["has_password"] is True


def test_room_to_dict_never_leaks_password_hash():
    r = models.Room(password_hash="super-secret-hash")
    assert "password_hash" not in r.to_dict()


def test_room_to_dict_exposes_listeners_and_auto_radio():
    r = models.Room()
    d = r.to_dict()
    assert d["listeners"] == []
    assert d["auto_radio"] is False


def test_listeners_dedup_by_identity():
    r = models.Room()
    r.presence = {
        "ws1": {"id": "u1", "name": "Alice"},
        "ws2": {"id": "u1", "name": "Alice"},
        "ws3": {"id": "anon:1", "name": "Guest"},
    }
    listeners = r.listeners()
    assert {l["id"] for l in listeners} == {"u1", "anon:1"}
    assert r.to_dict()["user_count"] == 2


def test_room_to_dict_queue_includes_local_path():
    r = models.Room()
    r.queue = [models.Track(id="a", url="/api/media/a")]
    d = r.to_dict()
    assert d["queue"][0]["url"] == "/api/media/a"


def test_track_from_youtube_maps_info_dict():
    info = {
        "title": "Song", "artist": "Band",
        "thumbnail": "http://t", "duration": 123,
        "source_url": "https://youtu.be/abc",
    }
    t = models.Track.from_youtube(info, added_by="Alice")
    assert (t.title, t.artist, t.thumbnail, t.duration) == (
        "Song", "Band", "http://t", 123,
    )
    assert t.added_by == "Alice"
    assert t.source_url == "https://youtu.be/abc"


def test_track_from_youtube_tolerates_missing_optional_fields():
    t = models.Track.from_youtube({}, added_by="Radio")
    assert t.title == "Unknown"
    assert t.source_url == ""

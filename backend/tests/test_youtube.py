import time


def test_video_id_extraction():
    from youtube import video_id
    assert video_id("https://www.youtube.com/watch?v=dQw4w9WgXcQ") == "dQw4w9WgXcQ"
    assert video_id("https://youtu.be/dQw4w9WgXcQ") == "dQw4w9WgXcQ"
    assert video_id("https://www.youtube.com/shorts/dQw4w9WgXcQ") == "dQw4w9WgXcQ"
    assert video_id("not a url") == ""


def test_parse_stream_expiry_reads_expire_param():
    from youtube import parse_stream_expiry
    assert parse_stream_expiry("https://r1.googlevideo.com/videoplayback?expire=1700000000&x=y") == 1700000000.0


def test_parse_stream_expiry_falls_back_when_absent():
    from youtube import parse_stream_expiry
    assert parse_stream_expiry("https://example.com/stream.m4a") > time.time()

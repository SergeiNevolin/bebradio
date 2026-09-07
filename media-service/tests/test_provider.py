import json
from pathlib import Path


def test_provider_media_id_is_stable_and_opaque(settings):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)
    first = provider.media_id("dQw4w9WgXcQ")
    second = provider.media_id("dQw4w9WgXcQ")

    assert first == second
    assert first.startswith("media_")
    assert "dQw4w9WgXcQ" not in first


def test_resolve_returns_media_service_contract(settings, monkeypatch):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)
    payload = {
        "id": "dQw4w9WgXcQ",
        "title": "Song",
        "uploader": "Artist",
        "thumbnail": "https://img.test/song.jpg",
        "duration": 180,
        "webpage_url": "https://youtube.com/watch?v=dQw4w9WgXcQ",
    }

    class Result:
        returncode = 0
        stdout = json.dumps(payload)

    monkeypatch.setattr(provider, "_run", lambda *args, **kwargs: Result())

    result = provider.resolve("https://youtu.be/dQw4w9WgXcQ")

    assert result["media_id"] == provider.media_id("dQw4w9WgXcQ")
    assert result["title"] == "Song"
    assert "id" not in result


def test_search_skips_invalid_json_and_empty_items(settings, monkeypatch):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)

    class Result:
        returncode = 0
        stdout = "not json\n" + json.dumps({"title": "Missing ID"}) + "\n" + json.dumps({"id": "abc12345678", "title": "Found"})

    monkeypatch.setattr(provider, "_run", lambda *args, **kwargs: Result())

    result = provider.search("found", 5)

    assert len(result) == 1
    assert result[0]["title"] == "Found"
    assert result[0]["media_id"].startswith("media_")


def test_download_requests_subtitles_alongside_audio(settings, monkeypatch):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)
    captured = {}

    class Result:
        returncode = 0

    def fake_run(args, timeout=60):
        captured["args"] = args
        return Result()

    monkeypatch.setattr(provider, "_run", fake_run)

    ok = provider.download("https://youtu.be/x", Path("/tmp/media_x.m4a"))

    assert ok is True
    args = captured["args"]
    assert "--write-subs" in args
    assert "--write-auto-subs" in args
    assert args[args.index("--sub-langs") + 1] == "ru.*,en.*"
    assert "--convert-subs" not in args  # ffmpeg is not in the media-service image


def test_parse_vtt_removes_tags_and_decodes_entities(settings):
    from providers.youtube import YouTubeProvider

    raw = """WEBVTT\n\n00:00:01.000 --> 00:00:03.500\n<b>Hello &amp; welcome</b>\n"""

    cues = YouTubeProvider._parse_vtt(raw)

    assert cues == [{"start": 1.0, "dur": 2.5, "text": "Hello & welcome"}]


def test_read_vtt_file_flags_asr_captions_as_auto(settings, tmp_path):
    from providers.youtube import YouTubeProvider

    manual = tmp_path / "media_x.en.vtt"
    manual.write_text("WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello there\n")
    asr = tmp_path / "media_x.ru.vtt"
    asr.write_text(
        "WEBVTT\n\n00:00:01.000 --> 00:00:03.000 align:start position:0%\n"
        "<00:00:01.240><c>Привет</c>\n"
    )

    assert YouTubeProvider.read_vtt_file(manual) == {
        "auto": False,
        "cues": [{"start": 1.0, "dur": 2.0, "text": "Hello there"}],
    }
    assert YouTubeProvider.read_vtt_file(asr)["auto"] is True

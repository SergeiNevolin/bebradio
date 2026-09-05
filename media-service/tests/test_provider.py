import json
import urllib.error


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


def test_parse_vtt_removes_tags_and_decodes_entities(settings):
    from providers.youtube import YouTubeProvider

    raw = """WEBVTT\n\n00:00:01.000 --> 00:00:03.500\n<b>Hello &amp; welcome</b>\n"""

    cues = YouTubeProvider._parse_vtt(raw)

    assert cues == [{"start": 1.0, "dur": 2.5, "text": "Hello & welcome"}]


def test_captions_rate_limit_returns_empty_result_without_raising(settings, monkeypatch):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)

    class Result:
        returncode = 0
        stdout = json.dumps({
            "id": "dQw4w9WgXcQ",
            "subtitles": {"en": [{"ext": "vtt", "url": "https://captions.test/en.vtt"}]},
        })

    def fail_request(*args, **kwargs):
        raise urllib.error.HTTPError("https://captions.test/en.vtt", 429, "Too Many Requests", {}, None)

    monkeypatch.setattr(provider, "_run", lambda *args, **kwargs: Result())
    monkeypatch.setattr(urllib.request, "urlopen", fail_request)

    result = provider.captions("https://youtube.com/watch?v=dQw4w9WgXcQ", "")

    assert result == {"lang": "", "auto": False, "cues": []}

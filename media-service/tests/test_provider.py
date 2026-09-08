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


def test_download_fetches_audio_first_then_captions_in_separate_passes(settings, monkeypatch):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)
    calls = []

    def fake_run(args, timeout=60):
        calls.append(args)
        return object()  # truthy => success

    monkeypatch.setattr(provider, "_run", fake_run)

    ok = provider.download("https://youtu.be/x", Path("/tmp/media_x.m4a"))

    assert ok is True
    assert len(calls) == 3
    audio, manual, auto = calls

    # Audio pass carries no subtitle flags: a captions rate-limit can't fail it.
    assert "--write-subs" not in audio and "--write-auto-subs" not in audio
    assert "-f" in audio and audio[audio.index("-f") + 1] == "bestaudio/best"

    assert "--write-subs" in manual and "--write-auto-subs" not in manual
    assert "--write-auto-subs" in auto and "--write-subs" not in auto

    for pass_args in (manual, auto):
        assert pass_args[pass_args.index("--sub-langs") + 1] == "en,en-orig,ru,ru-orig"
        assert "--skip-download" in pass_args
        assert "--convert-subs" not in pass_args  # no ffmpeg in the image

    # ASR captions are written to a distinct .auto.<lang>.vtt file.
    auto_out = auto[auto.index("-o") + 1]
    assert auto_out.startswith("subtitle:") and auto_out.endswith("media_x.auto.%(ext)s")
    manual_out = manual[manual.index("-o") + 1]
    assert not manual_out.startswith("subtitle:") and manual_out.endswith("media_x.%(ext)s")


def test_download_returns_false_when_audio_pass_fails(settings, monkeypatch):
    from providers.youtube import YouTubeProvider

    provider = YouTubeProvider(settings)
    calls = []

    def fake_run(args, timeout=60):
        calls.append(args)
        return None  # audio pass fails

    monkeypatch.setattr(provider, "_run", fake_run)

    assert provider.download("https://youtu.be/x", Path("/tmp/media_x.m4a")) is False
    assert len(calls) == 1  # no caption passes attempted


def test_parse_vtt_removes_tags_and_decodes_entities(settings):
    from providers.youtube import YouTubeProvider

    raw = """WEBVTT\n\n00:00:01.000 --> 00:00:03.500\n<b>Hello &amp; welcome</b>\n"""

    cues = YouTubeProvider._parse_vtt(raw)

    assert cues == [{"start": 1.0, "dur": 2.5, "text": "Hello & welcome"}]


def test_parse_vtt_file_reads_and_parses(settings, tmp_path):
    from providers.youtube import YouTubeProvider

    vtt = tmp_path / "media_x.en.vtt"
    vtt.write_text(
        "WEBVTT\n\n00:00:01.000 --> 00:00:03.000 align:start position:0%\n"
        "<00:00:01.240><c>Hello</c> there\n"
    )

    assert YouTubeProvider.parse_vtt_file(vtt) == [
        {"start": 1.0, "dur": 2.0, "text": "Hello there"}
    ]

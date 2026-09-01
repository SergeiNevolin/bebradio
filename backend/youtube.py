import json
import subprocess
from typing import Optional


YT_DLP_COMMON = ["yt-dlp", "--js-runtimes", "node"]


def fetch_track(url: str) -> Optional[dict]:
    """Fetch video info and stream URL from YouTube."""
    try:
        info_result = subprocess.run(
            [*YT_DLP_COMMON, "--dump-json", "--no-download", "--no-playlist", url],
            capture_output=True,
            text=True,
            timeout=60,
        )
        if info_result.returncode != 0:
            return None
        data = json.loads(info_result.stdout)

        stream_result = subprocess.run(
            [*YT_DLP_COMMON, "-f", "bestaudio[ext=m4a]/bestaudio", "-g", "--no-playlist", url],
            capture_output=True,
            text=True,
            timeout=60,
        )
        if stream_result.returncode != 0:
            return None

        return {
            "title": data.get("title", "Unknown"),
            "artist": data.get("uploader", data.get("channel", "Unknown")),
            "thumbnail": data.get("thumbnail", ""),
            "duration": data.get("duration", 0),
            "stream_url": stream_result.stdout.strip().split("\n")[0],
        }
    except Exception:
        return None


def search_youtube(query: str, limit: int = 5) -> list[dict]:
    """Search YouTube and return a list of results."""
    try:
        result = subprocess.run(
            [
                *YT_DLP_COMMON,
                f"ytsearch{limit}:{query}",
                "--dump-json", "--no-download", "--flat-playlist",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode != 0:
            return []

        items = []
        for line in result.stdout.strip().split("\n"):
            if not line:
                continue
            try:
                data = json.loads(line)
                video_id = data.get("id", "")
                items.append({
                    "id": video_id,
                    "title": data.get("title", "Unknown"),
                    "artist": data.get("uploader", data.get("channel", "Unknown")),
                    "thumbnail": data.get("thumbnail", f"https://i.ytimg.com/vi/{video_id}/hqdefault.jpg"),
                    "duration": data.get("duration", 0),
                    "url": f"https://www.youtube.com/watch?v={video_id}",
                })
            except json.JSONDecodeError:
                continue
        return items
    except Exception:
        return []

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parents[1]))


@pytest.fixture
def settings(tmp_path):
    from config import Settings

    return Settings(
        media_dir=tmp_path / "tracks",
        media_ttl=3600,
        media_max_size=1024,
        max_downloads=2,
        bgutil_base_url="http://provider:4416",
        mashup_dir=tmp_path / "mashups",
        mashup_max_size=1024,
        mashup_max_duration=10,
        mashup_total_limit=4096,
        mashup_max_jobs=2,
        mashup_cover_max_size=512,
    )

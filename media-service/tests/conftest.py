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
    )

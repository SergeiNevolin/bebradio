from api import create_app
from config import Settings
from service import MediaService


app = create_app(MediaService(Settings.from_env()))

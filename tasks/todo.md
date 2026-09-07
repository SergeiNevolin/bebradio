# TODO: Возврат режима «Караоке»

План: `tasks/plan.md`. Спека: `SPEC.md`. Ветка: `feature/karaoke-offline-captions` от `main`.

Порядок: T1 ‖ T2 → T3 → T4 → T5 → T6. Коммит после каждой задачи, тесты до коммита.
Каждая задача ≤ 5 файлов, одна фокус-сессия, dependency-ordered.

---

- [ ] **T1 — `storage.py`: аудио и субтитры не путаются**
  - Acceptance:
    - `path(media_id)` / `is_ready(media_id)` учитывают только аудио-расширения
      (список из `_AUDIO_MIME` в `api.py` — вынести в общее место или продублировать),
      `.vtt` их не удовлетворяет.
    - Новый `captions_path(media_id, lang="")`: возвращает путь к `media_<id>*.vtt`
      (приоритет запрошенного языка, затем `ru`, затем `en`, затем любой), игнорирует
      `.part`; нет файла → `None`.
    - `cleanup()` и `_enforce_size_limit()` сверяют принадлежность файла к треку по
      префиксу до первой точки (`media_<hash>`), а не по полному `path.stem`;
      `.vtt` удаляется тогда же, когда его аудио, и не раньше; пока аудио в
      `referenced` — `.vtt` тоже сохраняется.
  - Verify: `cd media-service && python -m pytest`. Новые кейсы в `test_storage.py`:
    `captions_path` по языку и без; `.vtt` переживает `cleanup`, пока аудио
    referenced; `.vtt` уходит вместе с аудио по TTL и по size-limit; `is_ready`
    == False, если на диске только `.vtt`.
  - Files: `media-service/storage.py`, `media-service/tests/test_storage.py`
    (опц. `media-service/api.py` если выносим список аудио-расширений).

- [ ] **T2 — `download()`: `yt-dlp` пишет `.vtt` рядом с аудио**
  - Acceptance:
    - `YouTubeProvider.download()` добавляет `--write-subs --write-auto-subs
      --sub-langs "ru.*,en.*" --sub-format vtt/best --convert-subs vtt`.
    - Отсутствие субтитров у видео не делает загрузку неуспешной (`ensure` → True,
      аудио на месте).
    - При наличии субтитров рядом с `media_<hash>.<ext>` появляется
      `media_<hash>.<lang>.vtt`; реальные имена файлов от `yt-dlp` сверены
      (запустить руками на 1–2 роликах) и учтены в `captions_path()` из T1.
  - Verify: `python -m pytest`. Кейсы в `test_provider.py`: аргументы `yt-dlp`
    содержат флаги субтитров; мок-даунлоадер без `.vtt` → `ensure` == True.
    Ручной прогон: реальный `yt-dlp` на ролике с авто-титрами → проверить имена.
  - Files: `media-service/providers/youtube.py`,
    `media-service/tests/test_provider.py`.

- [ ] **T3 — captions с диска + новый эндпоинт, удалить живой путь**
  - Acceptance:
    - `MediaService.captions_from_disk(media_id, lang)` читает `.vtt` через
      `storage.captions_path()`, парсит существующим `_parse_vtt`, возвращает
      `{"lang": str, "auto": bool, "cues": [...]}`; `auto` = True, если найден
      только авто-файл; нет файла → `{"lang": "", "auto": false, "cues": []}`.
    - `GET /v1/media/{media_id}/captions?lang=` в `api.py`: 400 на невалидном
      `media_id`, иначе 200 с телом выше (в т.ч. пустым).
    - `GET /v1/captions`, `MediaService.captions()` и `YouTubeProvider.captions()`
      удалены. `_parse_vtt`, `_VTT_TS`, `_TAGS` оставлены (нужны парсеру).
    - `_subtitle_cache` в `YouTubeProvider` — удалить, если больше не используется.
  - Verify: `python -m pytest`. `test_api.py`: 200+`cues` с подложенным `.vtt`;
    200+`[]` без файла; 400 на `../x`. `test_provider.py`: убрать/переписать тесты
    удалённого `captions()`. `grep -rn "captions" media-service` — живого пути нет.
  - Files: `media-service/service.py`, `media-service/api.py`,
    `media-service/providers/youtube.py`, `media-service/tests/test_api.py`,
    `media-service/tests/test_provider.py`.

- [ ] **T4 — backend: переименовать `Captions` → `MediaCaptions(mediaID, lang)` в цепочке клиента**
  - Acceptance:
    - `repository.MediaClient`: `Captions(sourceURL, lang)` → `MediaCaptions(mediaID,
      lang string) (map[string]any, error)`.
    - `media.Client.MediaCaptions`: `GET {baseURL}/v1/media/{mediaID}/captions?lang=`;
      сетевая ошибка / не-200 → `{"lang":"","auto":false,"cues":[]}` (как сейчас в
      `Captions`), а не ошибка.
    - `MediaUsecase.FetchSubtitles(sourceURL, lang)` → принимает `mediaID`, зовёт
      `MediaCaptions(mediaID, lang)`.
    - Мок `MockMediaClient` в `internal/domain/repository/mocks.go` и все ломающиеся
      `*_test.go` обновлены; сборка и тесты зелёные (поведение `handleGetLyrics` пока
      прежнее — правится в T5).
  - Verify: `cd backend && go build ./... && go test ./...` — зелёно.
    `grep -rn "\.Captions(\|Captions(sourceURL" backend/` — старой сигнатуры нет.
  - Files: `backend/internal/domain/repository/track_repo.go`,
    `backend/internal/infrastructure/media/client.go`,
    `backend/internal/usecase/media.go`,
    `backend/internal/domain/repository/mocks.go` (+ затронутые `*_test.go`).

- [ ] **T5 — backend: `handleGetLyrics` берёт `track.MediaID`**
  - Acceptance:
    - Ветка «нет текста» при `track == nil || track.MediaID == ""` (сейчас проверка
      на `track.SourceURL == ""`).
    - Иначе `MediaCaptions(track.MediaID, lang)`; тело ответа
      `/api/rooms/:roomID/lyrics` (`available`, `track_id`, `lang`, `auto`, `cues`)
      неизменно.
  - Verify: `cd backend && go test ./...` — зелёно. `handler_test.go`:
    `handleGetLyrics` зовёт `MediaCaptions` с `track.MediaID`, отдаёт прежнюю форму;
    пустой `MediaID` → `available:false`, исходящего вызова нет.
  - Files: `backend/internal/delivery/http/room_handler.go`,
    `backend/internal/delivery/http/handler_test.go`.

- [ ] **T6 — верификация end-to-end**
  - Acceptance: Success Criteria 1–7 из `SPEC.md` выполнены.
  - Verify:
    - `cd frontend && npm run build && npm run typecheck && npm test` — зелёно
      (караоке-тесты без правок).
    - Локальный стек: комната → добавить YouTube-трек с авто-титрами → 🎤 → строки
      подсвечиваются, дрейф ≤ ~0.3 с на треке без интро.
    - Трек без субтитров → «No lyrics for this track.», ручка 200, логи чистые.
    - После загрузки заглянуть в `media_dir`: есть `media_<hash>.<lang>.vtt`, нет
      мусорных `.part`; исходящих запросов к YouTube на пути показа нет (логи).
  - Files: — (только проверка; при обнаружении дрейфа синхрона — записать как
    известное ограничение в `SPEC.md`, скоуп не расширять).

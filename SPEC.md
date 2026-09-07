# Spec: Возврат режима «Караоке» для скачиваемых треков

## Objective

**Что.** Вернуть синхронный показ текста песни в комнате (`Player.tsx` → кнопка 🎤 →
`Karaoke.tsx`) для текущей архитектуры, где трек скачивается на media-service файлом,
а не проигрывается напрямую с YouTube.

**Зачем.** Вся цепочка караоке в коде цела, но субтитры тянутся отдельным **живым**
запросом `yt-dlp` к YouTube в момент клика (`YouTubeProvider.captions()`). Именно от
таких запросов проект ушёл, перейдя на серверную загрузку аудио: этот путь ловит
HTTP 429 / бот-фильтр YouTube и молча отдаёт `cues: []`. Пользователь видит
«No lyrics for this track» почти всегда.

**Пользователь.** Слушатель в комнате, включивший караоке-панель.

**Success looks like.** Текст едет в такт музыке для большинства YouTube-треков,
без зависимости от YouTube в рантайме показа.

## Tech Stack

- **backend** — Go 1.2x, chi router. `internal/delivery/http/room_handler.go`,
  `internal/usecase/media.go`, `internal/infrastructure/media/client.go`,
  `internal/domain/repository/track_repo.go`.
- **media-service** — Python 3.12, FastAPI. `providers/youtube.py`, `storage.py`,
  `service.py`, `api.py`. Внешний бинарь `yt-dlp`.
- **frontend** — React 18 + Vite + TypeScript. `src/components/Karaoke.tsx`,
  `src/components/Player.tsx`, `src/lib/api.ts`.

## Commands

```
# backend
cd backend && go build ./... && go test ./...

# media-service
cd media-service && python -m pytest

# frontend
cd frontend && npm run build && npm run typecheck && npm test
```

## Project Structure

```
backend/internal/delivery/http/room_handler.go   → handleGetLyrics (HTTP-ручка)
backend/internal/usecase/media.go                → FetchSubtitles → станет MediaCaptions(mediaID)
backend/internal/infrastructure/media/client.go  → Captions() → станет MediaCaptions()
backend/internal/domain/repository/track_repo.go → интерфейс MediaClient
media-service/providers/youtube.py               → download(): +флаги субтитров; _parse_vtt (переиспользуем)
media-service/storage.py                         → ensure/path/is_ready/cleanup/_enforce_size_limit
media-service/service.py                         → captions() из файла
media-service/api.py                             → GET /v1/media/{media_id}/captions (новый), /v1/captions удалить
media-service/tests/test_storage.py, test_api.py, test_provider.py → тесты
frontend/src/components/Karaoke.tsx              → без изменений логики (тот же ответ)
frontend/src/lib/api.ts                          → getLyrics: без изменений контракта
```

## Подход (утверждён)

Субтитры забираются **один раз при скачивании трека** и кладутся `.vtt` рядом с
аудиофайлом; показ читает файл с диска, YouTube в рантайме не трогается.

1. **Загрузка.** `YouTubeProvider.download()` делает **три отдельных прохода**
   `yt-dlp` (уточнено на T6 после живого прогона):
   а) аудио `-f bestaudio/best` — без флагов субтитров, чтобы rate-limit YouTube на
      субтитрах не ронял скачивание аудио (ровно эта боль и была в старом варианте);
   б) ручные субтитры `--skip-download --write-subs` → `media_<hash>.<lang>.vtt`;
   в) авто-субтитры `--skip-download --write-auto-subs -o "subtitle:…​.auto.%(ext)s"`
      → `media_<hash>.auto.<lang>.vtt`.
   `--sub-langs` — **точный список** `en,en-orig,ru,ru-orig` (regex `en.*` дополнительно
   тянет авто-переводы `en-de` и т.п.). Без `--convert-subs`: ffmpeg в образе нет,
   YouTube и так отдаёт vtt. Проходы б/в — best-effort: их провал не влияет на
   результат `download()`. Разделение ручных и авто по имени файла даёт честный
   флаг `auto` без эвристик по содержимому.
2. **Хранение.** `.vtt` живёт в `settings.media_dir`, привязан к аудио по префиксу
   `media_<hash>`. Чистится вместе с аудио (TTL, `_enforce_size_limit`, снятие
   ссылки). См. «Известные подводные камни».
3. **Отдача.** Новый эндпоинт media-service `GET /v1/media/{media_id}/captions?lang=`:
   `storage.captions_path()` находит `.vtt` по `media_id` (приоритет: ручные над
   `.auto.`, затем язык), `_parse_vtt` парсит, ответ
   `{"lang": str, "auto": bool, "cues": [{start, dur, text}]}` — `auto` = наличие
   `.auto.` в имени файла. Нет `.vtt` → `{"lang": "", "auto": false, "cues": []}`,
   статус 200. Чтения с диска, исходящих запросов нет.
4. **Backend.** `handleGetLyrics` вызывает субтитры по `track.MediaID`, а не по
   `track.SourceURL`. `FetchSubtitles(sourceURL, lang)` → `MediaCaptions(mediaID, lang)`
   по всей цепочке (usecase, client, интерфейс `MediaClient`, моки). Форма JSON-ответа
   ручки `/api/rooms/:roomID/lyrics` не меняется.
5. **Frontend.** Не трогаем: `Karaoke.tsx` и `api.getLyrics` работают с тем же
   контрактом. Тайминг-часы остаются `localPos` из `useGaplessPlayer` (`<audio>`).
6. **Фолбэк.** Как сейчас: нет `cues` → `status='empty'` → «No lyrics for this
   track.» Внешние lyrics-API (LRCLIB и т.п.) в этой итерации не подключаем.

## Известные подводные камни (учесть при реализации)

- **`storage.path()` глобит `media_id.*`** и возвращает `matches[0]`. С появлением
  `.vtt` рядом с `.m4a` метод может вернуть субтитры вместо аудио, а `is_ready()`
  (`path().is_file()`) — счесть трек готовым по одному лишь `.vtt`. Разделить:
  `path()`/`is_ready()` фильтруют по аудио-расширениям (`_AUDIO_MIME` уже есть в
  `api.py`), отдельный `captions_path()` ищет `media_<id>.*.vtt` / `media_<id>.vtt`.
- **`storage.cleanup()`** сверяет `path.stem in self._referenced`. У
  `media_<hash>.en.vtt` stem = `media_<hash>.en`, ссылки хранятся как `media_<hash>`
  → файл субтитров сочтётся «висячим» и удалится по TTL раньше аудио. Сверять по
  префиксу до первой точки, а не по полному `stem`.
- **`_enforce_size_limit()`** — та же проблема с `stem`; плюс размер `.vtt` мал,
  но эвиктиться должен вместе с аудио, не раньше.
- **`.part`-файлы субтитров** уже отфильтрованы по `suffix == ".part"` — проверить,
  что промежуточные файлы `yt-dlp` не остаются.
- **Синхронизация.** Таймкоды авто-VTT абсолютны от начала видео; `bestaudio`
  начинается с 0 той же точки — должно совпасть. На T6 проверено: контент и
  таймкоды корректны (`Me at the zoo`, без интро). Проверку на клипе с длинным
  интро оставляем на ручной прогон в UI; при стабильном дрейфе — фиксируем как
  известное ограничение, скоуп не расширяем.
- **Загрузка не должна падать из-за субтитров** (найдено на T6): `--write-subs`
  в одном проходе с аудио → при HTTP 429 на субтитрах yt-dlp прерывает весь
  вызов, аудио не скачивается. Решение — раздельные проходы (см. «Подход» п.1).
- **Ретро-треки.** Уже скачанные до изменения файлы `.vtt` не имеют. Приемлемо:
  докачаются при следующем `ensure` после TTL-эвикта, либо разовый ручной прогрев.
  Отдельную миграцию не делаем.

## Code Style

Совпадать с окружающим кодом. media-service — тип-хинты, `pathlib`, без новых
зависимостей (`yt-dlp` уже есть). Пример новой ручки в стиле `api.py`:

```python
@router.get("/media/{media_id}/captions")
async def media_captions(media_id: str, lang: str = ""):
    if not service.storage.valid_id(media_id):
        raise HTTPException(status_code=400, detail="Invalid media ID")
    return await service.captions_from_disk(media_id, lang)
```

Go — стиль `client.go`: короткие методы, ошибку сети глушим в пустой результат
(как текущий `Captions()`), а не роняем ручку.

## Testing Strategy

- **media-service (`pytest`, `media-service/tests/`).**
  - `test_provider.py`: `download()` формирует аргументы `yt-dlp` с флагами
    субтитров; `_parse_vtt` (уже покрыт — не ломать).
  - `test_storage.py`: `path()`/`is_ready()` не путают `.vtt` с аудио; `cleanup()`
    и `_enforce_size_limit()` не удаляют `.vtt` раньше аудио и удаляют вместе с ним;
    `captions_path()` находит файл по языку и без языка.
  - `test_api.py`: `GET /v1/media/{id}/captions` → 200 с `cues` при наличии `.vtt`;
    200 с пустым списком без файла; 400 на кривой `media_id`.
- **backend (`go test ./...`).**
  - `internal/delivery/http/handler_test.go` / `mocks.go`: `handleGetLyrics` зовёт
    `MediaCaptions(track.MediaID, lang)`, отдаёт прежнюю форму JSON; `track == nil`
    или пустой `MediaID` → `available:false`.
- **frontend (`vitest`, `src/test/Karaoke.test.tsx`).**
  - Существующие тесты караоке зелёные без правок (контракт не изменился).
- **Ручная проверка.** Комната → добавить YouTube-трек с авто-титрами → 🎤 →
  строки подсвечиваются в такт; трек без титров → «No lyrics for this track».

## Boundaries

- **Always:** гонять `go test ./...`, `python -m pytest`, `npm test` до коммита;
  держать неизменным контракт ответа `/api/rooms/:roomID/lyrics` и `api.getLyrics`;
  сетевые/файловые ошибки субтитров глушить в пустой результат, не в 5xx.
- **Ask first:** новые зависимости; изменение схемы БД / формата persist-файлов
  комнат; смена тайминг-источника с `localPos` на room-sync; подключение внешних
  lyrics-API; изменение TTL/лимитов media-storage.
- **Never:** коммитить секреты; удалять падающие тесты ради зелени; оставлять
  живой запрос `yt-dlp` к YouTube на пути показа караоке; трогать mashup-код.

## Success Criteria

1. При добавлении YouTube-трека с (авто)субтитрами файл `media_<hash>.<lang>.vtt`
   появляется в `media_dir` рядом с аудио после `ensure`.
2. `GET /v1/media/{media_id}/captions` возвращает непустой `cues` для такого трека,
   **не делая исходящих запросов к YouTube** (проверяется в тесте / по логам).
3. Панель караоке в комнате показывает текст, активная строка совпадает с музыкой
   в пределах ~0.3 с на треке без видео-интро.
4. Трек без субтитров → панель показывает «No lyrics for this track.», ручка
   отвечает 200, ошибок в логах нет.
5. `.vtt` удаляется вместе со своим аудио при TTL-эвикте и при
   `_enforce_size_limit`; не удаляется, пока аудио в `referenced`.
6. `go test ./...`, `python -m pytest`, `npm test`, `npm run typecheck` — зелёные.
7. Эндпоинт `/v1/captions` и метод `Captions(sourceURL, lang)` удалены; в коде не
   осталось живого пути субтитров через `source_url`.

## Open Questions

1. `--sub-langs`: фиксируем `ru.*,en.*`. Да.
2. Ручная субтитровка vs авто: при наличии обеих сейчас берётся ручная
   (`manual or automatic`). Оставляем это правило? Да.
3. Нужен ли разовый прогрев уже лежащих аудиофайлов, или ждём естественного
   TTL-цикла (спека сейчас закладывает второе)? Ждем цикла.

---
https://claude.ai/code/session_01634cKyQwmWyGXVJSLmuGRX

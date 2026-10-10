import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowLeft, Disc3, Heart, LayoutGrid, ListMusic, MicVocal, Plus, Search } from 'lucide-react'
import { TrackList, TrackRow } from '../../components/media/TrackRows'
import Shelf from '../../components/media/Shelf'
import Hero from '../../components/media/Hero'
import MediaCard from '../../components/media/MediaCard'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import { groupArtists, parseSong, rankSongs, songArtists, totalPlays } from '../lib/artists'
import { apiAvailable, hasAuthToken, BASE } from '../lib/api'
import { loadSong } from '../lib/songs'
import { formatTime } from '../lib/songs'
import { hasLocalLyrics, loadManifest, plural } from '../lib/songs'
import { useKaraoke } from '../store'
import { usePlayer } from '../../context/PlayerContext'
import { useAuth } from '../../context/AuthContext'
import type { SongMeta } from '../lib/types'
import type { KaraokePreview } from '../../lib/karaokePreview'
import Upload from './Upload'

type Tab = 'all' | 'fav'

function hueOf(s: string): number {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360
  return h
}

const HERO_CAT = (
  <video src="/karaoke-cat.mp4" width={498} height={498} autoPlay muted loop playsInline preload="metadata" aria-label="Поющий кот" />
);

function SongCard({ song, index, activeArtist }: { song: SongMeta; index: number; activeArtist?: string }) {
  const openSong = useKaraoke((s) => s.openSong)
  const setLoadingSong = useKaraoke((s) => s.setLoadingSong)
  const loadingSong = useKaraoke((s) => s.loadingSong)
  const favorites = useKaraoke((s) => s.favorites)
  const toggleFavorite = useKaraoke((s) => s.toggleFavorite)
  const pushRecent = useKaraoke((s) => s.pushRecent)
  const { setPreviewSong, previewSong } = usePlayer()
  const [busy, setBusy] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const hue = hueOf(song.id)
  const fav = favorites.includes(song.id)
  const edited = hasLocalLyrics(song.id)
  const parsed = parseSong(song.title)
  const artists = songArtists(song)
  const others = activeArtist
    ? artists.filter((a) => a.toLowerCase() !== activeArtist.toLowerCase())
    : artists

  // Превью как на главной: клик по треку открывает панель, петь — только «Петь».
  const preview = () => {
    if (previewSong?.id === song.id) setPreviewSong(null)
    else {
      setPreviewSong({
        id: song.id,
        title: parsed.title,
        language: song.language,
        duration: song.duration,
        audio: song.audio,
        original: song.original ?? null,
        owner_id: song.owner_id ?? null,
        owner_name: song.owner_name ?? null,
      })
    }
  }

  const sing = async () => {
    if (busy || loadingSong) return
    setBusy(true)
    setLoadingSong(true)
    setLoadError(null)
    setPreviewSong(null)
    try {
      const data = await loadSong(song)
      pushRecent(song.id)
      openSong(data)
    } catch (e) {
      console.error(e)
      setLoadError('Не открылась — проверьте файлы песни')
    } finally {
      setBusy(false)
      setLoadingSong(false)
    }
  }

  return (
    <TrackRow
      index={index}
      title={parsed.title}
      meta={
        <>
          {others.length > 0 && <span>{others.join(', ')} · </span>}
          {song.lines} строк
          {song.language && <span> · {song.language.toUpperCase()}</span>}
        </>
      }
      badge={
        edited ? (
          <span className="shrink-0 rounded-full border border-border px-2 py-0.5 text-[11px] font-semibold text-muted">
            свой текст
          </span>
        ) : undefined
      }
      error={loadError}
      number={index + 1}
      duration={formatTime(song.duration)}
      tint={`linear-gradient(135deg, hsl(${hue} 45% 32%), hsl(${(hue + 40) % 360} 50% 20%))`}
      glyph={<Disc3 className="h-5 w-5 text-white/70" />}
      busy={busy}
      onPlay={preview}
      playLabel={`Превью: ${parsed.title}`}
      actions={
        <>
          <button onClick={() => toggleFavorite(song.id)} title={fav ? 'Убрать из избранного' : 'В избранное'}
            aria-label={fav ? 'Убрать из избранного' : 'В избранное'}
            className={`grid h-9 w-9 shrink-0 place-items-center rounded-full transition ${fav ? 'text-danger' : 'text-muted/70 hover:bg-surface-hover hover:text-text'}`}>
            <Heart className={`h-4 w-4 ${fav ? 'fill-current' : ''}`} />
          </button>
          <button onClick={sing}
            className="flex shrink-0 items-center gap-1.5 rounded-full bg-surface-hover px-3.5 py-2 text-[13px] font-medium text-text transition group-hover:bg-primary group-hover:text-white">
            <MicVocal className="h-3.5 w-3.5" />
            Петь
          </button>
        </>
      }
    />
  )
}

const TABS: { id: Tab; label: string; icon: typeof LayoutGrid }[] = [
  { id: 'all', label: 'Все песни', icon: LayoutGrid },
  { id: 'fav', label: 'Избранное', icon: Heart },
]

export default function Catalog() {
  const songs = useKaraoke((s) => s.songs)
  const setSongs = useKaraoke((s) => s.setSongs)
  const openSong = useKaraoke((s) => s.openSong)
  const favorites = useKaraoke((s) => s.favorites)
  const recent = useKaraoke((s) => s.recent)
  const plays = useKaraoke((s) => s.plays)
  const pushRecent = useKaraoke((s) => s.pushRecent)
  const [q, setQ] = useState('')
  const [tab, setTab] = useState<Tab>('all')
  const [sort, setSort] = useState<'title' | 'dur-asc' | 'dur-desc'>('title')
  const [lang, setLang] = useState('ALL')
  const [artist, setArtist] = useState<string | null>(null)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [apiUp, setApiUp] = useState(false)
  const [refreshError, setRefreshError] = useState<string | null>(null)
  const setLoadingSong = useKaraoke((s) => s.setLoadingSong)
  const editRequest = useKaraoke((s) => s.editRequest)

  // загрузка — только для вошедших: бэкенд жив + токен (или standalone без входа)
  const canUpload = apiUp && hasAuthToken()

  useEffect(() => {
    apiAvailable().then(setApiUp).catch(() => setApiUp(false))
  }, [])

  // Заявка «Редактировать караоке» из превью: грузим песню, открываем плеер и редактор тайминга.
  useEffect(() => {
    if (!editRequest) return
    if (songs.length === 0) return
    const id = editRequest
    const meta = songs.find((s) => s.id === id)
    if (!meta) {
      useKaraoke.getState().consumeEditRequest()
      return
    }
    let cancelled = false
    void (async () => {
      const st = useKaraoke.getState()
      st.setLoadingSong(true)
      setRefreshError(null)
      try {
        const data = await loadSong(meta)
        if (cancelled) return
        const cur = useKaraoke.getState()
        cur.pushRecent(meta.id)
        cur.openSong(data)
        cur.setEditorOpen(true)
      } catch (e) {
        console.error(e)
        if (!cancelled) setRefreshError('Не открылась — проверьте файлы песни')
      } finally {
        const cur = useKaraoke.getState()
        cur.consumeEditRequest()
        if (!cancelled) cur.setLoadingSong(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [editRequest, songs])

  // Недавние в порядке проигрывания (сначала — последняя спетая).
  const recentOrdered = useMemo(() => {
    const order = new Map(recent.map((id, i) => [id, i]))
    return songs
      .filter((s) => order.has(s.id))
      .sort((a, b) => (order.get(a.id) ?? 0) - (order.get(b.id) ?? 0))
  }, [songs, recent])

  const byTab = useMemo(() => {
    if (tab === 'fav') {
      const favIds = new Set(favorites)
      return songs.filter((s) => favIds.has(s.id))
    }
    return songs
  }, [songs, tab, favorites])

  // Языки каталога для чипсов-фильтров.
  const langs = useMemo(() => {
    const set = new Set<string>()
    songs.forEach((s) => {
      if (s.language) set.add(s.language.toUpperCase())
    })
    return [...set].sort()
  }, [songs])

  const filtered = useMemo(
    () =>
      byTab.filter(
        (s) =>
          s.title.toLowerCase().includes(q.trim().toLowerCase()) &&
          (lang === 'ALL' || (s.language || '').toUpperCase() === lang),
      ),
    [byTab, q, lang],
  )

  const sorted = useMemo(() => {
    if (sort === 'dur-asc') return [...filtered].sort((a, b) => a.duration - b.duration)
    if (sort === 'dur-desc') return [...filtered].sort((a, b) => b.duration - a.duration)
    return filtered
  }, [filtered, sort])

  const refreshAndOpen = async (songId: string | null) => {
    setUploadOpen(false)
    setRefreshError(null)
    if (!songId) {
      setRefreshError('Сервер не вернул песню — выберите её в каталоге')
      return
    }
    setLoadingSong(true)
    try {
      const all = await loadManifest(500)
      setSongs(all)
      const meta = all.find((s) => s.id === songId)
      if (!meta) {
        setRefreshError('Песни нет в каталоге — обновите страницу')
        return
      }
      pushRecent(meta.id)
      openSong(await loadSong(meta))
    } catch (e) {
      console.error(e)
      setRefreshError('Не удалось открыть песню')
    } finally {
      setLoadingSong(false)
    }
  }

  const { setPreviewSong: setPreview, previewSong: currentPreview } = usePlayer()
  const { user } = useAuth()

  // Превью как на главной: клик по карточке открывает панель, петь — только «Петь».
  const togglePreview = (s: SongMeta) => {
    if (currentPreview?.id === s.id) {
      setPreview(null)
      return
    }
    const parsed = parseSong(s.title)
    const preview: KaraokePreview = {
      id: s.id,
      title: parsed.title,
      language: s.language,
      duration: s.duration,
      audio: s.audio,
      original: s.original ?? null,
      owner_id: s.owner_id ?? null,
      owner_name: s.owner_name ?? null,
    }
    setPreview(preview)
  }

  const artists = useMemo(() => groupArtists(songs), [songs])

  const popular = useMemo(() => rankSongs(songs, plays, favorites, 8), [songs, plays, favorites])

  const artistEntry = artist ? (artists.find((a) => a.name.toLowerCase() === artist) ?? null) : null

  const artistTop = useMemo(
    () => (artistEntry ? rankSongs(artistEntry.songs, plays, favorites, 5) : []),
    [artistEntry, plays, favorites],
  )

  const artistPlays = artistEntry ? totalPlays(artistEntry.songs, plays) : 0

  const tabCounts: Record<Tab, number> = {
    all: songs.length,
    fav: favorites.length,
  }

  // Полки всегда на месте; поиск, табы и фильтры режут только список ниже.

  return (
    <div className="w-full px-10 pb-16 pt-4 max-sm:px-3.5">
      {hasAuthToken() ? (
        <Hero
          title="Загрузи свою песню"
          sub="Загрузи песню, отредактируй текст и пой."
          art={HERO_CAT}
          actions={
              canUpload ? (
                <button onClick={() => setUploadOpen(true)} title="Загрузить свою песню (аудио или клип)"
                  className="btn shrink-0 gap-1.5">
                  <Plus className="h-4 w-4" />
                  <span className="hidden sm:inline">Загрузить</span>
                </button>
              ) : undefined
            }
          />
        ) : (
          <Hero
            title="Регистрируйся и загружай своё"
            sub="Петь можно без входа, а загрузка песен — для своих."
            art={HERO_CAT}
            actions={
              BASE ? (
                <>
                  <Link className="btn btn-sm" to="/register">
                    Регистрация
                  </Link>
                  <Link className="btn btn-secondary btn-sm" to="/login">
                    Войти
                  </Link>
                </>
              ) : undefined
            }
          />
        )}

      {artistEntry ? (
        <div className="pt-4">
          <button
            onClick={() => setArtist(null)}
            className="flex items-center gap-1.5 rounded-full bg-surface-hover px-3.5 py-2 text-[13px] font-medium text-muted transition hover:text-text"
          >
            <ArrowLeft className="h-4 w-4" />
            Каталог
          </button>
          <h2 className="mt-3 text-2xl font-extrabold tracking-tight text-text">{artistEntry.name}</h2>
          <p className="mt-1 text-[13px] text-muted">
            {artistEntry.songs.length} {plural(artistEntry.songs.length, 'песня', 'песни', 'песен')}
            {artistPlays > 0 && (
              <> · {artistPlays} {plural(artistPlays, 'исполнение', 'исполнения', 'исполнений')}</>
            )}
          </p>
          <h3 className="mb-2 mt-5 text-sm font-semibold text-muted">Популярное</h3>
          <TrackList>
            {artistTop.map((s, i) => (
              <SongCard key={s.id} song={s} index={i} activeArtist={artistEntry.name} />
            ))}
          </TrackList>
          <h3 className="mb-2 mt-5 text-sm font-semibold text-muted">Все песни</h3>
          <TrackList>
            {artistEntry.songs.map((s, i) => (
              <SongCard key={s.id} song={s} index={i} activeArtist={artistEntry.name} />
            ))}
          </TrackList>
        </div>
      ) : (
        <>
          {songs.length > 0 && (
              <Shelf title="Популярное">
                {popular.map((s) => {
                  const parsed = parseSong(s.title)
                  return (
                    <MediaCard
                      asButton
                      key={s.id}
                      title={parsed.title}
                      meta={
                        <>
                          {s.lines} строк · {formatTime(s.duration)}
                        </>
                      }
                      tint={tintForId(s.id)}
                      glyph={monoGlyph(parsed.title)}
                      playLabel={`Превью: ${parsed.title}`}
                      onToggle={() => togglePreview(s)}
                    />
                  )
                })}
              </Shelf>
          )}
          {recentOrdered.length > 0 && (
              <Shelf title="Недавние">
                {recentOrdered.slice(0, 8).map((s) => {
                  const parsed = parseSong(s.title)
                  return (
                    <MediaCard
                      asButton
                      key={s.id}
                      title={parsed.title}
                      meta={
                        <>
                          {s.lines} строк · {formatTime(s.duration)}
                        </>
                      }
                      tint={tintForId(s.id)}
                      glyph={monoGlyph(parsed.title)}
                      playLabel={`Превью: ${parsed.title}`}
                      onToggle={() => togglePreview(s)}
                    />
                  )
                })}
              </Shelf>
          )}
          {artists.length > 0 && (
              <Shelf title="Исполнители">
                {artists.map((a) => (
                  <MediaCard
                    asButton
                    key={a.name.toLowerCase()}
                    title={a.name}
                    meta={
                      <>{a.songs.length} {plural(a.songs.length, 'песня', 'песни', 'песен')}</>
                    }
                    tint={tintForId(a.name.toLowerCase())}
                    glyph={a.name.charAt(0).toUpperCase()}
                    shape="round"
                    showFab={false}
                    playLabel={a.name}
                    onToggle={() => setArtist(a.name.toLowerCase())}
                  />
                ))}
              </Shelf>
          )}
          <h2 className="mt-5 text-xl font-extrabold tracking-tight text-text">
            Все песни
            {songs.length > 0 && (
              <span className="ml-1 align-middle text-xs font-medium tracking-normal text-muted">
                · {sorted.length} {plural(sorted.length, 'песня', 'песни', 'песен')}
              </span>
            )}
          </h2>

      <div className="mt-3">
        <div className="relative">
          <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted/70" />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Найти песню…"
            aria-label="Найти песню"
            className="w-full rounded-2xl border border-border bg-surface py-3 pl-11 pr-4 text-[15px] text-text placeholder:text-muted/70 outline-none focus:border-primary"
          />
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-1.5" role="tablist" aria-label="Разделы каталога">
        {TABS.map((t) => (
          <button key={t.id} role="tab" aria-selected={tab === t.id} onClick={() => setTab(t.id)}            className={`flex items-center gap-1.5 rounded-full px-4 py-2 text-[13px] font-medium transition ${tab === t.id ? 'bg-primary text-white' : 'bg-surface-hover text-muted hover:bg-border hover:text-text'}`}>
            <t.icon className="h-3.5 w-3.5" />
            {t.label}
            <span className={tab === t.id ? 'text-white/70' : 'text-muted/60'}>{tabCounts[t.id]}</span>
          </button>
        ))}
        <div className="ml-auto flex shrink-0 items-center gap-1.5">
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as typeof sort)}
            aria-label="Сортировка"
            className="h-9 cursor-pointer rounded-full border border-border bg-surface-hover px-3 text-[13px] font-medium text-muted outline-none transition hover:text-text focus:border-primary"
          >
            <option value="title">Сначала A–Я</option>
            <option value="dur-asc">Сначала короткие</option>
            <option value="dur-desc">Сначала длинные</option>
          </select>
        </div>
      </div>

      {langs.length > 1 && (
        <div className="mt-2 flex flex-wrap gap-1.5" aria-label="Фильтр по языку">
          {['ALL', ...langs].map((l) => (
            <button
              key={l}
              onClick={() => setLang(l)}
              aria-pressed={lang === l}
              className={`rounded-full px-3 py-1 text-xs font-medium transition ${lang === l ? 'bg-primary text-white' : 'bg-surface-hover text-muted hover:text-text'}`}
            >
              {l === 'ALL' ? 'Все' : l}
            </button>
          ))}
        </div>
      )}

      {refreshError && (
        <p className="mt-2 rounded-xl border border-danger/20 bg-danger/10 px-3 py-2 text-[13px] text-danger">
          {refreshError}
        </p>
      )}

      {songs.length === 0 ? (
        <div className="mt-16 flex flex-col items-center text-center">
          <div className="grid h-16 w-16 place-items-center rounded-2xl bg-surface-hover">
            <ListMusic className="h-7 w-7 text-muted" />
          </div>
          <h2 className="mt-5 text-lg font-medium text-text">Пока нет песен</h2>
          <p className="mt-2 max-w-md text-sm leading-relaxed text-muted">
            {canUpload
              ? 'Нажмите «Загрузить» и добавьте первую песню — вокал отделится сам.'
              : apiUp
                ? 'Загрузка песен — только для вошедших: войдите в bebradio, и кнопка появится.'
                : 'Запустите бэкенд и загрузите песню через интерфейс, либо соберите пайплайном:'}
          </p>
          <ul className="mt-4 max-w-md space-y-1 text-left text-[13px] leading-relaxed text-muted">
            <li><b className="text-text">1.</b> Выберите песню и нажмите «Петь» — будет обратный отсчёт и минус.</li>
            <li><b className="text-text">2.</b> Пойте в микрофон — в конце получите оценку попадания в ноты.</li>
            <li><b className="text-text">3.</b> Текст правится карандашом «Текст» в плеере.</li>
          </ul>
          {!apiUp && (
            <code className="mt-4 rounded-xl bg-surface-hover px-4 py-3 text-left text-[12.5px] leading-relaxed text-muted">
              python -m uvicorn karaoke_api.app:app --port 8000
              <br />
              # дальше — кнопка «Загрузить» в интерфейсе
            </code>
          )}
        </div>
      ) : sorted.length === 0 ? (
        <div className="mt-10 flex flex-col items-center text-center">
          <p className="text-sm text-muted">
            {tab === 'fav' && !q ? 'Нажмите сердечко на песне — она появится здесь' : 'Ничего не найдено'}
          </p>
          {tab === 'fav' && !q ? (
            <button onClick={() => setTab('all')} className="btn btn-secondary btn-sm mt-3">
              Ко всем песням
            </button>
          ) : q ? (
            <button onClick={() => setQ('')} className="btn btn-secondary btn-sm mt-3">
              Сбросить поиск
            </button>
          ) : null}
        </div>
      ) : (
        <div className="mt-4">
          <TrackList>
            {sorted.map((s, i) => (
              <SongCard key={s.id} song={s} index={i} />
            ))}
          </TrackList>
        </div>
      )}
        </>
      )}
      {uploadOpen && (
        <Upload onClose={() => setUploadOpen(false)} onDone={(id) => void refreshAndOpen(id)} ownerName={user?.username} />
      )}
    </div>
  )
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/ToastContext'
import { api } from '../lib/api'
import type { Track } from '../types'
import { usePlayer } from '../context/PlayerContext'
import MashupCard from '../components/mashup/MashupCard'
import Shelf from '../components/media/Shelf'
import Hero from '../components/media/Hero'
import { TrackList, TrackRow } from '../components/media/TrackRows'
import { HeartFillIcon, HeartIcon } from '../components/player/icons'
import { formatTime } from '../lib/format'
import { monoGlyph, tintForId } from '../lib/mashupArt'
import UploadMashupModal from '../components/mashup/UploadMashupModal'
import EditMashupModal from '../components/mashup/EditMashupModal'
import styles from './Mashups.module.css'

const RECENT_LIMIT = 12
const TOP_PAGE = 24

export default function Mashups() {
  const { user } = useAuth()
  const { showToast } = useToast()
  const {
    player,
    registerTracks,
    toggleTrack,
    toggleLike: ctxToggleLike,
    applyLike,
    setProfileUserId,
  } = usePlayer()

  useEffect(() => {
    const previousTitle = document.title
    const description = document.querySelector<HTMLMetaElement>('meta[name="description"]')
    const canonical = document.querySelector<HTMLLinkElement>('link[rel="canonical"]')
    const previousDescription = description?.content
    const previousCanonical = canonical?.href

    document.title = 'Загружайте и слушайте мешапы — bebradio'
    if (description) {
      description.content = 'Слушайте лучшие мешапы онлайн на bebradio. Находите новые треки, добавляйте их в очередь и делитесь музыкой с друзьями.'
    }
    if (canonical) canonical.href = 'https://bebradio.ru/mashup'

    return () => {
      document.title = previousTitle
      if (description && previousDescription !== undefined) description.content = previousDescription
      if (canonical && previousCanonical !== undefined) canonical.href = previousCanonical
    }
  }, [])

  const [showUpload, setShowUpload] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [q, setQ] = useState('')
  const [sort, setSort] = useState<'recent' | 'top' | 'title'>('recent')

  const [recent, setRecent] = useState<Track[]>([])
  const [recentLoading, setRecentLoading] = useState(true)

  const [top, setTop] = useState<Track[]>([])
  const [topLoading, setTopLoading] = useState(true)
  const [topDone, setTopDone] = useState(false)

  const [liked, setLiked] = useState<Track[]>([])
  const [likedLoading, setLikedLoading] = useState(false)

  const [mine, setMine] = useState<Track[]>([])
  const [mineLoading, setMineLoading] = useState(false)

  // Apply an update to a single mashup across every list it may appear in.
  const patchAll = useCallback((id: string, updater: (m: Track) => Track) => {
    const apply = (arr: Track[]) => arr.map((m) => (m.id === id ? updater(m) : m))
    setRecent(apply)
    setTop(apply)
    setLiked(apply)
    setMine(apply)
  }, [])

  const removeEverywhere = useCallback((id: string) => {
    const drop = (arr: Track[]) => arr.filter((m) => m.id !== id)
    setRecent(drop)
    setTop(drop)
    setLiked(drop)
    setMine(drop)
  }, [])

  // ── Section loads ─────────────────────────────────────────────────
  const loadRecent = useCallback(async () => {
    setRecentLoading(true)
    try {
      setRecent(await api.listTracks('', 'recent', RECENT_LIMIT))
    } catch {
      showToast('Could not load mashups', 'error')
    } finally {
      setRecentLoading(false)
    }
  }, [showToast])

  const topRef = useRef<Track[]>([])
  topRef.current = top
  const topBusyRef = useRef(false)

  const loadTopPage = useCallback(
    async (reset: boolean) => {
      if (topBusyRef.current) return
      topBusyRef.current = true
      setTopLoading(true)
      const offset = reset ? 0 : topRef.current.length
      try {
        const page = await api.listTracks('', 'top', TOP_PAGE, offset)
        setTop((prev) => {
          const base = reset ? [] : prev
          const seen = new Set(base.map((m) => m.id))
          return [...base, ...page.filter((m) => !seen.has(m.id))]
        })
        setTopDone(page.length < TOP_PAGE)
      } catch {
        showToast('Could not load mashups', 'error')
      } finally {
        setTopLoading(false)
        topBusyRef.current = false
      }
    },
    [showToast],
  )

  const loadLiked = useCallback(async () => {
    setLikedLoading(true)
    try {
      setLiked(await api.likedTracks())
    } catch {
      showToast('Could not load liked mashups', 'error')
    } finally {
      setLikedLoading(false)
    }
  }, [showToast])

  const loadMine = useCallback(async () => {
    setMineLoading(true)
    try {
      setMine(await api.myTracks())
    } catch {
      showToast('Could not load your mashups', 'error')
    } finally {
      setMineLoading(false)
    }
  }, [showToast])

  useEffect(() => {
    loadRecent()
    setTop([])
    setTopDone(false)
    loadTopPage(true)
  }, [loadRecent, loadTopPage])

  useEffect(() => {
    if (user) {
      loadLiked()
      loadMine()
    } else {
      setLiked([])
      setMine([])
    }
  }, [user, loadLiked, loadMine])

  // Infinite scroll for the "Top by likes" shelf. The observer's root is that
  // rail's own horizontal scroll box.
  const sentinelRef = useRef<HTMLDivElement>(null)
  const topRailRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (topDone) return
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadTopPage(false)
      },
      { root: topRailRef.current },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [topDone, loadTopPage, top.length])

  // The player's walkable list is every mashup currently loaded, in a stable
  // order. Search results are appended rather than swapped in, so starting a
  // search never drops the playing track out of the list (which would stop it).
  const playerList = useMemo(() => {
    const seen = new Set<string>()
    return [...recent, ...top, ...liked, ...mine].filter((m) =>
      seen.has(m.id) ? false : seen.add(m.id),
    )
  }, [recent, top, liked, mine])

  // Секция «Все мешапы»: поиск и сортировка режут только список ниже.
  const allFiltered = useMemo(() => {
    const query = q.trim().toLowerCase()
    const rows = query
      ? playerList.filter(
          (m) =>
            m.title.toLowerCase().includes(query) ||
            (m.artist || '').toLowerCase().includes(query),
        )
      : [...playerList]
    if (sort === 'top') rows.sort((a, b) => b.likes - a.likes)
    else if (sort === 'title') rows.sort((a, b) => a.title.localeCompare(b.title, 'ru'))
    return rows
  }, [playerList, q, sort])

  useEffect(() => {
    registerTracks(playerList)
  }, [playerList, registerTracks])

  // The mashup open in the settings dialog, resolved live so cover changes made
  // inside it show up without a reopen.
  const editing = editingId ? playerList.find((m) => m.id === editingId) ?? null : null

  // Poll any still-processing mashups every 2s until they settle.
  const processingIds = useMemo(
    () => [...new Set(playerList.filter((m) => m.status === 'processing').map((m) => m.id))],
    [playerList],
  )
  const processingKey = processingIds.join(',')
  const pollRef = useRef(processingIds)
  pollRef.current = processingIds

  useEffect(() => {
    if (pollRef.current.length === 0) return
    const timer = setInterval(async () => {
      const ids = pollRef.current
      if (ids.length === 0) return
      const updated = await Promise.all(ids.map((id) => api.getTrack(id).catch(() => null)))
      updated.forEach((fresh) => {
        if (fresh) patchAll(fresh.id, () => fresh)
      })
    }, 2000)
    return () => clearInterval(timer)
  }, [processingKey, patchAll])

  // ── Actions ──────────────────────────────────────────────────────
  const handleDelete = async (m: Track) => {
    try {
      await api.deleteTrack(m.id)
      removeEverywhere(m.id)
      showToast('Track deleted', 'success')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Delete failed', 'error')
    }
  }

  const handleUploaded = (m: Track) => {
    setRecent((prev) => [m, ...prev.filter((x) => x.id !== m.id)])
    setMine((prev) => [m, ...prev.filter((x) => x.id !== m.id)])
  }

  const handleToggleLike = async (m: Track) => {
    if (!user) {
      showToast('Sign in to like mashups', 'error')
      return
    }
    const view = applyLike(m)
    const nextLiked = !view.liked
    const optimistic = (x: Track): Track => ({
      ...x,
      liked: nextLiked,
      likes: Math.max(0, x.likes + (nextLiked ? 1 : -1)),
    })
    patchAll(m.id, optimistic)
    setLiked((prev) => {
      if (nextLiked) {
        return prev.some((x) => x.id === m.id) ? prev.map((x) => (x.id === m.id ? optimistic(x) : x)) : [optimistic(m), ...prev]
      }
      return prev.filter((x) => x.id !== m.id)
    })
    const res = await ctxToggleLike(m)
    if (res) {
      const settle = (x: Track): Track => ({ ...x, liked: res.liked, likes: res.likes })
      patchAll(m.id, settle)
      setLiked((prev) => prev.map((x) => (x.id === m.id ? settle(x) : x)))
    } else {
      const revert = (x: Track): Track => ({ ...x, liked: view.liked, likes: view.likes })
      patchAll(m.id, revert)
      setLiked((prev) =>
        view.liked
          ? prev.some((x) => x.id === m.id)
            ? prev
            : [{ ...view }, ...prev]
          : prev.filter((x) => x.id !== m.id),
      )
    }
  }

  const handleChangeCover = async (m: Track, file: File) => {
    try {
      await api.uploadTrackCover(m.id, file)
      const fresh = await api.getTrack(m.id)
      patchAll(m.id, () => fresh)
      showToast('Cover updated', 'success')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Could not upload cover', 'error')
    }
  }

  const handleSaveMeta = async (m: Track, title: string, artist: string) => {
    try {
      await api.updateTrack(m.id, { title, artist })
      patchAll(m.id, (x) => ({ ...x, title, artist }))
      showToast('Metadata updated', 'success')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Could not update metadata', 'error')
    }
  }

  const activeId = player.current?.id
  const isAdmin = !!user && user.role === 'admin'

  const renderCard = (m: Track) => {
    const view = applyLike(m)
    return (
      <MashupCard
        key={m.id}
        mashup={view}
        active={m.id === activeId}
        isPlaying={player.isPlaying}
        canEdit={!!user && (user.id === m.owner_id || isAdmin)}
        onPlay={() => toggleTrack(view)}
        onToggleLike={() => handleToggleLike(view)}
        onEdit={() => setEditingId(m.id)}
        onOpenProfile={setProfileUserId}
      />
    )
  }

  const renderRow = (m: Track, index: number) => {
    const view = applyLike(m)
    const active = m.id === activeId
    const playing = active && player.isPlaying
    const ready = m.status === 'ready'
    return (
      <TrackRow
        key={m.id}
        index={index}
        title={view.title}
        meta={<>{view.artist || 'Unknown artist'}</>}
        badge={
          view.status === 'processing' ? (
            <span className={styles.rowBadgeWarn}>processing…</span>
          ) : view.status === 'failed' ? (
            <span className={styles.rowBadgeError} title={view.error || 'failed'}>
              failed
            </span>
          ) : undefined
        }
        duration={ready ? formatTime(view.duration) : undefined}
        artUrl={view.thumbnail || undefined}
        tint={tintForId(view.id)}
        glyph={monoGlyph(view.title)}
        onPlay={() => toggleTrack(view)}
        playLabel={playing ? `Pause ${view.title}` : `Play ${view.title}`}
        disabled={!ready}
        disabledLabel={`${view.title} is not ready`}
        actions={
          <>
            <button
              type="button"
              className={`${styles.rowLike} ${view.liked ? styles.rowLikeOn : ''}`}
              onClick={() => handleToggleLike(view)}
              aria-pressed={!!view.liked}
              aria-label={view.liked ? `Unlike ${view.title}` : `Like ${view.title}`}
            >
              {view.liked ? <HeartFillIcon size={15} /> : <HeartIcon size={15} />}
              <span>{view.likes}</span>
            </button>
            {(!!user && (user.id === m.owner_id || isAdmin)) && (
              <button
                type="button"
                className={styles.rowEdit}
                onClick={() => setEditingId(m.id)}
                aria-label={`Edit ${view.title}`}
              >
                Edit
              </button>
            )}
          </>
        }
      />
    )
  }

  const renderShelf = (
    title: string,
    items: Track[],
    loading: boolean,
    opts: { emptyText: string; rail?: boolean },
  ) => (
    <Shelf
      title={title}
      loading={loading}
      skeletonCount={6}
      empty={items.length === 0 ? <p className={styles.empty}>{opts.emptyText}</p> : undefined}
      trackRef={opts.rail ? topRailRef : undefined}
      sentinel={opts.rail && !topDone ? <div ref={sentinelRef} className={styles.railSentinel} /> : undefined}
    >
      {items.map(renderCard)}
    </Shelf>
  )

  return (
    <div className={styles.wrap}>
      <div className={`${styles.root} ${player.current ? styles.barClear : ''}`}>
        <div className={styles.page}>
      {user ? (
        <Hero
          title="Загрузи свой мешап"
          sub="Трек сразу попадёт в каталог — его можно ставить в очередь в комнатах."
          actions={
            <button className="btn" onClick={() => setShowUpload(true)}>
              Upload
            </button>
          }
        />
      ) : (
        <Hero
          title="Регистрируйся и загружай своё"
          sub="Слушать можно без входа — аккаунт нужен только для загрузки."
          actions={
            <>
              <Link className="btn btn-sm" to="/register">
                Регистрация
              </Link>
              <Link className="btn btn-secondary btn-sm" to="/login">
                Войти
              </Link>
            </>
          }
        />
      )}

      {renderShelf('Latest', recent, recentLoading, {
        emptyText: user ? 'No mashups yet — hit Upload.' : 'No mashups yet.',
      })}
      {renderShelf('Top by likes', top, topLoading && top.length === 0, {
        emptyText: 'No mashups yet.',
        rail: true,
      })}
      {user &&
        renderShelf('Liked', liked, likedLoading, {
          emptyText: 'You haven\u2019t liked any mashups yet.',
        })}
      {user &&
        renderShelf('My mashups', mine, mineLoading, {
          emptyText: 'You have not uploaded any mashups yet.',
        })}

      <section>
        <div className={styles.allHead}>
          <h2 className={styles.allTitle}>
            Все мешапы
            <span className={styles.allCount}>
              · {allFiltered.length}
            </span>
          </h2>
          <div className={styles.allTools}>
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Найти мешап…"
              aria-label="Найти мешап"
              className={styles.allSearch}
            />
            <select
              value={sort}
              onChange={(e) => setSort(e.target.value as typeof sort)}
              aria-label="Сортировка"
              className={styles.allSort}
            >
              <option value="recent">Сначала новые</option>
              <option value="top">По лайкам</option>
              <option value="title">A–Я</option>
            </select>
          </div>
        </div>
        {allFiltered.length === 0 ? (
          <div className={styles.allEmpty}>
            <p className={styles.empty}>Ничего не найдено</p>
            {q && (
              <button onClick={() => setQ('')} className="btn btn-secondary btn-sm">
                Сбросить поиск
              </button>
            )}
          </div>
        ) : (
          <TrackList>
            {allFiltered.map((m, i) => renderRow(m, i + 1))}
          </TrackList>
        )}
      </section>

      {showUpload && (
        <UploadMashupModal
          onClose={() => setShowUpload(false)}
          onUploaded={handleUploaded}
        />
      )}

      {editing && (
        <EditMashupModal
          mashup={editing}
          canManageCover={!!user && (user.id === editing.owner_id || isAdmin)}
          canEditMeta={!!user && user.id === editing.owner_id}
          canDelete={!!user && (user.id === editing.owner_id || isAdmin)}
          onSaveMeta={(title, artist) => handleSaveMeta(editing, title, artist)}
          onChangeCover={(file) => handleChangeCover(editing, file)}
          onDelete={() => handleDelete(editing)}
          onClose={() => setEditingId(null)}
        />
      )}

        </div>
      </div>
    </div>
  )
}


import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/ToastContext'
import { api } from '../lib/api'
import type { Track } from '../types'
import { useMashupPlayer } from '../hooks/useMashupPlayer'
import MashupCard from '../components/mashup/MashupCard'
import MashupPlayer from '../components/mashup/MashupPlayer'
import MashupSidebar from '../components/mashup/MashupSidebar'
import NowPlayingPanel from '../components/mashup/NowPlayingPanel'
import NowPlayingModal from '../components/mashup/NowPlayingModal'
import UploadMashupModal from '../components/mashup/UploadMashupModal'
import EditMashupModal from '../components/mashup/EditMashupModal'
import ProfileModal from '../components/ProfileModal'
import styles from './Mashups.module.css'

const RECENT_LIMIT = 12
const TOP_PAGE = 24

export default function Mashups() {
  const { user } = useAuth()
  const { showToast } = useToast()
  const player = useMashupPlayer()
  const { setList } = player

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
  const [profileUserId, setProfileUserId] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)
  const [queueOpen, setQueueOpen] = useState(false)

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

  useEffect(() => {
    setList(playerList)
  }, [playerList, setList])

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
    const nextLiked = !m.liked
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
    try {
      const res = nextLiked ? await api.likeTrack(m.id) : await api.unlikeTrack(m.id)
      const settle = (x: Track): Track => ({ ...x, liked: res.liked, likes: res.likes })
      patchAll(m.id, settle)
      setLiked((prev) => prev.map((x) => (x.id === m.id ? settle(x) : x)))
    } catch (err) {
      const revert = (x: Track): Track => ({ ...x, liked: m.liked, likes: m.likes })
      patchAll(m.id, revert)
      setLiked((prev) =>
        m.liked
          ? prev.some((x) => x.id === m.id)
            ? prev
            : [{ ...m }, ...prev]
          : prev.filter((x) => x.id !== m.id),
      )
      showToast(err instanceof Error ? err.message : 'Could not update like', 'error')
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

  const activeId = player.current?.id

  const renderCard = (m: Track) => (
    <div key={m.id} className={styles.railItem}>
      <MashupCard
        mashup={m}
        active={m.id === activeId}
        isPlaying={player.isPlaying}
        canEdit={!!user && user.id === m.owner_id}
        onPlay={() => (m.id === activeId ? player.toggle() : player.play(m))}
        onToggleLike={() => handleToggleLike(m)}
        onEdit={() => setEditingId(m.id)}
        onOpenProfile={setProfileUserId}
      />
    </div>
  )

  const skeletonRail = (
    <div className={styles.rail}>
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className={`${styles.railItem} ${styles.skeletonCard}`} />
      ))}
    </div>
  )

  const queue =
    player.index >= 0 ? playerList.slice(player.index + 1, player.index + 4) : []

  const hint = recentLoading
    ? 'Loading mashups…'
    : 'Uploads from everyone on bebradio. Click a card to play it here.'

  const renderShelf = (
    title: string,
    items: Track[],
    loading: boolean,
    opts: { emptyText: string; rail?: boolean },
  ) => (
    <section className={styles.shelf}>
      <h2 className={styles.shelfTitle}>{title}</h2>
      {loading ? (
        skeletonRail
      ) : items.length === 0 ? (
        <p className={styles.empty}>{opts.emptyText}</p>
      ) : (
        <div className={styles.rail} ref={opts.rail ? topRailRef : undefined}>
          {items.map(renderCard)}
          {opts.rail && !topDone && <div ref={sentinelRef} className={styles.railSentinel} />}
        </div>
      )}
    </section>
  )

  return (
    <div className={styles.page}>
      <div className={styles.shell}>
        <MashupSidebar
          items={playerList}
          likedItems={liked}
          mineItems={mine}
          signedIn={!!user}
          activeId={activeId}
          isPlaying={player.isPlaying}
          loading={recentLoading}
          onPlay={(m) => (m.id === activeId ? player.toggle() : player.play(m))}
        />

        <div className={styles.main}>
          <div className={styles.mainScroll}>
            <div className={styles.content}>
              <div className={styles.pagehead}>
                <h1 className={styles.title}>Загружайте и слушайте мешапы</h1>
                {!recentLoading && (
                  <span className={styles.counter}>{playerList.length}</span>
                )}
                {user && (
                  <button className="btn" onClick={() => setShowUpload(true)}>
                    Upload
                  </button>
                )}
              </div>
              <p className={styles.sub}>
                Слушайте мешапы онлайн, находите новые треки и собирайте свою очередь музыки.
                {hint && ` ${hint}`}
              </p>

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
            </div>
          </div>
        </div>

        {queueOpen && (
          <NowPlayingPanel
            current={player.current}
            queue={queue}
            loading={recentLoading && !player.current}
            onPlayFromQueue={(m) => player.play(m)}
            onToggleLike={handleToggleLike}
            onOpenProfile={setProfileUserId}
          />
        )}
      </div>

      <MashupPlayer
        player={player}
        onToggleLike={handleToggleLike}
        onExpand={() => setExpanded(true)}
        queueOpen={queueOpen}
        onToggleQueue={() => setQueueOpen((v) => !v)}
      />

      {expanded && player.current && (
        <NowPlayingModal
          player={player}
          queue={queue}
          onToggleLike={handleToggleLike}
          onOpenProfile={setProfileUserId}
          onClose={() => setExpanded(false)}
        />
      )}

      {showUpload && (
        <UploadMashupModal
          onClose={() => setShowUpload(false)}
          onUploaded={handleUploaded}
        />
      )}

      {editing && (
        <EditMashupModal
          mashup={editing}
          canManageCover={!!user && user.id === editing.owner_id}
          canDelete={!!user && user.id === editing.owner_id}
          onChangeCover={(file) => handleChangeCover(editing, file)}
          onDelete={() => handleDelete(editing)}
          onClose={() => setEditingId(null)}
        />
      )}

      {profileUserId && (
        <ProfileModal userId={profileUserId} onClose={() => setProfileUserId(null)} />
      )}
    </div>
  )
}


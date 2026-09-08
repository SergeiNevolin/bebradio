import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/ToastContext'
import { api } from '../lib/api'
import type { Mashup } from '../types'
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
const SEARCH_LIMIT = 50

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

  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [showUpload, setShowUpload] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [profileUserId, setProfileUserId] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)
  const [queueOpen, setQueueOpen] = useState(true)

  const [recent, setRecent] = useState<Mashup[]>([])
  const [recentLoading, setRecentLoading] = useState(true)

  const [top, setTop] = useState<Mashup[]>([])
  const [topLoading, setTopLoading] = useState(true)
  const [topDone, setTopDone] = useState(false)

  const [liked, setLiked] = useState<Mashup[]>([])
  const [likedLoading, setLikedLoading] = useState(false)

  const [mine, setMine] = useState<Mashup[]>([])
  const [mineLoading, setMineLoading] = useState(false)

  const [searchResults, setSearchResults] = useState<Mashup[]>([])
  const [searchLoading, setSearchLoading] = useState(false)

  // Debounce the search box (~300ms) before it becomes a request.
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query.trim()), 300)
    return () => clearTimeout(t)
  }, [query])

  // Apply an update to a single mashup across every list it may appear in.
  const patchAll = useCallback((id: string, updater: (m: Mashup) => Mashup) => {
    const apply = (arr: Mashup[]) => arr.map((m) => (m.id === id ? updater(m) : m))
    setRecent(apply)
    setTop(apply)
    setLiked(apply)
    setMine(apply)
    setSearchResults(apply)
  }, [])

  const removeEverywhere = useCallback((id: string) => {
    const drop = (arr: Mashup[]) => arr.filter((m) => m.id !== id)
    setRecent(drop)
    setTop(drop)
    setLiked(drop)
    setMine(drop)
    setSearchResults(drop)
  }, [])

  // ── Section loads ─────────────────────────────────────────────────
  const loadRecent = useCallback(async () => {
    setRecentLoading(true)
    try {
      setRecent(await api.listMashups('', 'recent', RECENT_LIMIT))
    } catch {
      showToast('Could not load mashups', 'error')
    } finally {
      setRecentLoading(false)
    }
  }, [showToast])

  const topRef = useRef<Mashup[]>([])
  topRef.current = top
  const topBusyRef = useRef(false)

  const loadTopPage = useCallback(
    async (reset: boolean) => {
      if (topBusyRef.current) return
      topBusyRef.current = true
      setTopLoading(true)
      const offset = reset ? 0 : topRef.current.length
      try {
        const page = await api.listMashups('', 'top', TOP_PAGE, offset)
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
      setLiked(await api.likedMashups())
    } catch {
      showToast('Could not load liked mashups', 'error')
    } finally {
      setLikedLoading(false)
    }
  }, [showToast])

  const loadMine = useCallback(async () => {
    setMineLoading(true)
    try {
      setMine(await api.myMashups())
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

  // Search: a non-empty query collapses the shelves to one result grid.
  useEffect(() => {
    if (!debouncedQuery) {
      setSearchResults([])
      return
    }
    let cancelled = false
    setSearchLoading(true)
    api
      .listMashups(debouncedQuery, 'recent', SEARCH_LIMIT)
      .then((r) => {
        if (!cancelled) setSearchResults(r)
      })
      .catch(() => {
        if (!cancelled) showToast('Search failed', 'error')
      })
      .finally(() => {
        if (!cancelled) setSearchLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [debouncedQuery, showToast])

  // Infinite scroll for the "Top by likes" shelf. The observer's root is that
  // rail's own horizontal scroll box.
  const sentinelRef = useRef<HTMLDivElement>(null)
  const topRailRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (debouncedQuery || topDone) return
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
  }, [debouncedQuery, topDone, loadTopPage, top.length])

  // The player's walkable list is every mashup currently loaded, in a stable
  // order. Search results are appended rather than swapped in, so starting a
  // search never drops the playing track out of the list (which would stop it).
  const playerList = useMemo(() => {
    const seen = new Set<string>()
    return [...recent, ...top, ...liked, ...mine, ...searchResults].filter((m) =>
      seen.has(m.id) ? false : seen.add(m.id),
    )
  }, [recent, top, liked, mine, searchResults])

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
      const updated = await Promise.all(ids.map((id) => api.getMashup(id).catch(() => null)))
      updated.forEach((fresh) => {
        if (fresh) patchAll(fresh.id, () => fresh)
      })
    }, 2000)
    return () => clearInterval(timer)
  }, [processingKey, patchAll])

  // ── Actions ──────────────────────────────────────────────────────
  const handleDelete = async (m: Mashup) => {
    try {
      await api.deleteMashup(m.id)
      removeEverywhere(m.id)
      showToast('Mashup deleted', 'success')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Delete failed', 'error')
    }
  }

  const handleUploaded = (m: Mashup) => {
    setRecent((prev) => [m, ...prev.filter((x) => x.id !== m.id)])
    setMine((prev) => [m, ...prev.filter((x) => x.id !== m.id)])
  }

  const handleToggleLike = async (m: Mashup) => {
    if (!user) {
      showToast('Sign in to like mashups', 'error')
      return
    }
    const nextLiked = !m.liked
    const optimistic = (x: Mashup): Mashup => ({
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
      const res = nextLiked ? await api.likeMashup(m.id) : await api.unlikeMashup(m.id)
      const settle = (x: Mashup): Mashup => ({ ...x, liked: res.liked, likes: res.likes })
      patchAll(m.id, settle)
      setLiked((prev) => prev.map((x) => (x.id === m.id ? settle(x) : x)))
    } catch (err) {
      const revert = (x: Mashup): Mashup => ({ ...x, liked: m.liked, likes: m.likes })
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

  const handleChangeCover = async (m: Mashup, file: File) => {
    try {
      await api.uploadMashupCover(m.id, file)
      const fresh = await api.getMashup(m.id)
      patchAll(m.id, () => fresh)
      showToast('Cover updated', 'success')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Could not upload cover', 'error')
    }
  }

  const activeId = player.current?.id

  const renderCard = (m: Mashup) => (
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

  const skeletonGrid = (
    <div className={styles.grid}>
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className={styles.skeletonCard} />
      ))}
    </div>
  )

  const queue =
    player.index >= 0 ? playerList.slice(player.index + 1, player.index + 4) : []

  const hint = recentLoading
    ? 'Loading mashups…'
    : debouncedQuery
      ? 'Showing what matches your search across every uploader.'
      : 'Uploads from everyone on bebradio. Click a card to play it here.'

  const renderShelf = (
    title: string,
    items: Mashup[],
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
            <div className={styles.maintop}>
              <div className={styles.searchWrap}>
                <svg
                  className={styles.searchIcon}
                  width="16"
                  height="16"
                  viewBox="0 0 16 16"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.7"
                  aria-hidden="true"
                >
                  <circle cx="7" cy="7" r="4.5" />
                  <path d="M10.5 10.5 14 14" strokeLinecap="round" />
                </svg>
                <input
                  className={styles.search}
                  type="search"
                  placeholder="Search mashups by title or artist"
                  aria-label="Search mashups"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </div>
              <span className={styles.spacer} />
              {user && (
                <button className="btn" onClick={() => setShowUpload(true)}>
                  Upload
                </button>
              )}
            </div>

            <div className={styles.content}>
              <div className={styles.pagehead}>
                <h1 className={styles.title}>Загружайте и слушайте мешапы</h1>
                {!recentLoading && (
                  <span className={styles.counter}>{playerList.length}</span>
                )}
              </div>
              <p className={styles.sub}>
                Слушайте мешапы онлайн, находите новые треки и собирайте свою очередь музыки.
                {hint && ` ${hint}`}
              </p>

              {debouncedQuery ? (
                <section className={styles.shelf}>
                  <h2 className={styles.shelfTitle}>Search results</h2>
                  {searchLoading ? (
                    skeletonGrid
                  ) : searchResults.length === 0 ? (
                    <div className={styles.blank}>
                      <div className={styles.blankTitle}>No mashups found</div>
                      <div className={styles.blankSub}>Nothing matches this query. Try a shorter one.</div>
                    </div>
                  ) : (
                    <div className={styles.grid}>{searchResults.map(renderCard)}</div>
                  )}
                </section>
              ) : (
                <>
                  {renderShelf('Latest', recent, recentLoading, {
                    emptyText: user ? 'No mashups yet — hit Upload.' : 'No mashups yet.',
                  })}
                  {renderShelf('Top by likes', top, topLoading && top.length === 0, {
                    emptyText: 'No mashups yet.',
                    rail: true,
                  })}
                  {user &&
                    renderShelf('Liked', liked, likedLoading, {
                      emptyText: 'You haven’t liked any mashups yet.',
                    })}
                  {user &&
                    renderShelf('My mashups', mine, mineLoading, {
                      emptyText: 'You have not uploaded any mashups yet.',
                    })}
                </>
              )}
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

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/ToastContext'
import { api } from '../lib/api'
import type { Mashup } from '../types'
import { useMashupPlayer } from '../hooks/useMashupPlayer'
import ScrollRow from '../components/ScrollRow'
import MashupCard from '../components/mashup/MashupCard'
import MashupPlayer from '../components/mashup/MashupPlayer'
import UploadMashupModal from '../components/mashup/UploadMashupModal'
import styles from './Mashups.module.css'

const RECENT_LIMIT = 12
const TOP_PAGE = 24
const SEARCH_LIMIT = 50

export default function Mashups() {
  const { user } = useAuth()
  const { showToast } = useToast()
  const player = useMashupPlayer()
  const { setList } = player

  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [showUpload, setShowUpload] = useState(false)

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

  // Search: a non-empty query collapses the page to one result grid.
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

  // Infinite scroll for the "Top by likes" list.
  const sentinelRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (debouncedQuery || topDone) return
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) loadTopPage(false)
    })
    io.observe(el)
    return () => io.disconnect()
  }, [debouncedQuery, topDone, loadTopPage, top.length])

  // Keep the player's walkable list in step with what is on screen.
  const playerList = useMemo(() => {
    const source = debouncedQuery
      ? searchResults
      : [...recent, ...top, ...liked, ...mine]
    const seen = new Set<string>()
    return source.filter((m) => (seen.has(m.id) ? false : seen.add(m.id)))
  }, [debouncedQuery, searchResults, recent, top, liked, mine])

  useEffect(() => {
    setList(playerList)
  }, [playerList, setList])

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
    <MashupCard
      key={m.id}
      mashup={m}
      active={m.id === activeId}
      isPlaying={player.isPlaying}
      canDelete={!!user && user.id === m.owner_id}
      canManageCover={!!user && user.id === m.owner_id}
      onPlay={() => player.play(m)}
      onDelete={() => handleDelete(m)}
      onToggleLike={() => handleToggleLike(m)}
      onChangeCover={(file) => handleChangeCover(m, file)}
    />
  )

  const skeletonGrid = (
    <div className={styles.grid}>
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className={styles.skeleton} />
      ))}
    </div>
  )

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>Mashups</h1>
        {user && (
          <button className="btn" onClick={() => setShowUpload(true)}>
            Upload
          </button>
        )}
      </div>

      <div className={styles.toolbar}>
        <input
          className={styles.search}
          type="search"
          placeholder="Search mashups…"
          aria-label="Search mashups"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      {debouncedQuery ? (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Search results</h2>
          {searchLoading ? (
            skeletonGrid
          ) : searchResults.length === 0 ? (
            <div className={styles.empty}>
              <p>No mashups found.</p>
            </div>
          ) : (
            <div className={styles.grid}>{searchResults.map(renderCard)}</div>
          )}
        </section>
      ) : (
        <>
          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>Latest</h2>
            {recentLoading ? (
              skeletonGrid
            ) : recent.length === 0 ? (
              <div className={styles.empty}>
                <p>No mashups yet.</p>
                {user && <p className={styles.emptySub}>Be the first — hit Upload.</p>}
              </div>
            ) : (
              <ScrollRow>
                {recent.map((m) => (
                  <div key={m.id} className={styles.rowItem}>
                    {renderCard(m)}
                  </div>
                ))}
              </ScrollRow>
            )}
          </section>

          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>Top by likes</h2>
            {topLoading && top.length === 0 ? (
              skeletonGrid
            ) : top.length === 0 ? (
              <div className={styles.empty}>
                <p>No mashups yet.</p>
              </div>
            ) : (
              <>
                <div className={styles.grid}>{top.map(renderCard)}</div>
                {!topDone && <div ref={sentinelRef} className={styles.sentinel} />}
                {topLoading && <div className={styles.loadingMore}>Loading…</div>}
              </>
            )}
          </section>

          {user && (
            <section className={styles.section}>
              <h2 className={styles.sectionTitle}>Liked</h2>
              {likedLoading ? (
                skeletonGrid
              ) : liked.length === 0 ? (
                <div className={styles.empty}>
                  <p>You haven’t liked any mashups yet.</p>
                </div>
              ) : (
                <div className={styles.grid}>{liked.map(renderCard)}</div>
              )}
            </section>
          )}

          {user && (
            <section className={styles.section}>
              <h2 className={styles.sectionTitle}>My mashups</h2>
              {mineLoading ? (
                skeletonGrid
              ) : mine.length === 0 ? (
                <div className={styles.empty}>
                  <p>You have not uploaded any mashups yet.</p>
                </div>
              ) : (
                <div className={styles.grid}>{mine.map(renderCard)}</div>
              )}
            </section>
          )}
        </>
      )}

      <MashupPlayer player={player} />

      {showUpload && (
        <UploadMashupModal
          onClose={() => setShowUpload(false)}
          onUploaded={handleUploaded}
        />
      )}
    </div>
  )
}

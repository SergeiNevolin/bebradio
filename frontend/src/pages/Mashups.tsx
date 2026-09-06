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

type Tab = 'all' | 'mine'

export default function Mashups() {
  const { user } = useAuth()
  const { showToast } = useToast()
  const player = useMashupPlayer()
  const { setList } = player

  const [tab, setTab] = useState<Tab>('all')
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [mashups, setMashups] = useState<Mashup[]>([])
  const [loading, setLoading] = useState(true)
  const [showUpload, setShowUpload] = useState(false)

  // Debounce the search box (~300ms) before it becomes a request.
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query.trim()), 300)
    return () => clearTimeout(t)
  }, [query])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data =
        tab === 'mine' ? await api.myMashups() : await api.listMashups(debouncedQuery)
      setMashups(data)
    } catch {
      showToast('Could not load mashups', 'error')
    } finally {
      setLoading(false)
    }
  }, [tab, debouncedQuery, showToast])

  useEffect(() => {
    load()
  }, [load])

  // Keep the player's list in step with what is on screen so next/prev walk it.
  useEffect(() => {
    setList(mashups)
  }, [mashups, setList])

  // Poll any still-processing mashups every 2s until they settle.
  const processingIds = useMemo(
    () => mashups.filter((m) => m.status === 'processing').map((m) => m.id),
    [mashups],
  )
  const processingKey = processingIds.join(',')
  const pollRef = useRef(processingIds)
  pollRef.current = processingIds

  useEffect(() => {
    if (pollRef.current.length === 0) return
    const timer = setInterval(async () => {
      const ids = pollRef.current
      if (ids.length === 0) return
      const updated = await Promise.all(
        ids.map((id) => api.getMashup(id).catch(() => null)),
      )
      setMashups((prev) =>
        prev.map((m) => {
          const fresh = updated.find((u) => u && u.id === m.id)
          return fresh ? fresh : m
        }),
      )
    }, 2000)
    return () => clearInterval(timer)
  }, [processingKey])

  const handleDelete = async (m: Mashup) => {
    try {
      await api.deleteMashup(m.id)
      setMashups((prev) => prev.filter((x) => x.id !== m.id))
      showToast('Mashup deleted', 'success')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Delete failed', 'error')
    }
  }

  const handleUploaded = (m: Mashup) => {
    setMashups((prev) => [m, ...prev.filter((x) => x.id !== m.id)])
  }

  const recent = mashups.slice(0, 10)
  const activeId = player.current?.id

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
        {user && (
          <div className={styles.tabs} role="tablist">
            <button
              role="tab"
              aria-selected={tab === 'all'}
              className={`${styles.tab} ${tab === 'all' ? styles.tabOn : ''}`}
              onClick={() => setTab('all')}
            >
              All
            </button>
            <button
              role="tab"
              aria-selected={tab === 'mine'}
              className={`${styles.tab} ${tab === 'mine' ? styles.tabOn : ''}`}
              onClick={() => setTab('mine')}
            >
              Mine
            </button>
          </div>
        )}
      </div>

      {loading ? (
        <div className={styles.grid}>
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className={styles.skeleton} />
          ))}
        </div>
      ) : mashups.length === 0 ? (
        <div className={styles.empty}>
          <p>{tab === 'mine' ? 'You have not uploaded any mashups yet.' : 'No mashups found.'}</p>
          {user && tab !== 'mine' && (
            <p className={styles.emptySub}>Be the first — hit Upload.</p>
          )}
        </div>
      ) : (
        <>
          {tab === 'all' && !debouncedQuery && recent.length > 0 && (
            <section className={styles.section}>
              <h2 className={styles.sectionTitle}>Latest</h2>
              <ScrollRow>
                {recent.map((m) => (
                  <div key={m.id} className={styles.rowItem}>
                    <MashupCard
                      mashup={m}
                      active={m.id === activeId}
                      isPlaying={player.isPlaying}
                      canDelete={!!user && user.id === m.owner_id}
                      onPlay={() => player.play(m)}
                      onDelete={() => handleDelete(m)}
                    />
                  </div>
                ))}
              </ScrollRow>
            </section>
          )}

          <section className={styles.section}>
            <div className={styles.grid}>
              {mashups.map((m) => (
                <MashupCard
                  key={m.id}
                  mashup={m}
                  active={m.id === activeId}
                  isPlaying={player.isPlaying}
                  canDelete={!!user && user.id === m.owner_id}
                  onPlay={() => player.play(m)}
                  onDelete={() => handleDelete(m)}
                />
              ))}
            </div>
          </section>
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

import { useState, useRef, useEffect, useCallback, memo } from 'react'
import { useToast } from '../context/ToastContext'
import { api } from '../lib/api'
import { formatTime as formatDuration } from '../lib/format'
import type { Track } from '../types'
import styles from './AddTrack.module.css'

interface YoutubeHit {
  id: string
  title: string
  artist: string
  thumbnail: string
  duration: number
  url: string
}

// Filters over the single unified result list. To add a source: extend
// SearchFilter, add a FILTERS entry, fetch it in runSearch and tag its rows;
// the backend dispatcher (room_handler.go) is symmetric — library sources go
// by track_id, URL sources by url+source.
type SearchFilter = 'all' | 'youtube' | 'bebradio'

const FILTERS: Array<{ id: SearchFilter; label: string }> = [
  { id: 'all', label: 'All' },
  { id: 'youtube', label: 'YouTube' },
  { id: 'bebradio', label: 'bebradio' },
]

interface UnifiedResult {
  key: string
  kind: 'youtube' | 'mashup'
  id: string
  title: string
  artist: string
  thumbnail: string
  duration: number
  /** YouTube URL to enqueue; '' for library rows (enqueued by id). */
  url: string
  ready: boolean
  /** Mashup processing state for the meta line; '' when ready. */
  statusNote: string
}

interface AddTrackProps {
  onAdd: (url: string) => Promise<{ success: boolean; error?: string }>
  /** Add a library track (mashup today) by id. Absent → YouTube-only UI. */
  onAddById?: (trackId: string) => Promise<{ success: boolean; error?: string }>
}

function isUrl(text: string): boolean {
  return /^https?:\/\//.test(text.trim())
}

function toYoutubeRow(r: YoutubeHit): UnifiedResult {
  return {
    key: `yt:${r.id}`,
    kind: 'youtube',
    id: r.id,
    title: r.title,
    artist: r.artist,
    thumbnail: r.thumbnail,
    duration: r.duration,
    url: r.url,
    ready: true,
    statusNote: '',
  }
}

function toMashupRow(m: Track): UnifiedResult {
  return {
    key: `m:${m.id}`,
    kind: 'mashup',
    id: m.id,
    title: m.title,
    artist: m.artist || 'Unknown artist',
    thumbnail: m.thumbnail,
    duration: m.duration,
    url: '',
    ready: m.status === 'ready',
    statusNote: m.status === 'ready' ? '' : m.status,
  }
}

function AddTrack({ onAdd, onAddById }: AddTrackProps) {
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<SearchFilter>('all')
  const [youtube, setYoutube] = useState<UnifiedResult[]>([])
  const [mashups, setMashups] = useState<UnifiedResult[]>([])
  const [searching, setSearching] = useState(false)
  const [searched, setSearched] = useState(false)
  const [adding, setAdding] = useState(false)
  const [showDropdown, setShowDropdown] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const searchRef = useRef(0)
  const { showToast } = useToast()

  const visible =
    filter === 'all' ? [...youtube, ...mashups] : filter === 'youtube' ? youtube : mashups

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Both providers resolve independently: a slow YouTube lookup must not
  // hold back instant library hits (each completion re-renders on its own).
  const runSearch = useCallback((q: string) => {
    const trimmed = q.trim()
    if (!trimmed || isUrl(trimmed) || trimmed.length < 2) {
      setYoutube([])
      setMashups([])
      setShowDropdown(false)
      setSearched(false)
      return
    }
    const reqId = ++searchRef.current
    setSearching(true)
    setSearched(false)
    setShowDropdown(true)
    setActiveIndex(-1)

    let pending = onAddById ? 2 : 1
    const settleOne = () => {
      if (reqId !== searchRef.current) return
      pending -= 1
      if (pending === 0) {
        setSearched(true)
        setSearching(false)
      }
    }

    api.searchTracks(trimmed).then(
      (hits) => {
        if (reqId === searchRef.current) setYoutube(hits.map(toYoutubeRow))
      },
      () => {
        if (reqId === searchRef.current) setYoutube([])
      },
    ).finally(settleOne)

    if (onAddById) {
      api.listTracks(trimmed, 'recent', 8).then(
        (tracks) => {
          if (reqId === searchRef.current) setMashups(tracks.map(toMashupRow))
        },
        () => {
          if (reqId === searchRef.current) setMashups([])
        },
      ).finally(settleOne)
    }
  }, [onAddById])

  const handleQueryChange = (value: string) => {
    setQuery(value)
    setSearched(false)
    setActiveIndex(-1)
    if (searchTimer.current) clearTimeout(searchTimer.current)
    if (!isUrl(value) && value.trim().length > 1) {
      setSearching(true)
      setShowDropdown(true)
      searchTimer.current = setTimeout(() => runSearch(value), 400)
    } else {
      setYoutube([])
      setMashups([])
      setShowDropdown(false)
      setSearching(false)
    }
  }

  const resetAfterAdd = () => {
    setQuery('')
    setYoutube([])
    setMashups([])
    setShowDropdown(false)
    setSearched(false)
    setActiveIndex(-1)
  }

  const clearInput = () => {
    resetAfterAdd()
    inputRef.current?.focus()
  }

  const addTrack = async (url: string) => {
    setAdding(true)
    const res = await onAdd(url)
    if (res.success) {
      resetAfterAdd()
      showToast('Added!')
    } else {
      showToast(res.error || 'Failed to add track', 'error')
    }
    setAdding(false)
  }

  const addMashup = async (id: string) => {
    if (!onAddById || adding) return
    setAdding(true)
    const res = await onAddById(id)
    if (res.success) {
      resetAfterAdd()
      showToast('Added!')
    } else {
      showToast(res.error || 'Failed to add track', 'error')
    }
    setAdding(false)
  }

  const select = async (result: UnifiedResult) => {
    if (result.kind === 'youtube') {
      setShowDropdown(false)
      setQuery(result.title)
      await addTrack(result.url)
    } else if (result.ready) {
      await addMashup(result.id)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = query.trim()
    if (!trimmed || adding) return

    if (isUrl(trimmed)) {
      await addTrack(trimmed)
    } else if (activeIndex >= 0 && activeIndex < visible.length) {
      await select(visible[activeIndex])
    } else if (visible.length > 0) {
      await select(visible[0])
    } else {
      runSearch(trimmed)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showDropdown) return

    if (e.key === 'Escape') {
      setShowDropdown(false)
      setActiveIndex(-1)
      return
    }

    if (!visible.length) return

    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIndex((prev) => (prev < visible.length - 1 ? prev + 1 : 0))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIndex((prev) => (prev > 0 ? prev - 1 : visible.length - 1))
    }
  }

  const applyFilter = (f: SearchFilter) => {
    setFilter(f)
    setActiveIndex(-1)
    // Chip clicks land outside the dropdown wrapper (which closes it), so
    // reopen it: the user is still mid-search.
    if (query.trim().length > 1) {
      setShowDropdown(true)
    }
  }

  const showNoResults =
    searched && !searching && visible.length === 0 && query.trim().length > 1 && !isUrl(query)

  return (
    <div className={styles.addTrack}>
      <h3>Add Track</h3>
      {onAddById && (
        <div className={styles.filterChips} role="group" aria-label="Search source filter">
          {FILTERS.map((f) => (
            <button
              key={f.id}
              type="button"
              aria-pressed={filter === f.id}
              className={`${styles.filterChip}${filter === f.id ? ` ${styles.filterChipActive}` : ''}`}
              onClick={() => applyFilter(f.id)}
            >
              {f.label}
            </button>
          ))}
        </div>
      )}
      <form className={styles.addTrackForm} onSubmit={handleSubmit}>
        <div className={styles.searchWrapper} ref={dropdownRef}>
          <input
            ref={inputRef}
            type="text"
            placeholder={onAddById ? 'Search YouTube or bebradio...' : 'Search or paste YouTube URL...'}
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            onFocus={() => (visible.length > 0 || showNoResults || searching) && setShowDropdown(true)}
            onKeyDown={handleKeyDown}
            disabled={adding}
            className={adding ? styles.inputDisabled : ''}
          />
          {searching && <span className={styles.searchSpinner} />}
          {query && !adding && (
            <button type="button" className={styles.searchClear} onClick={clearInput}>
              ×
            </button>
          )}
          {showDropdown && (visible.length > 0 || showNoResults || searching) && (
            <div className={styles.searchDropdown}>
              {searching && visible.length === 0 && (
                <div className={styles.searchLoading}>Searching...</div>
              )}
              {visible.map((r, i) => (
                <div
                  key={r.key}
                  className={`${styles.searchResult}${i === activeIndex ? ` ${styles.searchResultActive}` : ''}`}
                  onClick={() => select(r)}
                  onMouseEnter={() => setActiveIndex(i)}
                >
                  {r.thumbnail ? (
                    <img className={styles.searchResultThumb} src={r.thumbnail} alt="" />
                  ) : (
                    <span className={styles.searchResultThumb} aria-hidden="true" />
                  )}
                  <div className={styles.searchResultInfo}>
                    <div className={styles.searchResultTitle}>{r.title}</div>
                    <div className={styles.searchResultMeta}>
                      {r.artist}
                      {r.duration > 0 && <> · {formatDuration(r.duration)}</>}
                      {r.statusNote && <> · {r.statusNote}…</>}
                    </div>
                  </div>
                  <span className={styles.resultBadge}>
                    {r.kind === 'youtube' ? 'YouTube' : 'bebradio'}
                  </span>
                  <button
                    type="button"
                    className="btn"
                    disabled={!r.ready || adding}
                    onClick={(e) => {
                      e.stopPropagation()
                      select(r)
                    }}
                  >
                    Add
                  </button>
                </div>
              ))}
              {searching && visible.length > 0 && (
                <div className={styles.searchLoading}>Searching more...</div>
              )}
              {showNoResults && (
                <div className={styles.searchNoResults}>No results found</div>
              )}
            </div>
          )}
        </div>
        <button className="btn" type="submit" disabled={adding || !query.trim()}>
          {adding ? (
            <span className={styles.btnContent}><span className={styles.btnSpinner} /> Adding...</span>
          ) : (
            'Add'
          )}
        </button>
      </form>
    </div>
  )
}

export default memo(AddTrack)

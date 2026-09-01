import { useState, useRef, useEffect, useCallback } from 'react'
import { useToast } from '../context/ToastContext'

interface SearchResult {
  id: string
  title: string
  artist: string
  thumbnail: string
  duration: number
  url: string
}

interface AddTrackProps {
  onAdd: (url: string) => Promise<{ success: boolean; error?: string }>
}

function formatDuration(s: number): string {
  if (!s) return ''
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${sec.toString().padStart(2, '0')}`
}

function isUrl(text: string): boolean {
  return /^https?:\/\//.test(text.trim())
}

export default function AddTrack({ onAdd }: AddTrackProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [searching, setSearching] = useState(false)
  const [adding, setAdding] = useState(false)
  const [showDropdown, setShowDropdown] = useState(false)
  const [searched, setSearched] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const abortRef = useRef(0)
  const { showToast } = useToast()

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const search = useCallback(async (q: string) => {
    if (!q.trim() || isUrl(q)) {
      setResults([])
      setShowDropdown(false)
      setSearched(false)
      return
    }
    const reqId = ++abortRef.current
    setSearching(true)
    setSearched(false)
    try {
      const res = await fetch('/api/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: q.trim(), limit: 5 }),
      })
      if (reqId !== abortRef.current) return
      const data = await res.json()
      setResults(data)
      setSearched(true)
      setShowDropdown(true)
      setActiveIndex(-1)
    } catch {
      if (reqId !== abortRef.current) return
      setResults([])
      setSearched(true)
    }
    setSearching(false)
  }, [])

  const handleQueryChange = (value: string) => {
    setQuery(value)
    setSearched(false)
    setActiveIndex(-1)
    if (searchTimer.current) clearTimeout(searchTimer.current)
    if (!isUrl(value) && value.trim().length > 1) {
      setSearching(true)
      setShowDropdown(true)
      searchTimer.current = setTimeout(() => search(value), 400)
    } else {
      setResults([])
      setShowDropdown(false)
      setSearching(false)
    }
  }

  const clearInput = () => {
    setQuery('')
    setResults([])
    setShowDropdown(false)
    setSearched(false)
    setActiveIndex(-1)
    inputRef.current?.focus()
  }

  const addTrack = async (url: string) => {
    setAdding(true)
    const res = await onAdd(url)
    if (res.success) {
      setQuery('')
      setResults([])
      setShowDropdown(false)
      setSearched(false)
      showToast('Added!')
    } else {
      showToast(res.error || 'Failed to add track', 'error')
    }
    setAdding(false)
  }

  const handleSelect = async (result: SearchResult) => {
    setShowDropdown(false)
    setQuery(result.title)
    await addTrack(result.url)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = query.trim()
    if (!trimmed || adding) return

    if (isUrl(trimmed)) {
      await addTrack(trimmed)
    } else if (activeIndex >= 0 && activeIndex < results.length) {
      await handleSelect(results[activeIndex])
    } else if (results.length > 0) {
      await handleSelect(results[0])
    } else {
      search(trimmed)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showDropdown) return

    if (e.key === 'Escape') {
      setShowDropdown(false)
      setActiveIndex(-1)
      return
    }

    if (!results.length) return

    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIndex((prev) => (prev < results.length - 1 ? prev + 1 : 0))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIndex((prev) => (prev > 0 ? prev - 1 : results.length - 1))
    }
  }

  const busy = adding || searching
  const showNoResults = searched && !searching && results.length === 0 && query.trim().length > 1 && !isUrl(query)

  return (
    <div className="add-track">
      <h3>Add Track</h3>
      <form className="add-track-form" onSubmit={handleSubmit}>
        <div className="search-wrapper" ref={dropdownRef}>
          <input
            ref={inputRef}
            type="text"
            placeholder="Search or paste YouTube URL..."
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            onFocus={() => (results.length > 0 || showNoResults || searching) && setShowDropdown(true)}
            onKeyDown={handleKeyDown}
            disabled={adding}
            className={adding ? 'input-disabled' : ''}
          />
          {searching && <span className="search-spinner" />}
          {query && !adding && (
            <button type="button" className="search-clear" onClick={clearInput}>
              ×
            </button>
          )}
          {showDropdown && (results.length > 0 || showNoResults || searching) && (
            <div className="search-dropdown">
              {searching && results.length === 0 && (
                <div className="search-loading">Searching...</div>
              )}
              {results.map((r, i) => (
                <div
                  key={r.id}
                  className={`search-result${i === activeIndex ? ' active' : ''}`}
                  onClick={() => handleSelect(r)}
                  onMouseEnter={() => setActiveIndex(i)}
                >
                  {r.thumbnail && (
                    <img className="search-result-thumb" src={r.thumbnail} alt="" />
                  )}
                  <div className="search-result-info">
                    <div className="search-result-title">{r.title}</div>
                    <div className="search-result-meta">
                      {r.artist}
                      {r.duration > 0 && <> · {formatDuration(r.duration)}</>}
                    </div>
                  </div>
                </div>
              ))}
              {searching && results.length > 0 && (
                <div className="search-loading">Searching more...</div>
              )}
              {showNoResults && (
                <div className="search-no-results">No results found</div>
              )}
            </div>
          )}
        </div>
        <button className="btn" type="submit" disabled={adding || !query.trim()}>
          {adding ? (
            <span className="btn-content"><span className="btn-spinner" /> Adding...</span>
          ) : (
            'Add'
          )}
        </button>
      </form>
    </div>
  )
}

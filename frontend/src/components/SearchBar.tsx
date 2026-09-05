import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'

interface RoomResult {
  id: string
  name: string
  user_count: number
  is_playing: boolean
}

export default function SearchBar() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<RoomResult[]>([])
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    if (!query.trim()) {
      setResults([])
      return
    }
    const timer = setTimeout(async () => {
      try {
        const res = await fetch('/api/rooms')
        const data: RoomResult[] = await res.json()
        const q = query.trim().toLowerCase()
        setResults(
          data.filter(
            (r) => r.name.toLowerCase().includes(q) || r.id.toLowerCase().includes(q),
          ),
        )
      } catch { /* ignore */ }
    }, 200)
    return () => clearTimeout(timer)
  }, [query])

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const handleSelect = (roomId: string) => {
    setQuery('')
    setOpen(false)
    navigate(`/room/${roomId}`)
  }

  return (
    <div className="navbar-search" ref={ref}>
      <input
        type="text"
        className="search-bar-input"
        placeholder="Search rooms..."
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => query.trim() && setOpen(true)}
      />
      {open && results.length > 0 && (
        <div className="search-bar-dropdown">
          {results.slice(0, 8).map((room) => (
            <div
              key={room.id}
              className="search-bar-item"
              onClick={() => handleSelect(room.id)}
            >
              <div className="search-bar-item-name">
                {room.name}
              </div>
              <div className="search-bar-item-meta">
                <span className="search-bar-item-code">{room.id}</span>
                {room.is_playing && <span className="room-list-item-playing">LIVE</span>}
                <span>{room.user_count} listening</span>
              </div>
            </div>
          ))}
        </div>
      )}
      {open && query.trim() && results.length === 0 && (
        <div className="search-bar-dropdown">
          <div className="search-bar-empty">No rooms found</div>
        </div>
      )}
    </div>
  )
}

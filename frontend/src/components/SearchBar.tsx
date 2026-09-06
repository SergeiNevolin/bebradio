import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { type RoomListItem } from '../types'
import styles from './SearchBar.module.css'

export default function SearchBar() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<RoomListItem[]>([])
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
        const rooms = await api.getRooms()
        const q = query.trim().toLowerCase()
        setResults(
          rooms.filter(
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
    <div className={styles.navbarSearch} ref={ref}>
      <input
        type="text"
        className={styles.searchBarInput}
        placeholder="Search rooms..."
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => query.trim() && setOpen(true)}
      />
      {open && results.length > 0 && (
        <div className={styles.searchBarDropdown}>
          {results.slice(0, 8).map((room) => (
            <div
              key={room.id}
              className={styles.searchBarItem}
              onClick={() => handleSelect(room.id)}
            >
              <div className={styles.searchBarItemName}>
                {room.name}
              </div>
              <div className={styles.searchBarItemMeta}>
                <span className={styles.searchBarItemCode}>{room.id}</span>
                {room.is_playing && <span className={styles.roomListItemPlaying}>LIVE</span>}
                <span>{room.user_count} listening</span>
              </div>
            </div>
          ))}
        </div>
      )}
      {open && query.trim() && results.length === 0 && (
        <div className={styles.searchBarDropdown}>
          <div className={styles.searchBarEmpty}>No rooms found</div>
        </div>
      )}
    </div>
  )
}

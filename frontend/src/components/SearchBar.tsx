import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { type RoomListItem, type Track } from '../types'
import styles from './SearchBar.module.css'

export default function SearchBar() {
  const [query, setQuery] = useState('')
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [tracks, setTracks] = useState<Track[]>([])
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    if (!query.trim()) {
      setRooms([])
      setTracks([])
      return
    }
    const timer = setTimeout(async () => {
      const q = query.trim().toLowerCase()
      try {
        const [allRooms, allTracks] = await Promise.all([
          api.getRooms(),
          api.listTracks(q, 'recent', 5),
        ])
        setRooms(
          allRooms.filter(
            (r) => r.name.toLowerCase().includes(q) || r.id.toLowerCase().includes(q),
          ).slice(0, 5),
        )
        setTracks(allTracks)
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

  const handleSelectRoom = (roomId: string) => {
    setQuery('')
    setOpen(false)
    navigate(`/room/${roomId}`)
  }

  const handleSelectTrack = () => {
    setQuery('')
    setOpen(false)
    navigate('/mashup')
  }

  const hasResults = rooms.length > 0 || tracks.length > 0
  const isEmpty = query.trim() && !hasResults

  return (
    <div className={styles.navbarSearch} ref={ref}>
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
        type="text"
        className={styles.searchBarInput}
        placeholder="Поиск комнат и мэшапов..."
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => query.trim() && setOpen(true)}
      />
      {open && hasResults && (
        <div className={styles.searchBarDropdown}>
          {rooms.length > 0 && (
            <>
              <div className={styles.searchBarSection}>Комнаты</div>
              {rooms.map((room) => (
                <div
                  key={room.id}
                  className={styles.searchBarItem}
                  onClick={() => handleSelectRoom(room.id)}
                >
                  <div className={styles.searchBarItemName}>{room.name}</div>
                  <div className={styles.searchBarItemMeta}>
                    <span className={styles.searchBarItemCode}>{room.id}</span>
                    {room.is_playing && <span className={styles.roomListItemPlaying}>LIVE</span>}
                    <span>{room.user_count} слушают</span>
                  </div>
                </div>
              ))}
            </>
          )}
          {tracks.length > 0 && (
            <>
              <div className={styles.searchBarSection}>Мэшапы</div>
              {tracks.map((track) => (
                <div
                  key={track.id}
                  className={styles.searchBarItem}
                  onClick={handleSelectTrack}
                >
                  <div className={styles.searchBarItemName}>{track.title}</div>
                  <div className={styles.searchBarItemMeta}>
                    <span>{track.artist || 'Unknown artist'}</span>
                    <span>♥ {track.likes}</span>
                  </div>
                </div>
              ))}
            </>
          )}
        </div>
      )}
      {open && isEmpty && (
        <div className={styles.searchBarDropdown}>
          <div className={styles.searchBarEmpty}>Ничего не найдено</div>
        </div>
      )}
    </div>
  )
}

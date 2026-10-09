import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { useRoomEntry } from '../hooks/useRoomEntry'
import RoomCard from '../components/RoomCard'
import Shelf from '../components/media/Shelf'
import Hero from '../components/media/Hero'
import { type RoomListItem } from '../types'
import styles from './Rooms.module.css'

export default function Rooms() {
  const { user } = useAuth()
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [roomsLoading, setRoomsLoading] = useState(true)
  const [recentRooms, setRecentRooms] = useState<RoomListItem[]>([])
  const entry = useRoomEntry()

  const fetchRooms = async () => {
    try {
      const rooms = await api.getRooms()
      setRooms(rooms)
    } catch { /* ignore */ }
    setRoomsLoading(false)
  }

  const fetchRecent = useCallback(async () => {
    if (!user) return
    try {
      const rooms = await api.getRecentRooms()
      setRecentRooms(rooms)
    } catch { /* ignore */ }
  }, [user])

  useEffect(() => {
    fetchRooms()
    fetchRecent()
    const interval = setInterval(fetchRooms, 5000)
    return () => clearInterval(interval)
  }, [fetchRecent])

  const activeRooms = rooms.filter((r) => r.user_count > 0 || (r.track_count ?? 0) > 0)
  const idleRooms = rooms.filter((r) => r.user_count === 0 && (r.track_count ?? 0) === 0)
  const totalListeners = rooms.reduce((sum, r) => sum + r.user_count, 0)

  return (
    <div className={styles.rooms}>
      <Hero
        title="Слушать музыку вместе с друзьями"
        sub="Создайте музыкальную комнату и слушайте треки вместе онлайн в синхронном режиме."
        actions={
          <>
            <button className="btn" onClick={entry.openCreateModal}>
              Create Room
            </button>
            <button className="btn btn-secondary" onClick={entry.openJoinModal}>
              Join by Code
            </button>
          </>
        }
        stats={
          <>
            <div className={styles.heroStat}>
              <span className={styles.heroStatNum}>{rooms.length}</span>
              <span className={styles.roomsHeroStatLabel}>rooms</span>
            </div>
            <div className={styles.heroStat}>
              <span className={styles.heroStatNum}>{totalListeners}</span>
              <span className={styles.roomsHeroStatLabel}>listening</span>
            </div>
          </>
        }
      />
      {entry.error && <div className="error-msg">{entry.error}</div>}

      {user && recentRooms.length > 0 && (
        <Shelf title="Recently Played">
          {recentRooms.map((room) => (
            <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
          ))}
        </Shelf>
      )}

      {roomsLoading ? (
        <div className={styles.roomsLoading}>Loading...</div>
      ) : activeRooms.length === 0 && idleRooms.length === 0 ? (
        <div className={styles.roomsEmpty}>
          <p>No rooms yet</p>
          <p className={styles.roomsEmptySub}>Create the first one!</p>
        </div>
      ) : (
        // Основной список комнат (исключает Recently Played): активные в
        // «All rooms», пустые — в «Waiting for listeners». data-testid нужен
        // e2e, чтобы находить карточку вне зависимости от подполки.
        <div data-testid="rooms-list">
          {activeRooms.length > 0 && (
            <Shelf title="All rooms">
              {activeRooms.map((room) => (
                <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
              ))}
            </Shelf>
          )}
          {idleRooms.length > 0 && (
            <Shelf title="Waiting for listeners" small>
              {idleRooms.map((room) => (
                <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
              ))}
            </Shelf>
          )}
        </div>
      )}

      {entry.entryModals}
    </div>
  )
}

import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { useRoomEntry } from '../hooks/useRoomEntry'
import RoomCard from '../components/RoomCard'
import ScrollRow from '../components/ScrollRow'
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
      {/* Hero */}
      <div className={styles.roomsHero}>
        <div className={styles.roomsHeroContent}>
          <h1 className={styles.roomsHeroTitle}>Слушать музыку вместе с друзьями</h1>
          <p className={styles.roomsHeroSub}>Создайте музыкальную комнату и слушайте треки вместе онлайн в синхронном режиме.</p>
          <div className={styles.roomsHeroActions}>
            <button className={`btn ${styles.btnHero}`} onClick={entry.openCreateModal}>
              Create Room
            </button>
            <button className={`btn ${styles.btnHero} btn-secondary`} onClick={entry.openJoinModal}>
              Join by Code
            </button>
          </div>
          {entry.error && <div className="error-msg" style={{ marginTop: 12 }}>{entry.error}</div>}
        </div>
        <div className={styles.roomsHeroStats}>
          <div className={styles.roomsHeroStat}>
            <span className={styles.roomsHeroStatNum}>{rooms.length}</span>
            <span className={styles.roomsHeroStatLabel}>rooms</span>
          </div>
          <div className={styles.roomsHeroStat}>
            <span className={styles.roomsHeroStatNum}>{totalListeners}</span>
            <span className={styles.roomsHeroStatLabel}>listening</span>
          </div>
        </div>
      </div>

      {user && recentRooms.length > 0 && (
        <section className={styles.roomsSection}>
          <div className={styles.roomsSectionHeader}>
            <h2 className={styles.roomsSectionTitle}>Recently Played</h2>
          </div>
          <ScrollRow>
            {recentRooms.map((room) => (
              <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
            ))}
          </ScrollRow>
        </section>
      )}

      <section className={styles.roomsSection}>
        <div className={styles.roomsSectionHeader}>
          <h2 className={styles.roomsSectionTitle}>All rooms</h2>
        </div>
        {roomsLoading ? (
          <div className={styles.roomsLoading}>Loading...</div>
        ) : activeRooms.length === 0 && idleRooms.length === 0 ? (
          <div className={styles.roomsEmpty}>
            <p>No rooms yet</p>
            <p className={styles.roomsEmptySub}>Create the first one!</p>
          </div>
        ) : (
          <>
            {activeRooms.length > 0 && (
              <ScrollRow>
                {activeRooms.map((room) => (
                  <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
                ))}
              </ScrollRow>
            )}
            {idleRooms.length > 0 && (
              <>
                <div className={styles.roomsSectionHeader} style={{ marginTop: 24 }}>
                  <h2 className={styles.roomsSectionTitle} style={{ fontSize: 14, color: 'var(--text-muted)' }}>Waiting for listeners</h2>
                </div>
                <ScrollRow>
                  {idleRooms.map((room) => (
                    <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
                  ))}
                </ScrollRow>
              </>
            )}
          </>
        )}
      </section>

      {entry.entryModals}
    </div>
  )
}

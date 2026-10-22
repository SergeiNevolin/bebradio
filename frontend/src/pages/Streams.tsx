import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { useStreamEntry } from '../hooks/useStreamEntry'
import RoomCard from '../components/RoomCard'
import Shelf from '../components/media/Shelf'
import Hero from '../components/media/Hero'
import type { StreamListItem } from '../types'
import styles from './Rooms.module.css'

export default function Streams() {
  const { user } = useAuth()
  const [streams, setStreams] = useState<StreamListItem[]>([])
  const [loading, setLoading] = useState(true)
  const entry = useStreamEntry()
  const isAdmin = user?.role === 'admin'

  const fetchStreams = async () => {
    try {
      setStreams(await api.getStreams(100))
    } catch { /* ignore */ }
    setLoading(false)
  }

  useEffect(() => {
    fetchStreams()
    const interval = setInterval(fetchStreams, 5000)
    return () => clearInterval(interval)
  }, [])

  const totalListeners = streams.reduce((sum, s) => sum + s.user_count, 0)

  return (
    <div className={styles.rooms}>
      <Hero
        title="Потоки 24/7"
        sub="Круглосуточные радиостанции от администраторов. Без голосования — только музыка."
        actions={
          isAdmin ? (
            <button className="btn" onClick={entry.openCreateModal}>
              Создать поток
            </button>
          ) : undefined
        }
        stats={
          <>
            <div className={styles.heroStat}>
              <span className={styles.heroStatNum}>{streams.length}</span>
              <span className={styles.roomsHeroStatLabel}>потоков</span>
            </div>
            <div className={styles.heroStat}>
              <span className={styles.heroStatNum}>{totalListeners}</span>
              <span className={styles.roomsHeroStatLabel}>слушают</span>
            </div>
          </>
        }
      />
      {entry.error && <div className="error-msg">{entry.error}</div>}

      {loading ? (
        <div className={styles.roomsLoading}>Загрузка...</div>
      ) : streams.length === 0 ? (
        <div className={styles.roomsEmpty}>
          <p>Потоков пока нет</p>
          <p className={styles.roomsEmptySub}>
            {isAdmin ? 'Создайте первый!' : 'Загляните позже.'}
          </p>
        </div>
      ) : (
        <div data-testid="streams-list">
          <Shelf title="Все потоки">
            {streams.map((stream) => (
              <RoomCard
                key={stream.id}
                room={{
                  id: stream.id,
                  name: stream.name,
                  user_count: stream.user_count,
                  track_count: stream.track_count ?? 0,
                  is_playing: stream.is_playing,
                  has_password: false,
                  auto_radio: true,
                }}
                onOpen={entry.openStreamById}
              />
            ))}
          </Shelf>
        </div>
      )}

      {entry.entryModals}
    </div>
  )
}

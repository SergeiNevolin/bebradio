import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useRoomEntry } from '../hooks/useRoomEntry'
import RoomCard from '../components/RoomCard'
import ScrollRow from '../components/ScrollRow'
import { HeartFillIcon } from '../components/player/icons'
import { type RoomListItem, type Track } from '../types'
import styles from './Home.module.css'

const POPULAR_ROOMS_LIMIT = 8
const POPULAR_TRACKS_LIMIT = 5

export default function Home() {
  const navigate = useNavigate()
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [roomsLoading, setRoomsLoading] = useState(true)
  const [topTracks, setTopTracks] = useState<Track[]>([])
  const [tracksLoading, setTracksLoading] = useState(true)
  const entry = useRoomEntry()

  const fetchRooms = async () => {
    try {
      const rooms = await api.getRooms()
      setRooms(rooms)
    } catch { /* ignore */ }
    setRoomsLoading(false)
  }

  useEffect(() => {
    fetchRooms()
    api.listTracks('', 'top', POPULAR_TRACKS_LIMIT)
      .then(setTopTracks)
      .catch(() => {})
      .finally(() => setTracksLoading(false))
    const interval = setInterval(fetchRooms, 5000)
    return () => clearInterval(interval)
  }, [])

  const stations = rooms
    .filter((r) => r.auto_radio)
    .sort((a, b) => b.user_count - a.user_count)
    .slice(0, POPULAR_ROOMS_LIMIT)

  const liveRooms = rooms
    .filter((r) => !r.auto_radio && (r.user_count > 0 || (r.track_count ?? 0) > 0))
    .sort((a, b) => Number(b.is_playing) - Number(a.is_playing) || b.user_count - a.user_count)
    .slice(0, POPULAR_ROOMS_LIMIT)

  return (
    <div className={styles.home}>
      {entry.error && <div className="error-msg">{entry.error}</div>}

      {/* Станции + Эфир — два ряда в одну строку */}
      <div className={styles.roomsGrid}>
        {stations.length > 0 && (
          <section className={styles.roomsColumn}>
            <div className={styles.homeSectionHeader}>
              <h2 className={styles.homeSectionTitle}>Потоки</h2>
              <Link className={styles.homeSectionLink} to="/rooms">Все →</Link>
            </div>
            <ScrollRow>
              {stations.map((room) => (
                <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
              ))}
            </ScrollRow>
          </section>
        )}

        <section className={styles.roomsColumn}>
          <div className={styles.homeSectionHeader}>
            <h2 className={styles.homeSectionTitle}>Комнаты</h2>
            <Link className={styles.homeSectionLink} to="/rooms">Все →</Link>
          </div>
          {roomsLoading ? (
            <div className={styles.skeletonRow} aria-label="Загрузка комнат">
              {[0, 1, 2, 3].map((i) => (
                <div key={i} className={styles.skeletonCard} />
              ))}
            </div>
          ) : liveRooms.length === 0 ? (
            <div className={styles.homeEmpty}>
              <p>Тихо — эфиров нет</p>
              <p className={styles.homeEmptySub}>Создайте комнату!</p>
              <button className="btn" style={{ marginTop: 8 }} onClick={entry.openCreateModal}>
                Создать комнату
              </button>
            </div>
          ) : (
            <ScrollRow>
              {liveRooms.map((room) => (
                <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
              ))}
            </ScrollRow>
          )}
        </section>
      </div>

      {/* Топ мэшапов — вертикальный чарт */}
      <section className={styles.chartSection}>
        <div className={styles.homeSectionHeader}>
          <h2 className={styles.homeSectionTitle}>Топ мэшапов</h2>
          <Link className={styles.homeSectionLink} to="/mashup">Все мэшапы →</Link>
        </div>
        {tracksLoading ? (
          <div className={styles.chartSkeleton}>
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className={styles.chartSkeletonRow} />
            ))}
          </div>
        ) : topTracks.length === 0 ? (
          <div className={styles.homeEmpty}>
            <p>Мэшапов пока нет</p>
            <p className={styles.homeEmptySub}>Загрузите первый!</p>
          </div>
        ) : (
          <div className={styles.chart}>
            {topTracks.map((track, i) => (
              <button
                key={track.id}
                className={styles.chartRow}
                onClick={() => navigate('/mashup')}
                data-testid="top-track-card"
                data-track-id={track.id}
              >
                <span className={styles.chartPos}>{i + 1}</span>
                <div
                  className={styles.chartArt}
                  style={{ background: track.thumbnail ? undefined : undefined }}
                >
                  {track.thumbnail ? (
                    <img className={styles.chartCover} src={track.thumbnail} alt="" />
                  ) : (
                    <span className={styles.chartGlyph} aria-hidden="true">M</span>
                  )}
                </div>
                <div className={styles.chartInfo}>
                  <div className={styles.chartTitle}>{track.title}</div>
                  <div className={styles.chartArtist}>{track.artist || 'Unknown artist'}</div>
                </div>
                <span className={styles.chartLikes}>
                  <HeartFillIcon size={13} />
                  {track.likes}
                </span>
              </button>
            ))}
          </div>
        )}
      </section>

      {entry.entryModals}
    </div>
  )
}

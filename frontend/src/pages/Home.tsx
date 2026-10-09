import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { useRoomEntry } from '../hooks/useRoomEntry'
import { usePlayer } from '../context/PlayerContext'
import RoomCard from '../components/RoomCard'
import ScrollRow from '../components/ScrollRow'
import { HeartFillIcon, PauseIcon, PlayIcon } from '../components/player/icons'
import { tintForId, monoGlyph } from '../lib/mashupArt'
import type { KaraokePreview } from '../lib/karaokePreview'
import { type RoomListItem, type Track } from '../types'
import styles from './Home.module.css'

const POPULAR_ROOMS_LIMIT = 8
const POPULAR_TRACKS_LIMIT = 5
const KARAOKE_SHELF_LIMIT = 14
const KARAOKE_SHELF_VISIBLE = 8

function sampleEvenly<T>(items: T[], max: number): T[] {
  if (items.length <= max) return items
  const step = items.length / max
  return Array.from({ length: max }, (_, i) => items[Math.floor(i * step)])
}

function formatDuration(sec?: number): string {
  if (!sec || sec <= 0) return ''
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

function plural(n: number, one: string, few: string, many: string): string {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return one
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few
  return many
}

export default function Home() {
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [roomsLoading, setRoomsLoading] = useState(true)
  const [topTracks, setTopTracks] = useState<Track[]>([])
  const [tracksLoading, setTracksLoading] = useState(true)
  const [karaokeSongs, setKaraokeSongs] = useState<KaraokePreview[]>([])
  const entry = useRoomEntry()
  const { player, registerTracks, toggleTrack, applyLike, previewSong, setPreviewSong, setQueueOpen } = usePlayer()

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

  useEffect(() => {
    let alive = true
    fetch('/api/karaoke/api/songs')
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (alive && Array.isArray(data?.songs)) setKaraokeSongs(data.songs)
      })
      .catch(() => { /* ignore */ })
    return () => { alive = false }
  }, [])

  const stations = rooms
    .filter((r) => r.auto_radio)
    .sort((a, b) => b.user_count - a.user_count)
    .slice(0, POPULAR_ROOMS_LIMIT)

  const liveRooms = rooms
    .filter((r) => !r.auto_radio && (r.user_count > 0 || (r.track_count ?? 0) > 0))
    .sort((a, b) => Number(b.is_playing) - Number(a.is_playing) || b.user_count - a.user_count)
    .slice(0, POPULAR_ROOMS_LIMIT)

  useEffect(() => {
    registerTracks(topTracks)
  }, [topTracks, registerTracks])

  const shelfSongs = sampleEvenly(karaokeSongs, KARAOKE_SHELF_LIMIT)

  return (
    <div className={`${styles.home} ${player.current ? styles.homeWithPlayer : ''}`}>
      {entry.error && <div className="error-msg">{entry.error}</div>}

      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>Караоке</h2>
          <div className={styles.sectionHeaderRight}>
            <span className={styles.sectionCount}>
              {karaokeSongs.length}{' '}
              {plural(karaokeSongs.length, 'песня', 'песни', 'песен')}
            </span>
            <Link className={styles.sectionLink} to="/karaoke">Показать все</Link>
          </div>
        </div>
        {shelfSongs.length === 0 ? (
          <div className={styles.homeEmpty}>
            <p>Песен пока нет</p>
            <p className={styles.homeEmptySub}>Загрузите первую!</p>
          </div>
        ) : (
          <ScrollRow>
            {shelfSongs.slice(0, KARAOKE_SHELF_VISIBLE).map((song) => {
              const open = previewSong?.id === song.id
              return (
                <button
                  key={song.id}
                  type="button"
                  onClick={() => {
                    setQueueOpen(false)
                    setPreviewSong(open ? null : song)
                  }}
                  className={`${styles.shelfCard} ${open ? styles.shelfCardActive : ''}`}
                  data-testid="karaoke-song-card"
                  aria-pressed={open}
                  aria-label={`Превью: ${song.title}`}
                  title="Открыть превью и кнопку «Спеть»"
                >
                  <span className={styles.shelfCover} style={{ background: tintForId(song.id) }}>
                    <span className={styles.shelfGlyph} aria-hidden="true">{monoGlyph(song.title)}</span>
                    <span className={styles.shelfPlay} aria-hidden="true">
                      <PlayIcon size={16} />
                    </span>
                  </span>
                  <span className={styles.shelfTitle}>{song.title}</span>
                  <span className={styles.shelfMeta}>
                    {song.language && <span>{song.language.toUpperCase()}</span>}
                    {formatDuration(song.duration) && <span>{formatDuration(song.duration)}</span>}
                  </span>
                </button>
              )
            })}
          </ScrollRow>
        )}
      </section>

      {stations.length > 0 && (
        <section className={styles.section}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>Потоки</h2>
            <Link className={styles.sectionLink} to="/rooms">Показать все</Link>
          </div>
          <ScrollRow>
            {stations.map((room) => (
              <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
            ))}
          </ScrollRow>
        </section>
      )}

      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>Комнаты</h2>
          <Link className={styles.sectionLink} to="/rooms">Показать все</Link>
        </div>
        {roomsLoading ? (
          <div className={styles.shelfSkeleton} aria-label="Загрузка комнат">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className={styles.shelfSkeletonCard} />
            ))}
          </div>
        ) : liveRooms.length === 0 ? (
          <div className={styles.homeEmpty}>
            <p>Тихо — эфиров нет</p>
            <p className={styles.homeEmptySub}>Создайте комнату!</p>
            <button className="btn" style={{ marginTop: 12 }} onClick={entry.openCreateModal}>
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

      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>Мешапы</h2>
          <Link className={styles.sectionLink} to="/mashup">Показать все</Link>
        </div>
        {tracksLoading ? (
          <div className={styles.shelfSkeleton} aria-label="Загрузка мэшапов">
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className={styles.shelfSkeletonCard} />
            ))}
          </div>
        ) : topTracks.length === 0 ? (
          <div className={styles.homeEmpty}>
            <p>Мэшапов пока нет</p>
            <p className={styles.homeEmptySub}>Загрузите первый!</p>
          </div>
        ) : (
          <ScrollRow>
            {topTracks.map((track) => {
              const view = applyLike(track)
              const active = view.id === player.current?.id
              const playing = active && player.isPlaying
              return (
                <button
                  key={track.id}
                  className={`${styles.shelfCard} ${active ? styles.shelfCardActive : ''}`}
                  onClick={() => toggleTrack(view)}
                  data-testid="top-track-card"
                  data-track-id={track.id}
                  aria-pressed={active}
                  aria-label={playing ? `Pause ${view.title}` : `Play ${view.title}`}
                >
                  <span
                    className={styles.shelfCover}
                    style={view.thumbnail ? undefined : { background: tintForId(view.id) }}
                  >
                    {view.thumbnail ? (
                      <img className={styles.shelfImg} src={view.thumbnail} alt="" />
                    ) : (
                      <span className={styles.shelfGlyph} aria-hidden="true">{monoGlyph(view.title)}</span>
                    )}
                    <span className={styles.shelfPlay} aria-hidden="true">
                      {playing ? <PauseIcon size={16} /> : <PlayIcon size={16} />}
                    </span>
                  </span>
                  <span className={styles.shelfTitle}>{view.title}</span>
                  <span className={`${styles.shelfMeta} ${styles.shelfMetaSpread}`}>
                    <span className={styles.shelfArtist}>{view.artist || 'Unknown artist'}</span>
                    <span className={styles.shelfLikes}>
                      <HeartFillIcon size={11} />
                      {view.likes}
                    </span>
                  </span>
                </button>
              )
            })}
          </ScrollRow>
        )}
      </section>

      {entry.entryModals}
    </div>
  )
}

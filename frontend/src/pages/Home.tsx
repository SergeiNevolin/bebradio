import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import { useRoomEntry } from '../hooks/useRoomEntry'
import { usePlayer } from '../context/PlayerContext'
import RoomCard from '../components/RoomCard'
import Shelf from '../components/media/Shelf'
import MediaCard from '../components/media/MediaCard'
import { HeartFillIcon } from '../components/player/icons'
import { tintForId, monoGlyph } from '../lib/mashupArt'
import type { KaraokePreview } from '../lib/karaokePreview'
import { type RoomListItem, type Track } from '../types'
import styles from './Home.module.css'

const POPULAR_ROOMS_LIMIT = 20
const POPULAR_TRACKS_LIMIT = 20
const KARAOKE_SHELF_LIMIT = 28
const KARAOKE_SHELF_VISIBLE = 20

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

      {stations.length > 0 && (
        <Shelf title="Потоки" linkTo="/rooms">
          {stations.map((room) => (
            <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
          ))}
        </Shelf>
      )}

      <Shelf
        title="Караоке"
        count={<>{karaokeSongs.length} {plural(karaokeSongs.length, 'песня', 'песни', 'песен')}</>}
        linkTo="/karaoke"
        empty={
          shelfSongs.length === 0 ? (
            <div className={styles.homeEmpty}>
              <p>Песен пока нет</p>
              <p className={styles.homeEmptySub}>Загрузите первую!</p>
            </div>
          ) : undefined
        }
      >
        {shelfSongs.slice(0, KARAOKE_SHELF_VISIBLE).map((song) => {
          const open = previewSong?.id === song.id
          return (
            <MediaCard
              asButton
              key={song.id}
              title={song.title}
              meta={
                <>
                  {song.language && <span>{song.language.toUpperCase()}</span>}{' '}
                  {formatDuration(song.duration) && <span>{formatDuration(song.duration)}</span>}
                </>
              }
              tint={tintForId(song.id)}
              glyph={monoGlyph(song.title)}
              active={open}
              playing={false}
              playLabel={`Превью: ${song.title}`}
              pressed={open}
              onToggle={() => {
                setQueueOpen(false)
                setPreviewSong(open ? null : song)
              }}
              testId="karaoke-song-card"
            />
          )
        })}
      </Shelf>

      <Shelf
        title="Комнаты"
        linkTo="/rooms"
        loading={roomsLoading}
        skeletonCount={4}
        empty={
          liveRooms.length === 0 ? (
            <div className={styles.homeEmpty}>
              <p>Тихо — эфиров нет</p>
              <p className={styles.homeEmptySub}>Создайте комнату!</p>
              <button className="btn" style={{ marginTop: 12 }} onClick={entry.openCreateModal}>
                Создать комнату
              </button>
            </div>
          ) : undefined
        }
      >
        {liveRooms.map((room) => (
          <RoomCard key={room.id} room={room} onOpen={entry.openRoomById} />
        ))}
      </Shelf>

      <Shelf
        title="Мешапы"
        linkTo="/mashup"
        loading={tracksLoading}
        skeletonCount={5}
        empty={
          topTracks.length === 0 ? (
            <div className={styles.homeEmpty}>
              <p>Мэшапов пока нет</p>
              <p className={styles.homeEmptySub}>Загрузите первый!</p>
            </div>
          ) : undefined
        }
      >
        {topTracks.map((track) => {
          const view = applyLike(track)
          const active = view.id === player.current?.id
          const playing = active && player.isPlaying
          return (
            <MediaCard
              asButton
              key={track.id}
              title={view.title}
              meta={
                <span className={styles.shelfMetaSpread}>
                  <span className={styles.shelfArtist}>{view.artist || 'Unknown artist'}</span>
                  <span className={styles.shelfLikes}>
                    <HeartFillIcon size={11} />
                    {view.likes}
                  </span>
                </span>
              }
              coverUrl={view.thumbnail || undefined}
              tint={tintForId(view.id)}
              glyph={monoGlyph(view.title)}
              active={active}
              playing={playing}
              playLabel={playing ? `Pause ${view.title}` : `Play ${view.title}`}
              pressed={active}
              onToggle={() => toggleTrack(view)}
              testId="top-track-card"
              dataTrackId={track.id}
            />
          )
        })}
      </Shelf>

      {entry.entryModals}
    </div>
  )
}

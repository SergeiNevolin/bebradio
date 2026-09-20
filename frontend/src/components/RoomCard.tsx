import { memo } from 'react'
import type { RoomListItem } from '../types'
import { tintForId, monoGlyph } from '../lib/mashupArt'
import { LockIcon, PlayIcon } from './player/icons'
import styles from './RoomCard.module.css'

interface RoomCardProps {
  room: RoomListItem
  onOpen: (roomId: string) => void
}

function RoomCard({ room, onOpen }: RoomCardProps) {
  return (
    <div
      className={styles.card}
      onClick={() => onOpen(room.id)}
      data-testid="room-card"
      data-room-id={room.id}
    >
      <div className={styles.art} style={{ background: tintForId(room.id) }}>
        <span className={styles.glyph} aria-hidden="true">{monoGlyph(room.name)}</span>
        <div className={styles.badges}>
          {room.is_playing && <span className={styles.live}>Live</span>}
          {room.auto_radio && <span className={styles.station}>24/7</span>}
          {room.has_password && <span className={styles.lock} title="С паролем"><LockIcon size={12} /></span>}
        </div>
        <span className={styles.play} aria-hidden="true"><PlayIcon size={13} /></span>
      </div>
      <div className={styles.body}>
        <div className={styles.name} data-testid="room-card-name">{room.name}</div>
        <div className={styles.meta}>
          <span>{room.user_count} слушают</span>
          <span aria-hidden="true">•</span>
          <span>{room.track_count ?? 0} треков</span>
        </div>
        <div className={styles.code} data-testid="room-card-code">{room.id}</div>
      </div>
    </div>
  )
}

export default memo(RoomCard)

import { memo } from 'react'
import type { Track } from '../types'
import { monoGlyph, tintForId } from '../lib/mashupArt'
import { HeartFillIcon } from './player/icons'
import styles from './TopTrackCard.module.css'

interface TopTrackCardProps {
  track: Track
  position: number
  onOpen: (trackId: string) => void
}

/**
 * Compact discovery card for the home shelf: square cover art, title and
 * artist. Clicking opens the track on the Mashups page, where the full
 * player lives — the card itself carries no playback wiring.
 */
function TopTrackCard({ track, position, onOpen }: TopTrackCardProps) {
  return (
    <div
      className={styles.card}
      onClick={() => onOpen(track.id)}
      data-testid="top-track-card"
      data-track-id={track.id}
      title={`${track.title} — open in Mashups`}
    >
      <div className={styles.art} style={{ background: tintForId(track.id) }}>
        {track.thumbnail ? (
          <img className={styles.cover} src={track.thumbnail} alt="" />
        ) : (
          <span className={styles.glyph} aria-hidden="true">{monoGlyph(track.title)}</span>
        )}
        <span className={styles.pos} aria-hidden="true">{position}</span>
      </div>
      <div className={styles.body}>
        <div className={styles.title}>{track.title}</div>
        <div className={styles.meta}>
          {track.artist || 'Unknown artist'} •{' '}
          <span className={styles.like} aria-hidden="true"><HeartFillIcon size={11} /></span>{' '}
          {track.likes}
        </div>
      </div>
    </div>
  )
}

export default memo(TopTrackCard)

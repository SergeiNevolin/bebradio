import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import styles from './MashupCard.module.css'

interface MashupCardProps {
  mashup: Mashup
  active: boolean
  isPlaying: boolean
  canDelete: boolean
  onPlay: () => void
  onDelete: () => void
}

export default function MashupCard({
  mashup,
  active,
  isPlaying,
  canDelete,
  onPlay,
  onDelete,
}: MashupCardProps) {
  const ready = mashup.status === 'ready'

  return (
    <div className={`${styles.card} ${active ? styles.cardActive : ''}`}>
      <button
        type="button"
        className={styles.art}
        onClick={onPlay}
        disabled={!ready}
        aria-label={ready ? `Play ${mashup.title}` : `${mashup.title} is not ready`}
      >
        {mashup.cover_url ? (
          <img className={styles.cover} src={mashup.cover_url} alt="" />
        ) : (
          <span className={styles.coverFallback} aria-hidden="true">♪</span>
        )}
        {ready && (
          <span className={styles.playIcon}>{active && isPlaying ? '⏸' : '▶'}</span>
        )}
        {mashup.status === 'processing' && (
          <span className={styles.badge}>processing…</span>
        )}
        {mashup.status === 'failed' && (
          <span className={`${styles.badge} ${styles.badgeError}`}>failed</span>
        )}
      </button>

      <div className={styles.body}>
        <div className={styles.title} title={mashup.title}>{mashup.title}</div>
        <div className={styles.artist}>{mashup.artist || 'Unknown artist'}</div>
        <div className={styles.foot}>
          {ready && <span>{formatTime(mashup.duration)}</span>}
          {mashup.status === 'failed' && mashup.error && (
            <span className={styles.errText} title={mashup.error}>{mashup.error}</span>
          )}
        </div>
      </div>

      {canDelete && (
        <button
          type="button"
          className={styles.delete}
          onClick={onDelete}
          aria-label={`Delete ${mashup.title}`}
        >
          ×
        </button>
      )}
    </div>
  )
}

import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import styles from './MashupCard.module.css'

interface MashupCardProps {
  mashup: Mashup
  active: boolean
  isPlaying: boolean
  canEdit: boolean
  onPlay: () => void
  onToggleLike: () => void
  onEdit: () => void
}

export default function MashupCard({
  mashup,
  active,
  isPlaying,
  canEdit,
  onPlay,
  onToggleLike,
  onEdit,
}: MashupCardProps) {
  const ready = mashup.status === 'ready'
  const playing = active && isPlaying

  return (
    <div
      className={`${styles.card} ${active ? styles.cardActive : ''} ${ready ? styles.cardReady : ''}`}
      onClick={ready ? onPlay : undefined}
    >
      <button
        type="button"
        className={styles.art}
        onClick={(e) => {
          e.stopPropagation()
          onPlay()
        }}
        disabled={!ready}
        aria-label={
          !ready
            ? `${mashup.title} is not ready`
            : playing
              ? `Pause ${mashup.title}`
              : `Play ${mashup.title}`
        }
      >
        {mashup.cover_url ? (
          <img className={styles.cover} src={mashup.cover_url} alt="" />
        ) : (
          <span className={styles.coverFallback} aria-hidden="true">♪</span>
        )}
        {ready && (
          <span className={styles.playIcon}>{playing ? '⏸' : '▶'}</span>
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
        {mashup.owner_name && (
          <div className={styles.owner}>by {mashup.owner_name}</div>
        )}
        <div className={styles.foot}>
          <button
            type="button"
            className={`${styles.likeBtn} ${mashup.liked ? styles.likeBtnOn : ''}`}
            onClick={(e) => {
              e.stopPropagation()
              onToggleLike()
            }}
            aria-pressed={!!mashup.liked}
            aria-label={mashup.liked ? `Unlike ${mashup.title}` : `Like ${mashup.title}`}
          >
            <span className={styles.likeIcon} aria-hidden="true">{mashup.liked ? '♥' : '♡'}</span>
            <span>{mashup.likes}</span>
          </button>
          {ready && <span>{formatTime(mashup.duration)}</span>}
          {mashup.status === 'failed' && mashup.error && (
            <span className={styles.errText} title={mashup.error}>{mashup.error}</span>
          )}
          {canEdit && (
            <button
              type="button"
              className={styles.editBtn}
              onClick={(e) => {
                e.stopPropagation()
                onEdit()
              }}
              aria-label={`Edit ${mashup.title}`}
            >
              Edit
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

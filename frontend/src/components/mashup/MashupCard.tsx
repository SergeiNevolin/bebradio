import { useRef } from 'react'
import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import styles from './MashupCard.module.css'

interface MashupCardProps {
  mashup: Mashup
  active: boolean
  isPlaying: boolean
  canDelete: boolean
  canManageCover: boolean
  onPlay: () => void
  onDelete: () => void
  onToggleLike: () => void
  onChangeCover: (file: File) => void
}

export default function MashupCard({
  mashup,
  active,
  isPlaying,
  canDelete,
  canManageCover,
  onPlay,
  onDelete,
  onToggleLike,
  onChangeCover,
}: MashupCardProps) {
  const ready = mashup.status === 'ready'
  const coverInputRef = useRef<HTMLInputElement>(null)

  const pickCover = (file: File | null) => {
    if (file) onChangeCover(file)
  }

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
        {mashup.owner_name && (
          <div className={styles.owner}>by {mashup.owner_name}</div>
        )}
        <div className={styles.foot}>
          <button
            type="button"
            className={`${styles.likeBtn} ${mashup.liked ? styles.likeBtnOn : ''}`}
            onClick={onToggleLike}
            aria-pressed={!!mashup.liked}
            aria-label={mashup.liked ? `Unlike ${mashup.title}` : `Like ${mashup.title}`}
          >
            <span aria-hidden="true">{mashup.liked ? '♥' : '♡'}</span>
            <span>{mashup.likes}</span>
          </button>
          {ready && <span>{formatTime(mashup.duration)}</span>}
          {mashup.status === 'failed' && mashup.error && (
            <span className={styles.errText} title={mashup.error}>{mashup.error}</span>
          )}
        </div>
      </div>

      {canManageCover && (
        <>
          <button
            type="button"
            className={styles.coverBtn}
            onClick={() => coverInputRef.current?.click()}
            aria-label={`Change cover for ${mashup.title}`}
          >
            🖼
          </button>
          <input
            ref={coverInputRef}
            type="file"
            accept="image/*"
            hidden
            onChange={(e) => {
              pickCover(e.target.files?.[0] ?? null)
              e.target.value = ''
            }}
          />
        </>
      )}

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

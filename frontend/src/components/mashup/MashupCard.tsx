import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import styles from './MashupCard.module.css'

interface MashupCardProps {
  mashup: Mashup
  active: boolean
  isPlaying: boolean
  canEdit: boolean
  onPlay: () => void
  onToggleLike: () => void
  onEdit: () => void
  /** Opens the given uploader's profile (in a modal, on the Mashups page). */
  onOpenProfile?: (userId: string) => void
}

export default function MashupCard({
  mashup,
  active,
  isPlaying,
  canEdit,
  onPlay,
  onToggleLike,
  onEdit,
  onOpenProfile,
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
        style={mashup.cover_url ? undefined : { background: tintForId(mashup.id) }}
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
          <span className={styles.glyph} aria-hidden="true">{monoGlyph(mashup.title)}</span>
        )}

        {playing && (
          <span className={styles.eq} aria-hidden="true">
            <i /><i /><i />
          </span>
        )}

        {ready && (
          <span className={styles.fab} aria-hidden="true">{playing ? '⏸' : '▶'}</span>
        )}
      </button>

      <div className={styles.body}>
        <div className={styles.title} title={mashup.title}>{mashup.title}</div>
        <div
          className={styles.meta}
          title={
            mashup.owner_name
              ? `${mashup.artist || 'Unknown artist'} · by ${mashup.owner_name}`
              : mashup.artist || 'Unknown artist'
          }
        >
          {mashup.artist || 'Unknown artist'}
          {mashup.owner_name && mashup.owner_id && onOpenProfile ? (
            <>
              {' · by '}
              <button
                type="button"
                className={styles.ownerLink}
                onClick={(e) => {
                  e.stopPropagation()
                  onOpenProfile(mashup.owner_id)
                }}
              >
                {mashup.owner_name}
              </button>
            </>
          ) : mashup.owner_name ? (
            ` · by ${mashup.owner_name}`
          ) : null}
        </div>
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

          {ready && <span className={styles.dur}>{formatTime(mashup.duration)}</span>}
          {mashup.status === 'processing' && (
            <span className={`${styles.badge} ${styles.badgeWarn}`}>processing…</span>
          )}
          {mashup.status === 'failed' && (
            <span
              className={`${styles.badge} ${styles.badgeError}`}
              title={mashup.error || 'failed'}
            >
              failed
            </span>
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

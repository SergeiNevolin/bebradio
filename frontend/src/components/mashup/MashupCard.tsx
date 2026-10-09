import type { Track } from '../../types'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import { HeartFillIcon, HeartIcon } from '../player/icons'
import MediaCard from '../media/MediaCard'
import styles from './MashupCard.module.css'

interface MashupCardProps {
  mashup: Track
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
    <MediaCard
      title={mashup.title}
      meta={
        <>
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
        </>
      }
      metaTitle={
        mashup.owner_name
          ? `${mashup.artist || 'Unknown artist'} · by ${mashup.owner_name}`
          : mashup.artist || 'Unknown artist'
      }
      coverUrl={mashup.thumbnail || undefined}
      tint={tintForId(mashup.id)}
      glyph={monoGlyph(mashup.title)}
      active={active}
      playing={playing}
      showEq
      playLabel={
        !ready
          ? `${mashup.title} is not ready`
          : playing
            ? `Pause ${mashup.title}`
            : `Play ${mashup.title}`
      }
      onToggle={onPlay}
      disabled={!ready}
      foot={
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
            <span className={styles.likeIcon} aria-hidden="true">{mashup.liked ? <HeartFillIcon size={15} /> : <HeartIcon size={15} />}</span>
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
      }
    />
  )
}

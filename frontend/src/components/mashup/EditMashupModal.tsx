import { useRef } from 'react'
import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import styles from './EditMashupModal.module.css'

interface EditMashupModalProps {
  mashup: Mashup
  canManageCover: boolean
  canDelete: boolean
  onChangeCover: (file: File) => void
  onDelete: () => void
  onClose: () => void
}

/**
 * Settings dialog for a single mashup. Replaces the per-card cover / delete
 * icons: everything an owner can change about a mashup lives here.
 */
export default function EditMashupModal({
  mashup,
  canManageCover,
  canDelete,
  onChangeCover,
  onDelete,
  onClose,
}: EditMashupModalProps) {
  const coverInputRef = useRef<HTMLInputElement>(null)

  const stats = [
    mashup.status === 'ready' ? formatTime(mashup.duration) : mashup.status,
    `${mashup.likes} ${mashup.likes === 1 ? 'like' : 'likes'}`,
    `uploaded ${new Date(mashup.created_at).toLocaleDateString(undefined, {
      day: 'numeric',
      month: 'short',
    })}`,
  ].join(' · ')

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Mashup settings</h3>
          <button className="btn-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="modal-body">
          <div className={styles.summary}>
            {mashup.cover_url ? (
              <img className={styles.cover} src={mashup.cover_url} alt="" />
            ) : (
              <div className={styles.cover} style={{ background: tintForId(mashup.id) }}>
                <span className={styles.glyph} aria-hidden="true">{monoGlyph(mashup.title)}</span>
              </div>
            )}
            <div className={styles.meta}>
              <div className={styles.title} title={mashup.title}>{mashup.title}</div>
              <div className={styles.artist}>
                {mashup.artist || 'Unknown artist'}
                {mashup.owner_name ? ` · by ${mashup.owner_name}` : ''}
              </div>
              <div className={styles.stats}>{stats}</div>
            </div>
          </div>

          {canManageCover && (
            <section className={styles.section}>
              <h4 className={styles.sectionTitle}>Cover</h4>
              <p className={styles.hint}>
                A square PNG or JPG looks best. Up to 5&nbsp;MB; it is re-encoded to 1000&nbsp;px.
              </p>
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => coverInputRef.current?.click()}
              >
                {mashup.cover_url ? 'Replace cover' : 'Upload cover'}
              </button>
              <input
                ref={coverInputRef}
                type="file"
                accept="image/*"
                hidden
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  if (file) onChangeCover(file)
                  e.target.value = ''
                }}
              />
            </section>
          )}

          {canDelete && (
            <section className={styles.dangerBox}>
              <h4 className={styles.sectionTitle}>Delete this mashup</h4>
              <p className={styles.hint}>
                The track, its cover and the transcoded file are removed for good.
                Likes are removed with it.
              </p>
              <button
                type="button"
                className="btn btn-danger"
                onClick={() => {
                  onDelete()
                  onClose()
                }}
              >
                Delete mashup
              </button>
            </section>
          )}

          <div className={styles.footer}>
            <button type="button" className="btn btn-secondary" onClick={onClose}>Done</button>
          </div>
        </div>
      </div>
    </div>
  )
}

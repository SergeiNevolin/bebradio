import { useRef } from 'react'
import type { Mashup } from '../../types'
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
              <div className={styles.coverFallback} aria-hidden="true">♪</div>
            )}
            <div className={styles.meta}>
              <div className={styles.title} title={mashup.title}>{mashup.title}</div>
              <div className={styles.artist}>{mashup.artist || 'Unknown artist'}</div>
            </div>
          </div>

          {canManageCover && (
            <section className={styles.section}>
              <h4 className={styles.sectionTitle}>Cover</h4>
              <p className={styles.hint}>A square PNG or JPG looks best.</p>
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
            <section className={`${styles.section} ${styles.danger}`}>
              <h4 className={styles.sectionTitle}>Delete</h4>
              <p className={styles.hint}>The mashup and its files are removed for good.</p>
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
            <button type="button" className="btn" onClick={onClose}>Done</button>
          </div>
        </div>
      </div>
    </div>
  )
}

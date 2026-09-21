import { useRef, useState } from 'react'
import type { Track } from '../../types'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import styles from './EditMashupModal.module.css'

interface EditMashupModalProps {
  mashup: Track
  canManageCover: boolean
  canEditMeta: boolean
  canDelete: boolean
  onSaveMeta: (title: string, artist: string) => Promise<void>
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
  canEditMeta,
  canDelete,
  onSaveMeta,
  onChangeCover,
  onDelete,
  onClose,
}: EditMashupModalProps) {
  const coverInputRef = useRef<HTMLInputElement>(null)
  const [title, setTitle] = useState(mashup.title)
  const [artist, setArtist] = useState(mashup.artist)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const metaDirty = title !== mashup.title || artist !== mashup.artist

  const handleSaveMeta = async () => {
    if (!metaDirty || saving) return
    setSaving(true)
    try {
      await onSaveMeta(title.trim(), artist.trim())
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } finally {
      setSaving(false)
    }
  }

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
          <h3>Track settings</h3>
          <button className="btn-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="modal-body">
          <div className={styles.summary}>
            {mashup.thumbnail ? (
              <img className={styles.cover} src={mashup.thumbnail} alt="" />
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

          {canEditMeta && (
            <section className={styles.section}>
              <h4 className={styles.sectionTitle}>Metadata</h4>
              <label className={styles.fieldLabel}>
                Title
                <input
                  type="text"
                  className={styles.input}
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  maxLength={200}
                />
              </label>
              <label className={styles.fieldLabel}>
                Artist
                <input
                  type="text"
                  className={styles.input}
                  value={artist}
                  onChange={(e) => setArtist(e.target.value)}
                  maxLength={200}
                />
              </label>
              <div className={styles.metaActions}>
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={!metaDirty || saving}
                  onClick={handleSaveMeta}
                >
                  {saving ? 'Saving…' : saved ? 'Saved' : 'Save'}
                </button>
              </div>
            </section>
          )}

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
                {mashup.thumbnail ? 'Replace cover' : 'Upload cover'}
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

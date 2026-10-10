import { useState } from 'react'
import type { SongMeta } from '../../karaoke/lib/types'
import { tintForId } from '../../lib/mashupArt'
import { MicIcon } from '../player/icons'
import styles from './EditKaraokeModal.module.css'

interface EditKaraokeModalProps {
  song: SongMeta
  onSaveMeta: (title: string, artist: string) => Promise<void>
  onClose: () => void
}

/**
 * Настройки песни караоке: название и автор.
 * Тот же контракт, что у EditMashupModal: форма меты отдельно от редактора
 * (тайминг правится в редакторе караоке, а не здесь).
 */
export default function EditKaraokeModal({ song, onSaveMeta, onClose }: EditKaraokeModalProps) {
  const [title, setTitle] = useState(song.title)
  const [artist, setArtist] = useState(song.artist ?? '')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const metaDirty = title.trim() !== song.title || artist.trim() !== (song.artist ?? '')

  const handleSaveMeta = async () => {
    const nextTitle = title.trim()
    if (!nextTitle || !metaDirty || saving) return
    setSaving(true)
    setError(null)
    try {
      await onSaveMeta(nextTitle, artist.trim())
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось сохранить — попробуйте снова')
    } finally {
      setSaving(false)
    }
  }

  const stats = [
    song.language?.toUpperCase(),
    song.duration ? `${Math.floor(song.duration / 60)}:${String(Math.floor(song.duration % 60)).padStart(2, '0')}` : null,
    song.owner_name ? `Загрузил ${song.owner_name}` : null,
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Настройки караоке">
        <div className="modal-header">
          <h3>Настройки караоке</h3>
          <button className="btn-close" onClick={onClose} aria-label="Закрыть">×</button>
        </div>
        <div className="modal-body">
          <div className={styles.summary}>
            <div className={styles.cover} style={{ background: tintForId(song.id) }}>
              <MicIcon size={28} />
            </div>
            <div className={styles.meta}>
              <div className={styles.title} title={song.title}>{song.title}</div>
              <div className={styles.artist}>{song.artist || 'Автор неизвестен'}</div>
              {stats && <div className={styles.stats}>{stats}</div>}
            </div>
          </div>

          <section className={styles.section}>
            <h4 className={styles.sectionTitle}>Название и автор</h4>
            <label className={styles.fieldLabel}>
              Название
              <input
                type="text"
                className={styles.input}
                aria-label="Название песни"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={200}
              />
            </label>
            <label className={styles.fieldLabel}>
              Автор
              <input
                type="text"
                className={styles.input}
                aria-label="Автор песни"
                value={artist}
                onChange={(e) => setArtist(e.target.value)}
                maxLength={200}
              />
            </label>
            {error && <p className={styles.error}>{error}</p>}
            <div className={styles.metaActions}>
              <button
                type="button"
                className="btn btn-primary"
                disabled={!metaDirty || saving || !title.trim()}
                onClick={handleSaveMeta}
              >
                {saving ? 'Сохранение…' : saved ? 'Сохранено' : 'Сохранить'}
              </button>
            </div>
          </section>

          <div className={styles.footer}>
            <button type="button" className="btn btn-secondary" onClick={onClose}>Готово</button>
          </div>
        </div>
      </div>
    </div>
  )
}

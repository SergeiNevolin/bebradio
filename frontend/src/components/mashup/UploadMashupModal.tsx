import { useRef, useState } from 'react'
import { api } from '../../lib/api'
import { useToast } from '../../context/ToastContext'
import type { Mashup } from '../../types'
import styles from './UploadMashupModal.module.css'

interface UploadMashupModalProps {
  onClose: () => void
  onUploaded: (mashup: Mashup) => void
}

export default function UploadMashupModal({ onClose, onUploaded }: UploadMashupModalProps) {
  const { showToast } = useToast()
  const [file, setFile] = useState<File | null>(null)
  const [title, setTitle] = useState('')
  const [artist, setArtist] = useState('')
  const [progress, setProgress] = useState<number | null>(null)
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const busy = progress !== null

  const pickFile = (f: File | null) => {
    setFile(f)
    setError('')
    if (f && !title.trim()) {
      setTitle(f.name.replace(/\.[^.]+$/, ''))
    }
  }

  const submit = async () => {
    if (!file) {
      setError('Choose an audio file first')
      return
    }
    setProgress(0)
    setError('')
    try {
      const mashup = await api.uploadMashup(
        { file, title: title.trim(), artist: artist.trim() },
        (pct) => setProgress(pct),
      )
      showToast('Upload received, processing…', 'success')
      onUploaded(mashup)
      onClose()
    } catch (err) {
      setProgress(null)
      const message = err instanceof Error ? err.message : 'Upload failed'
      setError(message)
      showToast(message, 'error')
    }
  }

  return (
    <div className="modal-overlay" onClick={busy ? undefined : onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Upload a mashup</h3>
          <button className="btn-close" onClick={onClose} disabled={busy}>×</button>
        </div>
        <div className="modal-body">
          <label className="toggle-label" htmlFor="mashup-file">Audio file</label>
          <input
            id="mashup-file"
            ref={inputRef}
            type="file"
            accept="audio/*"
            disabled={busy}
            onChange={(e) => pickFile(e.target.files?.[0] ?? null)}
            style={{ width: '100%', marginTop: 6, marginBottom: 14 }}
          />

          <label className="toggle-label" htmlFor="mashup-title">Title</label>
          <input
            id="mashup-title"
            type="text"
            placeholder="Track title"
            value={title}
            disabled={busy}
            onChange={(e) => setTitle(e.target.value)}
            style={{ width: '100%', marginTop: 6, marginBottom: 14 }}
          />

          <label className="toggle-label" htmlFor="mashup-artist">Artist</label>
          <input
            id="mashup-artist"
            type="text"
            placeholder="Artist (optional)"
            value={artist}
            disabled={busy}
            onChange={(e) => setArtist(e.target.value)}
            style={{ width: '100%', marginTop: 6 }}
          />

          {progress !== null && (
            <div className={styles.progress} aria-label="Upload progress">
              <div className={styles.progressFill} style={{ width: `${progress}%` }} />
            </div>
          )}

          {error && <div className="error-msg" style={{ marginTop: 12 }}>{error}</div>}

          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <button className="btn" onClick={submit} disabled={busy || !file} style={{ flex: 1 }}>
              {busy ? `Uploading… ${progress}%` : 'Upload'}
            </button>
            <button className="btn btn-secondary" onClick={onClose} disabled={busy}>Cancel</button>
          </div>
        </div>
      </div>
    </div>
  )
}

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'

export default function CreateStreamModal({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleCreate = async () => {
    if (!name.trim()) return
    setLoading(true)
    setError('')
    try {
      const stream = await api.createStream(name.trim())
      onClose()
      navigate(`/stream/${stream.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать поток')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Создать поток</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <label className="toggle-label" htmlFor="create-stream-name">Название потока</label>
          <input
            id="create-stream-name"
            type="text"
            autoFocus
            placeholder="Нонстоп-мешапы"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            style={{ width: '100%', marginTop: 6 }}
          />
          <p style={{ marginTop: 8, fontSize: 13, opacity: 0.7 }}>
            Поток начнёт играть, когда вы включите мешап-радио в настройках или добавите треки вручную.
          </p>
          {error && <div className="error-msg" style={{ marginTop: 12 }}>{error}</div>}
          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <button className="btn" onClick={handleCreate} disabled={loading || !name.trim()} style={{ flex: 1 }}>
              {loading ? 'Создание...' : 'Создать поток'}
            </button>
            <button className="btn btn-secondary" onClick={onClose} disabled={loading}>Отмена</button>
          </div>
        </div>
      </div>
    </div>
  )
}

import { useState } from 'react'
import type { StreamFilter, StreamState } from '../../types'
import { api } from '../../lib/api'

interface StreamSettingsModalProps {
  stream: StreamState
  onUpdate: (patch: { enabled?: boolean; filter?: StreamFilter }) => void
  onDelete: () => void
  onClose: () => void
  deleting: boolean
  deleteError: string
}

export default function StreamSettingsModal({
  stream,
  onUpdate,
  onDelete,
  onClose,
  deleting,
  deleteError,
}: StreamSettingsModalProps) {
  const sf = stream.stream_filter ?? { q: '', title: '', artist: '', owner: '' }
  const [filterQ, setFilterQ] = useState(sf.q ?? '')
  const [filterTitle, setFilterTitle] = useState(sf.title ?? '')
  const [filterArtist, setFilterArtist] = useState(sf.artist ?? '')
  const [filterOwner, setFilterOwner] = useState(sf.owner ?? '')
  const [refillCount, setRefillCount] = useState(3)
  const [refilling, setRefilling] = useState(false)
  const [refillMsg, setRefillMsg] = useState('')
  const filterDirty =
    filterQ.trim() !== (sf.q ?? '') ||
    filterTitle.trim() !== (sf.title ?? '') ||
    filterArtist.trim() !== (sf.artist ?? '') ||
    filterOwner.trim() !== (sf.owner ?? '')

  const handleRandomRefill = async () => {
    setRefilling(true)
    setRefillMsg('')
    try {
      const res = await api.triggerStreamRefill(stream.id, refillCount)
      const n = res.added?.length ?? 0
      setRefillMsg(n > 0 ? `Добавлено: ${n}` : 'Нет подходящих треков')
    } catch (err) {
      setRefillMsg(err instanceof Error ? err.message : 'Не удалось обновить')
    } finally {
      setRefilling(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Настройки потока</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={stream.mashup_radio ?? true}
              onChange={(e) => onUpdate({ enabled: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">
              Мешап-радио (поддерживать очередь из библиотеки)
            </span>
          </label>

          <div data-testid="stream-mashup-settings" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
            <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
              Обновить сейчас
            </div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
              <select
                value={refillCount}
                onChange={(e) => setRefillCount(Number(e.target.value))}
                aria-label="Сколько треков добавить"
              >
                <option value={1}>1 трек</option>
                <option value={3}>3 трека</option>
                <option value={5}>5 треков</option>
                <option value={10}>10 треков</option>
              </select>
              <button className="btn btn-sm" disabled={refilling} onClick={handleRandomRefill}>
                {refilling ? 'Добавление...' : 'Добавить случайные треки'}
              </button>
              {refillMsg && <span style={{ fontSize: 13, opacity: 0.8 }}>{refillMsg}</span>}
            </div>

            <div className="toggle-label" style={{ fontWeight: 600, marginTop: 12, marginBottom: 4 }}>
              Фильтр библиотеки
            </div>
            <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
              Используются только подходящие мешапы. Пустые поля означают любое значение.
            </div>
            <input
              type="text"
              placeholder="Исполнитель"
              value={filterArtist}
              onChange={(e) => setFilterArtist(e.target.value)}
              style={{ width: '100%', marginBottom: 8 }}
              aria-label="Фильтр по исполнителю"
            />
            <input
              type="text"
              placeholder="Название"
              value={filterTitle}
              onChange={(e) => setFilterTitle(e.target.value)}
              style={{ width: '100%', marginBottom: 8 }}
              aria-label="Фильтр по названию"
            />
            <input
              type="text"
              placeholder="Имя загрузившего"
              value={filterOwner}
              onChange={(e) => setFilterOwner(e.target.value)}
              style={{ width: '100%', marginBottom: 8 }}
              aria-label="Фильтр по загрузившему"
            />
            <input
              type="text"
              placeholder="Свободный текст"
              value={filterQ}
              onChange={(e) => setFilterQ(e.target.value)}
              style={{ width: '100%' }}
              aria-label="Фильтр по тексту"
            />
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <button
                className="btn btn-sm"
                disabled={!filterDirty}
                onClick={() => onUpdate({
                  filter: {
                    q: filterQ.trim(),
                    title: filterTitle.trim(),
                    artist: filterArtist.trim(),
                    owner: filterOwner.trim(),
                  },
                })}
              >
                Сохранить фильтр
              </button>
              {(sf.q || sf.title || sf.artist || sf.owner) && (
                <button
                  className="btn btn-sm btn-secondary"
                  onClick={() => {
                    setFilterQ('')
                    setFilterTitle('')
                    setFilterArtist('')
                    setFilterOwner('')
                    onUpdate({ filter: { q: '', title: '', artist: '', owner: '' } })
                  }}
                >
                  Очистить
                </button>
              )}
            </div>
          </div>

          <div className="settings-danger" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
            <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
              Удалить поток
            </div>
            <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
              Удаляет поток, его очередь и чат для всех. Это действие нельзя отменить.
            </div>
            {deleteError && <div className="error-msg" style={{ marginBottom: 8 }}>{deleteError}</div>}
            <button className="btn btn-sm btn-danger" onClick={onDelete} disabled={deleting}>
              {deleting ? 'Удаление...' : 'Удалить этот поток'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

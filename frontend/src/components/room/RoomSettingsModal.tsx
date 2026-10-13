import { useState } from 'react'
import type { RoomState } from '../../types'
import { api } from '../../lib/api'

interface RoomSettingsModalProps {
  room: RoomState
  onUpdate: (settings: {
    allow_anonymous_add?: boolean
    is_private?: boolean
    auto_radio?: boolean
    mashup_radio?: boolean
    password?: string
    stream_filter?: { q?: string; title?: string; artist?: string; owner?: string }
  }) => void
  onDelete: () => void
  onClose: () => void
  settingsPassword: string
  setSettingsPassword: (v: string) => void
  deleting: boolean
  deleteError: string
}

export default function RoomSettingsModal({
  room,
  onUpdate,
  onDelete,
  onClose,
  settingsPassword,
  setSettingsPassword,
  deleting,
  deleteError,
}: RoomSettingsModalProps) {
  const sf = room.stream_filter ?? { q: '', title: '', artist: '', owner: '' }
  const [filterQ, setFilterQ] = useState(sf.q ?? '')
  const [filterTitle, setFilterTitle] = useState(sf.title ?? '')
  const [filterArtist, setFilterArtist] = useState(sf.artist ?? '')
  const [filterOwner, setFilterOwner] = useState(sf.owner ?? '')
  const [refillCount, setRefillCount] = useState(3)
  const [refilling, setRefilling] = useState(false)
  const [refillMsg, setRefillMsg] = useState('')
  const filterDirty =
    (filterQ ?? '') !== (sf.q ?? '') ||
    (filterTitle ?? '') !== (sf.title ?? '') ||
    (filterArtist ?? '') !== (sf.artist ?? '') ||
    (filterOwner ?? '') !== (sf.owner ?? '')

  const handleRandomRefill = async () => {
    setRefilling(true)
    setRefillMsg('')
    try {
      const res = await api.triggerStreamRefill(room.id, refillCount)
      const n = res.added?.length ?? 0
      setRefillMsg(n > 0 ? `Добавлено: ${n}` : 'Ничего не подошло под фильтр')
    } catch (err) {
      setRefillMsg(err instanceof Error ? err.message : 'Не вышло добавить')
    } finally {
      setRefilling(false)
    }
  }
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Room Settings</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={room?.allow_anonymous_add ?? true}
              onChange={(e) => onUpdate({ allow_anonymous_add: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">Allow anonymous users to add tracks</span>
          </label>
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={room?.is_private ?? false}
              onChange={(e) => onUpdate({ is_private: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">Private room (hidden from public list)</span>
          </label>
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={room?.auto_radio ?? false}
              onChange={(e) => onUpdate({ auto_radio: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">Auto-radio (keep playing related tracks when the queue runs out)</span>
          </label>

          {room?.is_stream && (
            <div data-testid="stream-mashup-settings" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
              <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
                Добавление мешапов
              </div>
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={room?.mashup_radio ?? true}
                  onChange={(e) => onUpdate({ mashup_radio: e.target.checked })}
                />
                <span className="toggle-slider"></span>
                <span className="toggle-label">Автодобавление случайных мешапов (очередь пополняется сама, даже без слушателей)</span>
              </label>
              <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4, marginTop: 8 }}>
                Добавить рандомные мешапы
              </div>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                <select
                  value={refillCount}
                  onChange={(e) => setRefillCount(Number(e.target.value))}
                  aria-label="Сколько случайных мешапов добавить"
                >
                  <option value={1}>1 трек</option>
                  <option value={3}>3 трека</option>
                  <option value={5}>5 треков</option>
                  <option value={10}>10 треков</option>
                </select>
                <button
                  className="btn btn-sm"
                  disabled={refilling}
                  onClick={handleRandomRefill}
                >
                  {refilling ? 'Добавляю…' : 'Добавить случайные'}
                </button>
                {refillMsg && <span style={{ fontSize: 13, opacity: 0.8 }}>{refillMsg}</span>}
              </div>
              <div className="toggle-label" style={{ fontWeight: 600, marginTop: 12, marginBottom: 4 }}>
                Фильтр отбора
              </div>
              <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
                Какие мешапы крутить в эфире: по названию, автору и загрузчику. Пусто — вся база.
              </div>
              <input
                type="text"
                placeholder="Автор (artist)…"
                value={filterArtist}
                onChange={(e) => setFilterArtist(e.target.value)}
                style={{ width: '100%', marginBottom: 8 }}
                aria-label="Фильтр потока по автору"
              />
              <input
                type="text"
                placeholder="Название…"
                value={filterTitle}
                onChange={(e) => setFilterTitle(e.target.value)}
                style={{ width: '100%', marginBottom: 8 }}
                aria-label="Фильтр потока по названию"
              />
              <input
                type="text"
                placeholder="Загрузчик (username)…"
                value={filterOwner}
                onChange={(e) => setFilterOwner(e.target.value)}
                style={{ width: '100%', marginBottom: 8 }}
                aria-label="Фильтр потока по загрузчику"
              />
              <input
                type="text"
                placeholder="Общий поиск…"
                value={filterQ}
                onChange={(e) => setFilterQ(e.target.value)}
                style={{ width: '100%' }}
                aria-label="Общий фильтр потока"
              />
              <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                <button
                  className="btn btn-sm"
                  disabled={!filterDirty}
                  onClick={() => onUpdate({ stream_filter: { q: filterQ.trim(), title: filterTitle.trim(), artist: filterArtist.trim(), owner: filterOwner.trim() } })}
                >
                  Применить фильтр
                </button>
                {(sf.q || sf.title || sf.artist || sf.owner) && (
                  <button
                    className="btn btn-sm btn-secondary"
                    onClick={() => {
                      setFilterQ('')
                      setFilterTitle('')
                      setFilterArtist('')
                      setFilterOwner('')
                      onUpdate({ stream_filter: { q: '', title: '', artist: '', owner: '' } })
                    }}
                  >
                    Сбросить
                  </button>
                )}
              </div>
            </div>
          )}

          <div className="settings-password" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
            <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
              Room password
            </div>
            <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
              {room?.has_password
                ? 'This room is password protected.'
                : 'Anyone with the code can join.'}
            </div>
            <input
              type="password"
              placeholder={room?.has_password ? 'New password' : 'Set a password'}
              value={settingsPassword}
              onChange={(e) => setSettingsPassword(e.target.value)}
              style={{ width: '100%' }}
            />
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <button
                className="btn btn-sm"
                disabled={!settingsPassword.trim()}
                onClick={() => onUpdate({ password: settingsPassword.trim() })}
              >
                {room?.has_password ? 'Change password' : 'Set password'}
              </button>
              {room?.has_password && (
                <button
                  className="btn btn-sm btn-secondary"
                  onClick={() => onUpdate({ password: '' })}
                >
                  Remove password
                </button>
              )}
            </div>
          </div>

          <div className="settings-danger" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
            <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
              Delete room
            </div>
            <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
              Removes the room, its queue and chat for everyone. This cannot be undone.
            </div>
            {deleteError && <div className="error-msg" style={{ marginBottom: 8 }}>{deleteError}</div>}
            <button
              className="btn btn-sm btn-danger"
              onClick={onDelete}
              disabled={deleting}
            >
              {deleting ? 'Deleting...' : 'Delete this room'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

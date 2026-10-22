import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'
import { setRoomAccess } from '../../lib/roomAccess'

export default function CreateRoomModal({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate()
  const [roomName, setRoomName] = useState('')
  const [roomPassword, setRoomPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleCreate = async () => {
    if (!roomName.trim()) return
    setLoading(true)
    setError('')
    try {
      const data = await api.createRoom(roomName.trim(), roomPassword.trim() || undefined)
      if (data.access) setRoomAccess(data.id, data.access)
      onClose()
      navigate(`/room/${data.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create room')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Create a room</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <label className="toggle-label" htmlFor="create-room-name">Room name</label>
          <input
            id="create-room-name"
            type="text"
            autoFocus
            placeholder="Room name"
            value={roomName}
            onChange={(e) => setRoomName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            style={{ width: '100%', marginTop: 6, marginBottom: 14 }}
          />
          <label className="toggle-label" htmlFor="create-room-password">Password (optional)</label>
          <input
            id="create-room-password"
            type="password"
            placeholder="Leave empty for an open room"
            value={roomPassword}
            onChange={(e) => setRoomPassword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            style={{ width: '100%', marginTop: 6 }}
          />
          <p style={{ marginTop: 8, fontSize: 13, opacity: 0.7 }}>
            With a password, listeners must enter it before they can open the room.
          </p>
          {error && <div className="error-msg" style={{ marginTop: 12 }}>{error}</div>}
          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <button className="btn" onClick={handleCreate} disabled={loading || !roomName.trim()} style={{ flex: 1 }}>
              {loading ? 'Creating...' : 'Create room'}
            </button>
            <button className="btn btn-secondary" onClick={onClose} disabled={loading}>Cancel</button>
          </div>
        </div>
      </div>
    </div>
  )
}

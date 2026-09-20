import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'

interface JoinRoomModalProps {
  onClose: () => void
  /** Called with the room code when it turns out to be password protected. */
  onLocked: (code: string) => void
}

export default function JoinRoomModal({ onClose, onLocked }: JoinRoomModalProps) {
  const navigate = useNavigate()
  const [joinCode, setJoinCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleJoin = async () => {
    const code = joinCode.trim().toUpperCase()
    if (!code) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getRoomByCode(code)
      if ((data as Record<string, unknown>).locked) {
        onLocked(code)
        return
      }
      onClose()
      navigate(`/room/${code}`)
    } catch {
      setError('Room not found')
    } finally {
      setLoading(false)
    }
  }

  const close = () => {
    setJoinCode('')
    onClose()
  }

  return (
    <div className="modal-overlay" onClick={close}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Join a room</h3>
          <button className="btn-close" onClick={close}>×</button>
        </div>
        <div className="modal-body">
          <label className="toggle-label" htmlFor="join-room-code">Room code</label>
          <input
            id="join-room-code"
            type="text"
            autoFocus
            placeholder="e.g. ABC123"
            value={joinCode}
            onChange={(e) => setJoinCode(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleJoin()}
            style={{ width: '100%', marginTop: 6, textTransform: 'uppercase', letterSpacing: 2 }}
          />
          {error && <div className="error-msg" style={{ marginTop: 12 }}>{error}</div>}
          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <button className="btn" onClick={() => handleJoin()} disabled={loading || !joinCode.trim()} style={{ flex: 1 }}>
              {loading ? 'Joining...' : 'Join room'}
            </button>
            <button className="btn btn-secondary" onClick={close} disabled={loading}>Cancel</button>
          </div>
        </div>
      </div>
    </div>
  )
}

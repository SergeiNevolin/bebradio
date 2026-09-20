import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'
import { setRoomAccess } from '../../lib/roomAccess'

export default function RoomPasswordModal({ code, onClose }: { code: string; onClose: () => void }) {
  const navigate = useNavigate()
  const [promptPassword, setPromptPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const submitPassword = async () => {
    if (!code || !promptPassword) return
    setLoading(true)
    setError('')
    try {
      const data = await api.joinRoom(code, promptPassword)
      setRoomAccess(code, data.access)
      onClose()
      navigate(`/room/${code}`)
    } catch {
      setError('Could not join room')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Password required</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <p style={{ marginBottom: 12, fontSize: 14 }}>
            Room <strong>{code}</strong> is password protected.
          </p>
          <input
            type="password"
            autoFocus
            placeholder="Room password"
            value={promptPassword}
            onChange={(e) => setPromptPassword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && submitPassword()}
            style={{ width: '100%' }}
          />
          {error && <div className="error-msg" style={{ marginTop: 8 }}>{error}</div>}
          <button className="btn" onClick={submitPassword} disabled={loading || !promptPassword} style={{ marginTop: 12, width: '100%' }}>
            Enter room
          </button>
        </div>
      </div>
    </div>
  )
}

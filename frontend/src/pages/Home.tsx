import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { setRoomAccess } from '../lib/roomAccess'

interface RoomListItem {
  id: string
  name: string
  user_count: number
  track_count: number
  is_playing: boolean
  has_password: boolean
}

export default function Home() {
  const { authHeaders } = useAuth()
  const [roomName, setRoomName] = useState('')
  const [roomPassword, setRoomPassword] = useState('')
  const [joinCode, setJoinCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [roomsLoading, setRoomsLoading] = useState(true)
  const [passwordPrompt, setPasswordPrompt] = useState<string | null>(null)
  const [promptPassword, setPromptPassword] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const navigate = useNavigate()

  const openCreate = () => {
    setError('')
    setRoomPassword('')
    setShowCreate(true)
  }

  const closeCreate = () => {
    setShowCreate(false)
    setRoomPassword('')
  }

  const fetchRooms = async () => {
    try {
      const res = await fetch('/api/rooms')
      const data = await res.json()
      setRooms(data)
    } catch { /* ignore */ }
    setRoomsLoading(false)
  }

  useEffect(() => {
    fetchRooms()
    const interval = setInterval(fetchRooms, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleCreate = async () => {
    if (!roomName.trim()) return
    setLoading(true)
    setError('')
    try {
      const res = await fetch('/api/rooms', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({
          name: roomName.trim(),
          password: roomPassword.trim() || null,
        }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Failed to create room')
      }
      const data = await res.json()
      if (data.access) setRoomAccess(data.id, data.access)
      setShowCreate(false)
      navigate(`/room/${data.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create room')
    } finally {
      setLoading(false)
    }
  }

  const handleJoin = async (roomId?: string) => {
    const code = (roomId || joinCode).trim().toUpperCase()
    if (!code) return
    setLoading(true)
    setError('')
    try {
      const res = await fetch(`/api/rooms/${code}`)
      if (!res.ok) throw new Error()
      const data = await res.json()
      if (data.locked) {
        setPasswordPrompt(code)
        setPromptPassword('')
        return
      }
      navigate(`/room/${code}`)
    } catch {
      setError('Room not found')
    } finally {
      setLoading(false)
    }
  }

  const submitPassword = async () => {
    if (!passwordPrompt || !promptPassword) return
    setLoading(true)
    setError('')
    try {
      const res = await fetch(`/api/rooms/${passwordPrompt}/join`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: promptPassword }),
      })
      if (!res.ok) {
        setError('Incorrect room password')
        return
      }
      const data = await res.json()
      setRoomAccess(passwordPrompt, data.access)
      const code = passwordPrompt
      setPasswordPrompt(null)
      navigate(`/room/${code}`)
    } catch {
      setError('Could not join room')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="home">
      <h1 className="home-title">bebradio</h1>
      <p className="home-subtitle">listen together</p>

      <div className="home-actions">
        <input
          type="text"
          placeholder="Room name"
          value={roomName}
          onChange={(e) => setRoomName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && roomName.trim() && openCreate()}
        />
        <button className="btn" onClick={openCreate} disabled={loading || !roomName.trim()}>
          Create
        </button>
      </div>

      <div className="home-actions">
        <input
          type="text"
          placeholder="Enter room code"
          value={joinCode}
          onChange={(e) => setJoinCode(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleJoin()}
          style={{ textTransform: 'uppercase', letterSpacing: 2 }}
        />
        <button className="btn btn-secondary" onClick={() => handleJoin()} disabled={loading}>
          Join
        </button>
      </div>

      {error && !showCreate && !passwordPrompt && <div className="error-msg">{error}</div>}

      {showCreate && (
        <div className="modal-overlay" onClick={closeCreate}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>Create a room</h3>
              <button className="btn-close" onClick={closeCreate}>×</button>
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
                <button
                  className="btn"
                  onClick={handleCreate}
                  disabled={loading || !roomName.trim()}
                  style={{ flex: 1 }}
                >
                  {loading ? 'Creating...' : 'Create room'}
                </button>
                <button className="btn btn-secondary" onClick={closeCreate} disabled={loading}>
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {passwordPrompt && (
        <div className="modal-overlay" onClick={() => setPasswordPrompt(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>Password required</h3>
              <button className="btn-close" onClick={() => setPasswordPrompt(null)}>×</button>
            </div>
            <div className="modal-body">
              <p style={{ marginBottom: 12, fontSize: 14 }}>
                Room <strong>{passwordPrompt}</strong> is password protected.
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
              <button
                className="btn"
                onClick={submitPassword}
                disabled={loading || !promptPassword}
                style={{ marginTop: 12, width: '100%' }}
              >
                Enter room
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="room-list-section">
        <h2>Active Rooms</h2>
        {roomsLoading ? (
          <div className="room-list-empty">Loading...</div>
        ) : rooms.length === 0 ? (
          <div className="room-list-empty">No active rooms yet</div>
        ) : (
          <div className="room-list">
            {rooms.map((room) => (
              <div key={room.id} className="room-list-item" onClick={() => handleJoin(room.id)}>
                <div className="room-list-item-info">
                  <div className="room-list-item-name">
                    {room.has_password && <span title="Password protected">🔒 </span>}
                    {room.name}
                  </div>
                  <div className="room-list-item-meta">
                    <span className="room-list-item-code">{room.id}</span>
                    {room.is_playing && <span className="room-list-item-playing">LIVE</span>}
                  </div>
                </div>
                <div className="room-list-item-stats">
                  <span>{room.track_count} tracks</span>
                  <span>{room.user_count} listening</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

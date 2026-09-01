import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

interface RoomListItem {
  id: string
  name: string
  user_count: number
  track_count: number
  is_playing: boolean
}

export default function Home() {
  const { authHeaders } = useAuth()
  const [roomName, setRoomName] = useState('')
  const [joinCode, setJoinCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [roomsLoading, setRoomsLoading] = useState(true)
  const navigate = useNavigate()

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
        body: JSON.stringify({ name: roomName.trim() }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Failed to create room')
      }
      const data = await res.json()
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
      navigate(`/room/${code}`)
    } catch {
      setError('Room not found')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="home">
      <h1>Listen <span>Together</span></h1>
      <p>Listen to music together with friends in real-time</p>

      <div className="home-actions">
        <input
          type="text"
          placeholder="Room name"
          value={roomName}
          onChange={(e) => setRoomName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
        <button className="btn" onClick={handleCreate} disabled={loading}>
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

      {error && <div className="error-msg">{error}</div>}

      <div className="room-list-section">
        <h2>Active Rooms</h2>
        {roomsLoading ? (
          <div className="room-list-empty">Loading...</div>
        ) : rooms.length === 0 ? (
          <div className="room-list-empty">No active rooms yet. Create one above.</div>
        ) : (
          <div className="room-list">
            {rooms.map((room) => (
              <div key={room.id} className="room-list-item" onClick={() => handleJoin(room.id)}>
                <div className="room-list-item-info">
                  <div className="room-list-item-name">{room.name}</div>
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

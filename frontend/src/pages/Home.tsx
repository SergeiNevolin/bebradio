import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { setRoomAccess } from '../lib/roomAccess'
import { type RoomListItem } from '../types'
import ScrollRow from '../components/ScrollRow'
import styles from './Home.module.css'

export default function Home() {
  const { user } = useAuth()
  const [roomName, setRoomName] = useState('')
  const [roomPassword, setRoomPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [rooms, setRooms] = useState<RoomListItem[]>([])
  const [roomsLoading, setRoomsLoading] = useState(true)
  const [passwordPrompt, setPasswordPrompt] = useState<string | null>(null)
  const [promptPassword, setPromptPassword] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [showJoin, setShowJoin] = useState(false)
  const [joinCode, setJoinCode] = useState('')
  const [recentRooms, setRecentRooms] = useState<RoomListItem[]>([])
  const navigate = useNavigate()

  const closeCreate = () => {
    setShowCreate(false)
    setRoomPassword('')
  }

  const fetchRooms = async () => {
    try {
      const rooms = await api.getRooms()
      setRooms(rooms)
    } catch { /* ignore */ }
    setRoomsLoading(false)
  }

  const fetchRecent = useCallback(async () => {
    if (!user) return
    try {
      const rooms = await api.getRecentRooms()
      setRecentRooms(rooms)
    } catch { /* ignore */ }
  }, [user])

  useEffect(() => {
    fetchRooms()
    fetchRecent()
    const interval = setInterval(fetchRooms, 5000)
    return () => clearInterval(interval)
  }, [fetchRecent])

  const handleCreate = async () => {
    if (!roomName.trim()) return
    setLoading(true)
    setError('')
    try {
      const data = await api.createRoom(roomName.trim(), roomPassword.trim() || undefined)
      if (data.access) setRoomAccess(data.id, data.access)
      closeCreate()
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
      const data = await api.getRoomByCode(code)
      if ((data as Record<string, unknown>).locked) {
        setPasswordPrompt(code)
        setPromptPassword('')
        setShowJoin(false)
        return
      }
      setShowJoin(false)
      setJoinCode('')
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
      const data = await api.joinRoom(passwordPrompt, promptPassword)
      setRoomAccess(passwordPrompt, data.access)
      setPasswordPrompt(null)
      navigate(`/room/${passwordPrompt}`)
    } catch {
      setError('Could not join room')
    } finally {
      setLoading(false)
    }
  }

  const totalListeners = rooms.reduce((sum, r) => sum + r.user_count, 0)
  const activeRooms = rooms.filter((r) => r.user_count > 0 || (r.track_count ?? 0) > 0)
  const idleRooms = rooms.filter((r) => r.user_count === 0 && (r.track_count ?? 0) === 0)

  const renderCard = (room: RoomListItem) => (
    <div key={room.id} className={styles.homeCard} onClick={() => handleJoin(room.id)}>
      <div className={styles.homeCardIndicator}>
        {room.is_playing && <span className={styles.homeCardLive}>LIVE</span>}
        {room.has_password && <span className={styles.homeCardLock}>🔒</span>}
      </div>
      <div className={styles.homeCardBody}>
        <div className={styles.homeCardName}>{room.name}</div>
        <div className={styles.homeCardId}>{room.id}</div>
      </div>
      <div className={styles.homeCardFooter}>
        <span className={styles.homeCardStat}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
          {room.user_count}
        </span>
        <span className={styles.homeCardStat}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>
          {room.track_count}
        </span>
      </div>
    </div>
  )

  return (
    <div className={styles.home}>
      {/* Hero */}
      <div className={styles.homeHero}>
        <div className={styles.homeHeroContent}>
          <h1 className={styles.homeHeroTitle}>Слушать музыку вместе с друзьями</h1>
          <p className={styles.homeHeroSub}>Создайте музыкальную комнату и слушайте треки вместе онлайн в синхронном режиме.</p>
          <div className={styles.homeHeroActions}>
            <button className={`btn ${styles.btnHero}`} onClick={() => setShowCreate(true)} disabled={loading}>
              Create Room
            </button>
            <button className={`btn ${styles.btnHero} btn-secondary`} onClick={() => setShowJoin(true)} disabled={loading}>
              Join by Code
            </button>
          </div>
          {error && !showCreate && !passwordPrompt && <div className="error-msg" style={{ marginTop: 12 }}>{error}</div>}
        </div>
        <div className={styles.homeHeroStats}>
          <div className={styles.homeHeroStat}>
            <span className={styles.homeHeroStatNum}>{rooms.length}</span>
            <span className={styles.homeHeroStatLabel}>rooms</span>
          </div>
          <div className={styles.homeHeroStat}>
            <span className={styles.homeHeroStatNum}>{totalListeners}</span>
            <span className={styles.homeHeroStatLabel}>listening</span>
          </div>
        </div>
      </div>

      {/* Recently Played */}
      {user && recentRooms.length > 0 && (
        <section className={styles.homeSection}>
          <div className={styles.homeSectionHeader}>
            <h2 className={styles.homeSectionTitle}>Recently Played</h2>
          </div>
          <ScrollRow>
            {recentRooms.map(renderCard)}
          </ScrollRow>
        </section>
      )}

      {/* Top Rooms */}
      <section className={styles.homeSection}>
        <div className={styles.homeSectionHeader}>
          <h2 className={styles.homeSectionTitle}>Top Rooms</h2>
        </div>
        {roomsLoading ? (
          <div className={styles.homeLoading}>Loading...</div>
        ) : activeRooms.length === 0 && idleRooms.length === 0 ? (
          <div className={styles.homeEmpty}>
            <p>No rooms yet</p>
            <p className={styles.homeEmptySub}>Create the first one!</p>
          </div>
        ) : (
          <>
            {activeRooms.length > 0 && (
              <ScrollRow>
                {activeRooms.map(renderCard)}
              </ScrollRow>
            )}
            {idleRooms.length > 0 && (
              <>
                <div className={styles.homeSectionHeader} style={{ marginTop: 24 }}>
                  <h2 className={styles.homeSectionTitle} style={{ fontSize: 14, color: 'var(--text-muted)' }}>Waiting for listeners</h2>
                </div>
                <ScrollRow>
                  {idleRooms.map(renderCard)}
                </ScrollRow>
              </>
            )}
          </>
        )}
      </section>

      {/* Modals */}
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
                <button className="btn" onClick={handleCreate} disabled={loading || !roomName.trim()} style={{ flex: 1 }}>
                  {loading ? 'Creating...' : 'Create room'}
                </button>
                <button className="btn btn-secondary" onClick={closeCreate} disabled={loading}>Cancel</button>
              </div>
            </div>
          </div>
        </div>
      )}

      {showJoin && (
        <div className="modal-overlay" onClick={() => { setShowJoin(false); setJoinCode('') }}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>Join a room</h3>
              <button className="btn-close" onClick={() => { setShowJoin(false); setJoinCode('') }}>×</button>
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
                <button className="btn btn-secondary" onClick={() => { setShowJoin(false); setJoinCode('') }} disabled={loading}>Cancel</button>
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
              {error && <div className="error-msg" style={{ marginTop: 8 }}>{error}</div>}
              <button className="btn" onClick={submitPassword} disabled={loading || !promptPassword} style={{ marginTop: 12, width: '100%' }}>
                Enter room
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

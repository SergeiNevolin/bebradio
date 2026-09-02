import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { getRoomAccess, setRoomAccess, clearRoomAccess } from '../lib/roomAccess'
import Player from '../components/Player'
import Queue from '../components/Queue'
import AddTrack from '../components/AddTrack'
import Chat, { type ChatMessage } from '../components/Chat'
import Listeners from '../components/Listeners'
import { ReactionBar, ReactionsOverlay, type FloatingReaction } from '../components/Reactions'

import type { RoomState } from '../types'

export default function Room() {
  const { roomId } = useParams<{ roomId: string }>()
  const navigate = useNavigate()
  const { user, token, authHeaders } = useAuth()
  const [room, setRoom] = useState<RoomState | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [locked, setLocked] = useState(false)
  const [passwordInput, setPasswordInput] = useState('')
  const [unlocking, setUnlocking] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [settingsPassword, setSettingsPassword] = useState('')
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [copied, setCopied] = useState(false)
  const [userVote, setUserVote] = useState<1 | -1 | 0>(0)
  const [reactions, setReactions] = useState<FloatingReaction[]>([])
  const prevTrackId = useRef<string | null>(null)

  const identity = useCallback(() => ({
    user_id: user?.id || '',
    username: user?.username || 'Anonymous',
  }), [user?.id, user?.username])

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const unmountedRef = useRef(false)

  const isOwner = user && room && user.id === room.owner_id
  const canAddTrack = user || room?.allow_anonymous_add

  const connectWs = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const access = getRoomAccess(roomId!)
    const query = access ? `?access=${encodeURIComponent(access)}` : ''
    const wsUrl = `${protocol}//${window.location.host}/ws/${roomId}${query}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ action: 'hello', ...identity() }))
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.error && data.locked) {
          setLocked(true)
          ws.close()
          return
        }
        if (data.type === 'room_deleted') {
          unmountedRef.current = true
          ws.close()
          clearRoomAccess(roomId!)
          navigate('/', { replace: true })
          return
        }
        if (data.type === 'reaction') {
          const item: FloatingReaction = {
            key: `${data.id}-${Math.random().toString(36).slice(2)}`,
            emoji: data.emoji,
            username: data.username,
            left: 8 + Math.random() * 78,
          }
          setReactions((prev) => [...prev, item])
          setTimeout(() => {
            setReactions((prev) => prev.filter((r) => r.key !== item.key))
          }, 3800)
          return
        }
        if (data.type === 'chat') {
          setChatMessages((prev) => [...prev.slice(-99), data.message])
        } else {
          setRoom(data as RoomState)
          if (data.messages) {
            setChatMessages(data.messages)
          }
        }
      } catch { /* ignore */ }
    }

    ws.onclose = () => {
      if (unmountedRef.current) return
      reconnectTimer.current = setTimeout(connectWs, 2000)
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [roomId, navigate, identity])

  const fetchRoom = useCallback(async (): Promise<RoomState | null> => {
    try {
      const access = getRoomAccess(roomId!)
      const query = access ? `?access=${encodeURIComponent(access)}` : ''
      const headers: Record<string, string> = token ? { Authorization: `Bearer ${token}` } : {}
      const res = await fetch(`/api/rooms/${roomId}${query}`, { headers })
      if (!res.ok) throw new Error()
      const data = await res.json() as RoomState
      if (data.locked) {
        setRoom(data as RoomState)
        setLocked(true)
        return null
      }
      if (data.access) setRoomAccess(roomId!, data.access)
      setLocked(false)
      setRoom(data)
      return data
    } catch {
      setError('Room not found')
      return null
    } finally {
      setLoading(false)
    }
  }, [roomId, token])

  useEffect(() => {
    unmountedRef.current = false
    let cancelled = false
    fetchRoom().then((data) => {
      if (!cancelled && data) connectWs()
    })

    return () => {
      cancelled = true
      unmountedRef.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [roomId, fetchRoom, connectWs])

  const sendWs = (msg: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }

  const handleUnlock = async () => {
    if (!passwordInput) return
    setUnlocking(true)
    setError('')
    try {
      const res = await fetch(`/api/rooms/${roomId}/join`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: passwordInput }),
      })
      if (!res.ok) {
        setError('Incorrect room password')
        return
      }
      const data = await res.json()
      setRoomAccess(roomId!, data.access)
      setPasswordInput('')
      setLocked(false)
      setLoading(true)
      const fresh = await fetchRoom()
      if (fresh) connectWs()
    } catch {
      setError('Could not join room')
    } finally {
      setUnlocking(false)
    }
  }

  const handleAddTrack = async (url: string): Promise<{ success: boolean; error?: string }> => {
    try {
      const access = getRoomAccess(roomId!)
      const query = access ? `?access=${encodeURIComponent(access)}` : ''
      const res = await fetch(`/api/rooms/${roomId}/queue${query}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ url }),
      })
      if (!res.ok) {
        const data = await res.json() as { error?: string }
        throw new Error(data.error || 'Failed to add track')
      }
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Unknown error' }
    }
  }

  const handlePlayback = (action: string, extra: Record<string, unknown> = {}) => {
    sendWs({ action, ...extra })
  }

  const handleCopyCode = () => {
    navigator.clipboard.writeText(roomId!)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleShare = async () => {
    const url = `${window.location.origin}/room/${roomId}`
    if (navigator.share) {
      await navigator.share({ title: room?.name, url })
    } else {
      await navigator.clipboard.writeText(url)
    }
  }

  const handleUpdateSettings = async (settings: {
    allow_anonymous_add?: boolean
    is_private?: boolean
    auto_radio?: boolean
    password?: string
  }) => {
    if (settings.password === undefined) {
      setRoom((prev) => prev ? { ...prev, ...settings } : prev)
    }
    try {
      const res = await fetch(`/api/rooms/${roomId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify(settings),
      })
      if (res.ok) {
        const data = await res.json() as RoomState
        setRoom((prev) => prev ? { ...prev, ...data } : data)
        if ('password' in settings) {
          // Owner keeps access; drop any stale token when the password is removed.
          if (!settings.password) clearRoomAccess(roomId!)
          setSettingsPassword('')
        }
      }
    } catch { /* ignore */ }
  }

  const handleDeleteRoom = async () => {
    if (!window.confirm('Delete this room for everyone? This cannot be undone.')) return
    setDeleting(true)
    try {
      const res = await fetch(`/api/rooms/${roomId}`, {
        method: 'DELETE',
        headers: { ...authHeaders() },
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({})) as { error?: string }
        throw new Error(data.error || 'Failed to delete room')
      }
      unmountedRef.current = true
      wsRef.current?.close()
      clearRoomAccess(roomId!)
      navigate('/', { replace: true })
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete room')
      setDeleting(false)
    }
  }

  const handleSendChat = (text: string) => {
    sendWs({
      action: 'chat',
      text,
      user_id: user?.id || '',
      username: user?.username || 'Anonymous',
    })
  }

  useEffect(() => {
    if (room?.current_track?.id !== prevTrackId.current) {
      prevTrackId.current = room?.current_track?.id ?? null
      setUserVote(0)
    }
  }, [room?.current_track?.id])

  const handleVote = (trackId: string, vote: 1 | -1 | 0) => {
    setUserVote(vote)
    sendWs({
      action: 'vote',
      user_id: user?.id || '',
      track_id: trackId,
      vote,
    })
  }

  const handleSkipVote = () => {
    sendWs({
      action: 'skip_vote',
      user_id: user?.id || '',
    })
  }

  const handleReact = (emoji: string) => {
    sendWs({ action: 'reaction', emoji, ...identity() })
  }

  if (loading) return <div className="loading">Loading...</div>

  if (locked) return (
    <div className="loading">
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, maxWidth: 360 }}>
        <h2>🔒 {room?.name || 'Room'}</h2>
        <p style={{ fontSize: 14, textAlign: 'center' }}>This room is password protected.</p>
        <input
          type="password"
          autoFocus
          placeholder="Room password"
          value={passwordInput}
          onChange={(e) => setPasswordInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleUnlock()}
          style={{ width: '100%' }}
        />
        {error && <div className="error-msg">{error}</div>}
        <button className="btn" onClick={handleUnlock} disabled={unlocking || !passwordInput} style={{ width: '100%' }}>
          {unlocking ? 'Checking...' : 'Enter room'}
        </button>
        <button className="btn btn-secondary" onClick={() => navigate('/')}>
          Back to Home
        </button>
      </div>
    </div>
  )

  if (error) return (
    <div className="loading">
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
        <div className="error-msg">{error}</div>
        <button className="btn btn-secondary" onClick={() => navigate('/')}>
          Back to Home
        </button>
      </div>
    </div>
  )

  return (
    <div className="room-page">
      <header className="room-header">
        <div className="room-header-left">
          <button className="btn btn-ghost btn-icon" onClick={() => navigate('/')} title="Back to home">
            ←
          </button>
          <div className="room-title-group">
            <h1 className="room-title">
              {room?.has_password && <span title="Password protected">🔒 </span>}
              {room?.name || 'Room'}
            </h1>
            <div className="room-meta">
              <Listeners
                listeners={room?.listeners ?? []}
                count={room?.user_count || 0}
              />
              {room?.auto_radio && (
                <>
                  <span className="room-divider">·</span>
                  <span className="radio-badge" title="Auto-radio keeps the queue full">📻 Radio</span>
                </>
              )}
              <span className="room-divider">·</span>
              <button
                className="room-code-btn"
                onClick={handleCopyCode}
                title={copied ? 'Copied!' : 'Click to copy room code'}
              >
                {copied ? '✓ Copied' : roomId}
              </button>
            </div>
          </div>
        </div>
        <div className="room-header-right">
          <button className="btn btn-ghost btn-sm" onClick={handleShare} title="Share room">
            Share
          </button>
          {isOwner && (
            <button className="btn btn-ghost btn-sm" onClick={() => setShowSettings(true)}>
              Settings
            </button>
          )}
        </div>
      </header>

      {showSettings && isOwner && (
        <div className="modal-overlay" onClick={() => setShowSettings(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>Room Settings</h3>
              <button className="btn-close" onClick={() => setShowSettings(false)}>×</button>
            </div>
            <div className="modal-body">
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={room?.allow_anonymous_add ?? true}
                  onChange={(e) => handleUpdateSettings({ allow_anonymous_add: e.target.checked })}
                />
                <span className="toggle-slider"></span>
                <span className="toggle-label">Allow anonymous users to add tracks</span>
              </label>
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={room?.is_private ?? false}
                  onChange={(e) => handleUpdateSettings({ is_private: e.target.checked })}
                />
                <span className="toggle-slider"></span>
                <span className="toggle-label">Private room (hidden from public list)</span>
              </label>
              <label className="settings-toggle">
                <input
                  type="checkbox"
                  checked={room?.auto_radio ?? false}
                  onChange={(e) => handleUpdateSettings({ auto_radio: e.target.checked })}
                />
                <span className="toggle-slider"></span>
                <span className="toggle-label">Auto-radio (keep playing related tracks when the queue runs out)</span>
              </label>

              <div
                className="settings-password"
                style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}
              >
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
                    onClick={() => handleUpdateSettings({ password: settingsPassword.trim() })}
                  >
                    {room?.has_password ? 'Change password' : 'Set password'}
                  </button>
                  {room?.has_password && (
                    <button
                      className="btn btn-sm btn-secondary"
                      onClick={() => handleUpdateSettings({ password: '' })}
                    >
                      Remove password
                    </button>
                  )}
                </div>
              </div>

              <div
                className="settings-danger"
                style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}
              >
                <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
                  Delete room
                </div>
                <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
                  Removes the room, its queue and chat for everyone. This cannot be undone.
                </div>
                {deleteError && <div className="error-msg" style={{ marginBottom: 8 }}>{deleteError}</div>}
                <button
                  className="btn btn-sm btn-danger"
                  onClick={handleDeleteRoom}
                  disabled={deleting}
                >
                  {deleting ? 'Deleting...' : 'Delete this room'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {!canAddTrack && (
        <div className="auth-banner">
          <span>Sign in to add tracks to the queue</span>
        </div>
      )}

      <div className="room-content">
        <div className="room-main">
          {canAddTrack && <AddTrack onAdd={handleAddTrack} />}
          <div className="player-wrap">
            <ReactionsOverlay items={reactions} />
            <Player
              roomId={roomId}
              track={room?.current_track ?? null}
              isPlaying={room?.is_playing ?? false}
              position={room?.position ?? 0}
              onPlayback={handlePlayback}
              likes={room?.current_track ? (room.track_votes?.likes ?? 0) : 0}
              dislikes={room?.current_track ? (room.track_votes?.dislikes ?? 0) : 0}
              userVote={userVote}
              onVote={handleVote}
              onSkipVote={handleSkipVote}
              skipVoters={room?.skip_voters ?? []}
              currentUserId={user?.id || ''}
            />
          </div>
          <Queue
            queue={room?.queue ?? []}
            currentIndex={room?.current_index ?? 0}
          />
        </div>
        <div className="room-chat">
          <Chat
            messages={chatMessages}
            onSend={handleSendChat}
            currentUserId={user?.id || ''}
          />
          <ReactionBar onReact={handleReact} />
        </div>
      </div>
    </div>
  )
}

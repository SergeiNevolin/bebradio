import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import Player from '../components/Player'
import Queue from '../components/Queue'
import AddTrack from '../components/AddTrack'
import Chat, { type ChatMessage } from '../components/Chat'

import type { RoomState } from '../types'

export default function Room() {
  const { roomId } = useParams<{ roomId: string }>()
  const navigate = useNavigate()
  const { user, authHeaders } = useAuth()
  const [room, setRoom] = useState<RoomState | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showSettings, setShowSettings] = useState(false)
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [copied, setCopied] = useState(false)
  const [userVote, setUserVote] = useState<1 | -1 | 0>(0)
  const prevTrackId = useRef<string | null>(null)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const isOwner = user && room && user.id === room.owner_id
  const canAddTrack = user || room?.allow_anonymous_add

  const connectWs = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/${roomId}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
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
      reconnectTimer.current = setTimeout(connectWs, 2000)
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [roomId])

  useEffect(() => {
    const fetchRoom = async () => {
      try {
        const res = await fetch(`/api/rooms/${roomId}`)
        if (!res.ok) throw new Error()
        const data = await res.json() as RoomState
        setRoom(data)
      } catch {
        setError('Room not found')
      } finally {
        setLoading(false)
      }
    }
    fetchRoom()
    connectWs()

    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [roomId, connectWs])

  const sendWs = (msg: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }

  const handleAddTrack = async (url: string): Promise<{ success: boolean; error?: string }> => {
    try {
      const res = await fetch(`/api/rooms/${roomId}/queue`, {
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

  const handleUpdateSettings = async (settings: { allow_anonymous_add?: boolean; is_private?: boolean }) => {
    setRoom((prev) => prev ? { ...prev, ...settings } : prev)
    try {
      await fetch(`/api/rooms/${roomId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify(settings),
      })
    } catch { /* ignore */ }
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

  if (loading) return <div className="loading">Loading...</div>
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
            <h1 className="room-title">{room?.name || 'Room'}</h1>
            <div className="room-meta">
              <span className="room-status">
                <span className="status-dot"></span>
                {room?.user_count || 0} listening
              </span>
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
          <Player
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
        </div>
      </div>
    </div>
  )
}

import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { setRoomAccess, clearRoomAccess } from '../lib/roomAccess'
import { useRoomWebSocket } from '../hooks/useRoomWebSocket'
import Player from '../components/Player'
import Queue from '../components/Queue'
import AddTrack from '../components/AddTrack'
import Chat from '../components/Chat'
import { ReactionBar, ReactionsOverlay } from '../components/Reactions'
import Listeners from '../components/Listeners'
import ProfileModal from '../components/ProfileModal'
import RoomHeader from '../components/room/RoomHeader'
import RoomPasswordGate from '../components/room/RoomPasswordGate'
import RoomSettingsModal from '../components/room/RoomSettingsModal'
import type { RoomState } from '../types'
import styles from './Room.module.css'

export default function Room() {
  const { roomId } = useParams<{ roomId: string }>()
  const navigate = useNavigate()
  const { user, token } = useAuth()

  const [passwordInput, setPasswordInput] = useState('')
  const [unlocking, setUnlocking] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [settingsPassword, setSettingsPassword] = useState('')
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const [copied, setCopied] = useState(false)
  const [profileUserId, setProfileUserId] = useState<string | null>(null)

  const {
    room, setRoom, loading, setLoading, error, setError, locked, setLocked,
    fetchRoom, connectWs, chatMessages, reactions, userVote,
    handleVote, handleSkipVote, handleReact, handleSendChat, handlePlayback,
  } = useRoomWebSocket(roomId, user, token, navigate)

  const isOwner = user && room && user.id === room.owner_id
  const canAddTrack = user || room?.allow_anonymous_add

  const handleUnlock = async () => {
    if (!passwordInput) return
    setUnlocking(true)
    setError('')
    try {
      const data = await api.joinRoom(roomId!, passwordInput)
      setRoomAccess(roomId!, data.access)
      setPasswordInput('')
      setLocked(false)
      setLoading(true)
      const fresh = await fetchRoom()
      if (fresh) connectWs()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not join room')
    } finally {
      setUnlocking(false)
    }
  }

  const handleAddTrack = async (url: string): Promise<{ success: boolean; error?: string }> => {
    try {
      await api.addTrack(roomId!, url)
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Unknown error' }
    }
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
      const data = await api.updateRoom(roomId!, settings) as unknown as RoomState
      setRoom((prev) => prev ? { ...prev, ...data } : data)
      if ('password' in settings) {
        if (!settings.password) clearRoomAccess(roomId!)
        setSettingsPassword('')
      }
    } catch { /* ignore */ }
  }

  const handleDeleteRoom = async () => {
    if (!window.confirm('Delete this room for everyone? This cannot be undone.')) return
    setDeleting(true)
    try {
      await api.deleteRoom(roomId!)
      clearRoomAccess(roomId!)
      navigate('/', { replace: true })
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete room')
      setDeleting(false)
    }
  }

  if (loading) return <div className="loading">Loading...</div>

  if (locked) return (
    <RoomPasswordGate
      roomName={room?.name || 'Room'}
      passwordInput={passwordInput}
      setPasswordInput={setPasswordInput}
      onUnlock={handleUnlock}
      unlocking={unlocking}
      error={error}
      onBack={() => navigate('/')}
    />
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
    <div className={styles.roomPage}>
      <RoomHeader
        room={room!}
        roomId={roomId!}
        isOwner={!!isOwner}
        copied={copied}
        onCopyCode={handleCopyCode}
        onShare={handleShare}
        onOpenSettings={() => setShowSettings(true)}
      />

      {showSettings && isOwner && (
        <RoomSettingsModal
          room={room!}
          onUpdate={handleUpdateSettings}
          onDelete={handleDeleteRoom}
          onClose={() => setShowSettings(false)}
          settingsPassword={settingsPassword}
          setSettingsPassword={setSettingsPassword}
          deleting={deleting}
          deleteError={deleteError}
        />
      )}

      {!canAddTrack && (
        <div className={styles.authBanner}>
          <span>Sign in to add tracks to the queue</span>
        </div>
      )}

      <div className={styles.roomContent}>
        <div className={styles.roomMain}>
          {canAddTrack && <AddTrack onAdd={handleAddTrack} />}
          <div className={styles.playerWrap}>
            <ReactionsOverlay items={reactions} />
            <Player
              roomId={roomId}
              track={room?.current_track ?? null}
              nextTrack={room?.queue?.[(room?.current_index ?? 0) + 1] ?? null}
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
            searching={room?.radio_searching ?? false}
          />
        </div>
        <div className={styles.roomChat}>
          <Chat
            messages={chatMessages}
            onSend={handleSendChat}
            currentUserId={user?.id || ''}
            onSelectUser={setProfileUserId}
          />
          <ReactionBar onReact={handleReact} />
        </div>
        <div className={styles.roomListeners}>
          <Listeners
            listeners={room?.listeners ?? []}
            ownerId={room?.owner_id ?? ''}
            onSelectUser={setProfileUserId}
          />
        </div>
      </div>

      {profileUserId && (
        <ProfileModal userId={profileUserId} onClose={() => setProfileUserId(null)} />
      )}
    </div>
  )
}

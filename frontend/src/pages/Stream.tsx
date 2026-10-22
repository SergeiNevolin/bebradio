import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { usePlayer } from '../context/PlayerContext'
import { api } from '../lib/api'
import { useStreamWebSocket } from '../hooks/useRoomWebSocket'
import RoomPlayer from '../components/player/RoomPlayer'
import Queue from '../components/room/Queue'
import AddTrack from '../components/room/AddTrack'
import Chat from '../components/room/Chat'
import { ReactionBar, ReactionsOverlay } from '../components/room/Reactions'
import ProfileModal from '../components/ProfileModal'
import RoomHeader from '../components/room/RoomHeader'
import StreamSettingsModal from '../components/stream/StreamSettingsModal'
import type { StreamFilter, StreamState } from '../types'
import styles from './Room.module.css'

export default function Stream() {
  const { streamId } = useParams<{ streamId: string }>()
  const navigate = useNavigate()
  const { user, token } = useAuth()
  const { player: sharedPlayer } = usePlayer()

  const [showSettings, setShowSettings] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const [copied, setCopied] = useState(false)
  const [profileUserId, setProfileUserId] = useState<string | null>(null)

  const pauseSharedPlayer = sharedPlayer.pause
  useEffect(() => {
    pauseSharedPlayer()
  }, [pauseSharedPlayer])

  const {
    room, setRoom, loading, error, setError,
    chatMessages, reactions, userVote,
    handleVote, handleSkipVote, handleReact, handleSendChat, handlePlayback,
  } = useStreamWebSocket(streamId, user, token, navigate)

  const stream = room as StreamState | null
  const isAdmin = !!user && user.role === 'admin'
  const isOwner = !!user && !!stream && user.id === stream.owner_id
  const canManage = isAdmin || isOwner

  const handleAddTrack = async (url: string): Promise<{ success: boolean; error?: string }> => {
    try {
      await api.addStreamTrack(streamId!, url)
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Unknown error' }
    }
  }

  const handleAddMashup = async (trackId: string): Promise<{ success: boolean; error?: string }> => {
    try {
      await api.addStreamTrackById(streamId!, trackId)
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Unknown error' }
    }
  }

  const handleCopyCode = () => {
    navigator.clipboard.writeText(streamId!)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleShare = async () => {
    const url = `${window.location.origin}/stream/${streamId}`
    if (navigator.share) {
      await navigator.share({ title: stream?.name, url })
    } else {
      await navigator.clipboard.writeText(url)
    }
  }

  const handleUpdateFeature = async (patch: { enabled?: boolean; filter?: StreamFilter }) => {
    setRoom((prev) => {
      if (!prev) return prev
      const current = prev as StreamState
      return {
        ...current,
        mashup_radio: patch.enabled ?? current.mashup_radio,
        stream_filter: { ...current.stream_filter, ...patch.filter },
        features: {
          ...current.features,
          mashup_radio: {
            enabled: patch.enabled ?? current.features.mashup_radio.enabled,
            filter: { ...current.features.mashup_radio.filter, ...patch.filter },
          },
        },
      }
    })
    try {
      const data = await api.updateMashupRadio(streamId!, patch)
      setRoom(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось обновить поток')
    }
  }

  const handleDeleteStream = async () => {
    if (!window.confirm('Удалить этот поток для всех? Это действие нельзя отменить.')) return
    setDeleting(true)
    setDeleteError('')
    try {
      await api.deleteStream(streamId!)
      navigate('/streams', { replace: true })
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Не удалось удалить поток')
      setDeleting(false)
    }
  }

  if (loading) return <div className="loading">Loading...</div>

  if (error) return (
    <div className="loading">
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
        <div className="error-msg">{error}</div>
        <button className="btn btn-secondary" onClick={() => navigate('/streams')}>
          К потокам
        </button>
      </div>
    </div>
  )

  if (!stream) return null

  return (
    <div className={styles.roomPage}>
      <RoomHeader
        room={stream}
        roomId={streamId!}
        isOwner={canManage}
        copied={copied}
        onCopyCode={handleCopyCode}
        onShare={handleShare}
        onOpenSettings={() => setShowSettings(true)}
      />

      {showSettings && canManage && (
        <StreamSettingsModal
          stream={stream}
          onUpdate={handleUpdateFeature}
          onDelete={handleDeleteStream}
          onClose={() => setShowSettings(false)}
          deleting={deleting}
          deleteError={deleteError}
        />
      )}

      {canManage && (
        <div className={styles.roomSearch}>
          <AddTrack onAdd={handleAddTrack} onAddById={handleAddMashup} />
        </div>
      )}

      <div className={styles.roomContent}>
        <div className={styles.roomMain}>
          <div className={styles.playerWrap}>
            <ReactionsOverlay items={reactions} />
            <RoomPlayer
              track={stream.current_track ?? null}
              nextTrack={stream.queue?.[(stream.current_index ?? 0) + 1] ?? null}
              isPlaying={stream.is_playing ?? false}
              position={stream.position ?? 0}
              onPlayback={handlePlayback}
              likes={0}
              dislikes={0}
              userVote={userVote}
              onVote={handleVote}
              onSkipVote={handleSkipVote}
              skipVoters={[]}
              currentUserId={user?.id || ''}
              canVote={false}
              canSkip={canManage}
            />
          </div>
          <Queue
            queue={stream.queue ?? []}
            currentIndex={stream.current_index ?? 0}
            searching={stream.radio_searching ?? false}
            canImport={false}
            onImport={async () => {}}
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
      </div>

      {profileUserId && (
        <ProfileModal userId={profileUserId} onClose={() => setProfileUserId(null)} />
      )}
    </div>
  )
}

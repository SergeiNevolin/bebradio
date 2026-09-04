import { memo, useCallback, useEffect, useState } from 'react'
import type { Track } from '../types'
import Karaoke from './Karaoke'
import SeekBar from './player/SeekBar'
import VolumeControl from './player/VolumeControl'
import { useGaplessPlayer } from '../hooks/useGaplessPlayer'
import { useVolume } from '../hooks/useVolume'

/** Seconds of overlap between tracks. 0 would disable crossfading. */
const CROSSFADE_SECONDS = 3

interface PlayerProps {
  track: Track | null
  nextTrack?: Track | null
  isPlaying: boolean
  position: number
  onPlayback: (action: string, extra?: Record<string, unknown>) => void
  likes: number
  dislikes: number
  userVote: 1 | -1 | 0
  onVote: (trackId: string, vote: 1 | -1 | 0) => void
  onSkipVote: () => void
  skipVoters: string[]
  currentUserId: string
  roomId?: string
}

function Player({
  track,
  nextTrack,
  isPlaying,
  position,
  onPlayback,
  likes,
  dislikes,
  userVote,
  onVote,
  onSkipVote,
  skipVoters,
  currentUserId,
  roomId,
}: PlayerProps) {
  const [showKaraoke, setShowKaraoke] = useState(false)

  const onEnded = useCallback(() => onPlayback('next'), [onPlayback])
  const onSync = useCallback(
    (p: number) => onPlayback('sync', { position: p }),
    [onPlayback],
  )

  const { volume, muted, setVolume, toggleMute } = useVolume()

  const {
    deckRefs,
    position: localPos,
    duration,
    needsGesture,
    crossfading,
    unlock,
  } = useGaplessPlayer({
    trackId: track?.id,
    src: track?.url,
    nextTrackId: nextTrack?.id,
    nextSrc: nextTrack?.url,
    isPlaying,
    serverPosition: position,
    volume,
    muted,
    crossfadeSeconds: CROSSFADE_SECONDS,
    onEnded,
    onSync,
  })

  // Keyboard shortcuts: space resyncs / starts, up/down adjust volume.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return
      if (!track) return

      if (e.code === 'Space') {
        e.preventDefault()
        onPlayback(
          isPlaying ? 'sync' : 'next',
          isPlaying ? { position: localPos } : {},
        )
      } else if (e.code === 'ArrowUp') {
        e.preventDefault()
        setVolume(volume + 0.1)
      } else if (e.code === 'ArrowDown') {
        e.preventDefault()
        setVolume(volume - 0.1)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [track, isPlaying, localPos, volume, onPlayback, setVolume])

  const hasVoted = !!track && skipVoters.includes(currentUserId)
  const skipCount = skipVoters.length
  const voteTotal = likes + dislikes

  return (
    <div className={`player${crossfading ? ' is-crossfading' : ''}`}>
      <audio ref={deckRefs[0]} preload="auto" />
      <audio ref={deckRefs[1]} preload="auto" />

      {needsGesture && track && (
        <button type="button" className="player-unlock" onClick={unlock}>
          ▶ Tap to enable sound
        </button>
      )}

      {!track ? (
        <div className="player-empty">Add a track to start listening together</div>
      ) : (
        <>
          <div className="player-head">
            {track.thumbnail && <img className="player-thumb" src={track.thumbnail} alt="" />}
            <div className="player-info">
              <div className="title">{track.title}</div>
              <div className="artist">{track.artist}</div>
              <div className="added-by">Added by {track.added_by}</div>
            </div>
          </div>

          <SeekBar position={localPos} duration={duration} />

          <div className="player-controls">
            <VolumeControl
              volume={volume}
              muted={muted}
              onVolume={setVolume}
              onToggleMute={toggleMute}
            />
            <div className="player-controls-gap" />
            {roomId && (
              <button
                type="button"
                className={`player-chip${showKaraoke ? ' is-on' : ''}`}
                aria-pressed={showKaraoke}
                onClick={() => setShowKaraoke((v) => !v)}
              >
                🎤 Karaoke
              </button>
            )}
            <button
              type="button"
              className={`player-chip${hasVoted ? ' is-on' : ''}`}
              onClick={onSkipVote}
              title="Vote to skip"
            >
              ⏭ Skip{skipCount > 0 ? ` (${skipCount})` : ''}
            </button>
          </div>

          {roomId && showKaraoke && (
            <Karaoke roomId={roomId} trackId={track.id} currentTime={localPos} />
          )}

          <div className="vote-buttons">
            <button
              type="button"
              className={`vote-btn ${userVote === 1 ? 'vote-btn-active' : ''}`}
              onClick={() => onVote(track.id, userVote === 1 ? 0 : 1)}
            >
              👍 {likes}
            </button>
            <div className="vote-scale">
              <div className="vote-bar">
                {likes > 0 && (
                  <div className="vote-bar-like" style={{ width: `${(likes / voteTotal) * 100}%` }} />
                )}
                {dislikes > 0 && (
                  <div
                    className="vote-bar-dislike"
                    style={{ width: `${(dislikes / voteTotal) * 100}%` }}
                  />
                )}
              </div>
            </div>
            <button
              type="button"
              className={`vote-btn ${userVote === -1 ? 'vote-btn-active-down' : ''}`}
              onClick={() => onVote(track.id, userVote === -1 ? 0 : -1)}
            >
              👎 {dislikes}
            </button>
          </div>
        </>
      )}
    </div>
  )
}

export default memo(Player)

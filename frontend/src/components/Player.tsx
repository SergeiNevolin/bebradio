import { useRef, useEffect, useState } from 'react'
import type { Track } from '../types'

function formatTime(s: number): string {
  if (!s || isNaN(s)) return '0:00'
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${sec.toString().padStart(2, '0')}`
}

interface PlayerProps {
  track: Track | null
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
}

export default function Player({ track, isPlaying, position, onPlayback, likes, dislikes, userVote, onVote, onSkipVote, skipVoters, currentUserId }: PlayerProps) {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [localPos, setLocalPos] = useState(0)
  const [duration, setDuration] = useState(0)
  const [volume, setVolume] = useState<number>(() => {
    const saved = localStorage.getItem('player_volume')
    return saved !== null ? Number(saved) : 0.7
  })
  const [muted, setMuted] = useState(false)
  const trackIdRef = useRef<string | null>(null)
  const posInterval = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return

    const onEnded = () => onPlayback('next')
    audio.addEventListener('ended', onEnded)
    return () => audio.removeEventListener('ended', onEnded)
  }, [onPlayback])

  // Stop playback when the player is removed from the screen (e.g. navigating
  // out of the room). A detached <audio> element can otherwise keep playing
  // until the browser garbage-collects it.
  useEffect(() => {
    const audio = audioRef.current
    return () => {
      if (!audio) return
      audio.pause()
      audio.removeAttribute('src')
      audio.load()
    }
  }, [])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return

    const onMeta = () => setDuration(audio.duration)
    audio.addEventListener('loadedmetadata', onMeta)
    return () => audio.removeEventListener('loadedmetadata', onMeta)
  }, [])

  useEffect(() => {
    if (posInterval.current) clearInterval(posInterval.current)
    posInterval.current = setInterval(() => {
      const audio = audioRef.current
      if (audio && !audio.paused) {
        setLocalPos(audio.currentTime)
      }
    }, 250)
    return () => { if (posInterval.current) clearInterval(posInterval.current) }
  }, [])

  useEffect(() => {
    if (!isPlaying) return
    const syncInterval = setInterval(() => {
      const audio = audioRef.current
      if (audio && !audio.paused) {
        onPlayback('sync', { position: audio.currentTime })
      }
    }, 5000)
    return () => clearInterval(syncInterval)
  }, [isPlaying, onPlayback])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !track?.url) return

    if (track.id !== trackIdRef.current) {
      trackIdRef.current = track.id
      audio.src = track.url
      setLocalPos(0)
      setDuration(0)
      audio.load()
    }
  }, [track?.id, track?.url])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !track?.url) return
    if (trackIdRef.current !== track.id) return

    if (isPlaying) {
      audio.play().catch(() => {})
    } else {
      audio.pause()
    }
  }, [isPlaying, track?.id])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !track?.url) return
    if (position === undefined) return

    if (Math.abs(audio.currentTime - position) > 1.5) {
      audio.currentTime = position
      setLocalPos(position)
    }
  }, [position, track?.url])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return
    audio.volume = muted ? 0 : volume
    localStorage.setItem('player_volume', String(volume))
  }, [volume, muted])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return

      if (e.code === 'Space') {
        e.preventDefault()
        if (track) {
          onPlayback(isPlaying ? 'sync' : 'next', isPlaying ? { position: audioRef.current?.currentTime || 0 } : {})
        }
      } else if (e.code === 'ArrowRight') {
        e.preventDefault()
        if (track && audioRef.current) {
          audioRef.current.currentTime = Math.min(audioRef.current.currentTime + 10, duration)
        }
      } else if (e.code === 'ArrowLeft') {
        e.preventDefault()
        if (track && audioRef.current) {
          audioRef.current.currentTime = Math.max(audioRef.current.currentTime - 10, 0)
        }
      } else if (e.code === 'ArrowUp') {
        e.preventDefault()
        setVolume((v) => Math.min(v + 0.1, 1))
      } else if (e.code === 'ArrowDown') {
        e.preventDefault()
        setVolume((v) => Math.max(v - 0.1, 0))
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [track, isPlaying, duration, onPlayback])

  const hasVoted = track ? (skipVoters.includes(currentUserId)) : false
  const skipCount = skipVoters.length

  return (
    <div className="player">
      <audio ref={audioRef} preload="auto" />

      {!track ? (
        <div className="player-empty">
          Add a track to start listening together
        </div>
      ) : (
        <>
          <div className="volume-control">
            <button
              className="volume-btn"
              onClick={() => setMuted((m) => !m)}
              title={muted ? 'Unmute' : 'Mute'}
            >
              {muted || volume === 0 ? (
                <svg viewBox="0 0 24 24"><path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51A8.796 8.796 0 0021 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06a8.99 8.99 0 003.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/></svg>
              ) : volume < 0.5 ? (
                <svg viewBox="0 0 24 24"><path d="M18.5 12c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM5 9v6h4l5 5V4L9 9H5z"/></svg>
              ) : (
                <svg viewBox="0 0 24 24"><path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/></svg>
              )}
            </button>
            <input
              type="range"
              className="volume-slider"
              min="0"
              max="1"
              step="0.01"
              value={muted ? 0 : volume}
              onChange={(e) => {
                setVolume(Number(e.target.value))
                if (muted) setMuted(false)
              }}
            />
          </div>

          <div className="player-track">
            {track.thumbnail && (
              <img className="player-thumb" src={track.thumbnail} alt="" />
            )}
            <div className="player-info">
              <div className="title">{track.title}</div>
              <div className="artist">{track.artist}</div>
              <div className="added-by">Added by {track.added_by}</div>
            </div>
          </div>

          <div className="time-display">
            <span>{formatTime(Math.min(localPos, duration))}</span>
            <span>{formatTime(duration)}</span>
          </div>

          <div className="progress-bar">
            <div
              className="progress-fill"
              style={{ width: `${duration ? Math.min((localPos / duration) * 100, 100) : 0}%` }}
            />
          </div>

          <div className="vote-buttons">
            <div className="vote-like-group">
              <button
                className={`vote-btn ${userVote === 1 ? 'vote-btn-active' : ''}`}
                onClick={() => onVote(track.id, userVote === 1 ? 0 : 1)}
              >
                👍 {likes}
              </button>
              <button
                className={`vote-btn ${userVote === -1 ? 'vote-btn-active-down' : ''}`}
                onClick={() => onVote(track.id, userVote === -1 ? 0 : -1)}
              >
                👎 {dislikes}
              </button>
            </div>
            <div className="vote-scale">
              <div className="vote-bar">
                {likes > 0 && (
                  <div
                    className="vote-bar-like"
                    style={{ width: `${(likes / (likes + dislikes)) * 100}%` }}
                  />
                )}
                {dislikes > 0 && (
                  <div
                    className="vote-bar-dislike"
                    style={{ width: `${(dislikes / (likes + dislikes)) * 100}%` }}
                  />
                )}
              </div>
            </div>
            <button
              className={`vote-btn vote-skip ${hasVoted ? 'vote-skip-active' : ''}`}
              onClick={onSkipVote}
            >
              ⏭ Skip {skipCount > 0 && `(${skipCount})`}
            </button>
          </div>
        </>
      )}
    </div>
  )
}

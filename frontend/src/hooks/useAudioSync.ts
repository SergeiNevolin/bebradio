import { useCallback, useEffect, useRef, useState } from 'react'

interface AudioSyncOptions {
  /** Current track id; a change swaps the media source. */
  trackId: string | undefined
  /** Playable stream URL for the current track. */
  src: string | undefined
  /** Whether the room's shared playback is running. */
  isPlaying: boolean
  /** Authoritative position (seconds) from the server, for drift correction. */
  serverPosition: number
  /** Fired when the media element reaches the end of the track. */
  onEnded: () => void
  /** Fired every few seconds while playing so the server can stay in sync. */
  onSync: (position: number) => void
}

interface AudioSync {
  audioRef: React.RefObject<HTMLAudioElement>
  /** Live playback position in seconds. */
  position: number
  /** Track length in seconds (0 until metadata loads). */
  duration: number
  /** True when the browser blocked autoplay and needs a user gesture. */
  needsGesture: boolean
  /** Retry playback after autoplay was blocked. */
  unlock: () => void
  /** Move playback locally (the caller is responsible for telling the server). */
  seek: (seconds: number) => void
}

const SYNC_INTERVAL_MS = 5000
const TICK_INTERVAL_MS = 250
const DRIFT_TOLERANCE_S = 1.5

/**
 * Owns the `<audio>` element and keeps it in step with the room's shared
 * playback state: source swapping, play/pause, position drift correction,
 * periodic sync, end-of-track handling and mobile autoplay unlocking.
 */
export function useAudioSync({
  trackId,
  src,
  isPlaying,
  serverPosition,
  onEnded,
  onSync,
}: AudioSyncOptions): AudioSync {
  const audioRef = useRef<HTMLAudioElement>(null)
  const loadedIdRef = useRef<string | null>(null)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [needsGesture, setNeedsGesture] = useState(false)

  // End-of-track -> advance the queue.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return
    const handler = () => onEnded()
    audio.addEventListener('ended', handler)
    return () => audio.removeEventListener('ended', handler)
  }, [onEnded])

  // Track length.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return
    const handler = () => setDuration(audio.duration || 0)
    audio.addEventListener('loadedmetadata', handler)
    return () => audio.removeEventListener('loadedmetadata', handler)
  }, [])

  // Local position ticker.
  useEffect(() => {
    const id = setInterval(() => {
      const audio = audioRef.current
      if (audio && !audio.paused) setPosition(audio.currentTime)
    }, TICK_INTERVAL_MS)
    return () => clearInterval(id)
  }, [])

  // Push our position to the server while playing.
  useEffect(() => {
    if (!isPlaying) return
    const id = setInterval(() => {
      const audio = audioRef.current
      if (audio && !audio.paused) onSync(audio.currentTime)
    }, SYNC_INTERVAL_MS)
    return () => clearInterval(id)
  }, [isPlaying, onSync])

  // Swap the source when the track changes; tear it down when the queue empties.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return

    if (!src) {
      // The last track was skipped/removed. Nothing else pauses us, so stop
      // here or the buffered stream keeps playing with no track on screen.
      if (loadedIdRef.current !== null) {
        loadedIdRef.current = null
        audio.pause()
        audio.removeAttribute('src')
        audio.load()
        setPosition(0)
        setDuration(0)
        setNeedsGesture(false)
      }
      return
    }

    if (trackId !== loadedIdRef.current) {
      loadedIdRef.current = trackId ?? null
      audio.src = src
      setPosition(0)
      setDuration(0)
      audio.load()
    }
  }, [trackId, src])

  // Play / pause to match the shared state.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !src || loadedIdRef.current !== trackId) return

    if (isPlaying) {
      audio
        .play()
        .then(() => setNeedsGesture(false))
        .catch(() => setNeedsGesture(true))
    } else {
      audio.pause()
      setNeedsGesture(false)
    }
  }, [isPlaying, trackId, src])

  // After autoplay was blocked, the first interaction anywhere unlocks it.
  useEffect(() => {
    if (!needsGesture || !isPlaying) return
    const resume = () => {
      audioRef.current?.play().then(() => setNeedsGesture(false)).catch(() => {})
    }
    window.addEventListener('pointerdown', resume)
    window.addEventListener('keydown', resume)
    return () => {
      window.removeEventListener('pointerdown', resume)
      window.removeEventListener('keydown', resume)
    }
  }, [needsGesture, isPlaying])

  // Correct drift against the server's authoritative position.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !src || serverPosition === undefined) return
    if (Math.abs(audio.currentTime - serverPosition) > DRIFT_TOLERANCE_S) {
      audio.currentTime = serverPosition
      setPosition(serverPosition)
    }
  }, [serverPosition, src])

  // Stop playback when the player leaves the screen (e.g. leaving the room);
  // a detached <audio> otherwise plays until it is garbage-collected.
  useEffect(() => {
    const audio = audioRef.current
    return () => {
      if (!audio) return
      audio.pause()
      audio.removeAttribute('src')
      audio.load()
    }
  }, [])

  const seek = useCallback((seconds: number) => {
    const audio = audioRef.current
    if (!audio) return
    audio.currentTime = seconds
    setPosition(seconds)
  }, [])

  const unlock = useCallback(() => {
    audioRef.current?.play().then(() => setNeedsGesture(false)).catch(() => {})
  }, [])

  return { audioRef, position, duration, needsGesture, unlock, seek }
}

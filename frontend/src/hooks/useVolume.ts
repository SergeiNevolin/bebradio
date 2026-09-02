import { useCallback, useEffect, useState } from 'react'

const STORAGE_KEY = 'player_volume'
const DEFAULT_VOLUME = 0.7

function readStored(): number {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    return saved !== null ? Number(saved) : DEFAULT_VOLUME
  } catch {
    return DEFAULT_VOLUME
  }
}

interface VolumeControls {
  volume: number
  muted: boolean
  /** Set the volume (0–1); any explicit change also unmutes. */
  setVolume: (v: number) => void
  toggleMute: () => void
}

/**
 * Playback volume with mute, persisted to localStorage and applied to the
 * given media element.
 */
export function useVolume(audioRef: React.RefObject<HTMLAudioElement>): VolumeControls {
  const [volume, setVolumeState] = useState(readStored)
  const [muted, setMuted] = useState(false)

  useEffect(() => {
    const audio = audioRef.current
    if (audio) audio.volume = muted ? 0 : volume
    try {
      localStorage.setItem(STORAGE_KEY, String(volume))
    } catch {
      /* ignore */
    }
  }, [volume, muted, audioRef])

  const setVolume = useCallback((v: number) => {
    const clamped = Math.min(1, Math.max(0, v))
    setVolumeState(clamped)
    if (clamped > 0) setMuted(false)
  }, [])

  const toggleMute = useCallback(() => setMuted((m) => !m), [])

  return { volume, muted, setVolume, toggleMute }
}

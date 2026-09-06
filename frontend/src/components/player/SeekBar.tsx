import { useCallback, useRef, type KeyboardEvent, type PointerEvent } from 'react'
import { formatTime } from '../../lib/format'
import styles from './SeekBar.module.css'

interface SeekBarProps {
  /** Live playback position in seconds. */
  position: number
  /** Track length in seconds. */
  duration: number
  /**
   * When provided the bar becomes an interactive slider (click + drag + arrow
   * keys). Without it the bar stays a read-only `role="meter"` indicator, which
   * is how the room Player uses it.
   */
  onSeek?: (seconds: number) => void
}

export default function SeekBar({ position, duration, onSeek }: SeekBarProps) {
  const trackRef = useRef<HTMLDivElement>(null)
  const shown = position
  const pct = duration ? Math.min(100, Math.max(0, (shown / duration) * 100)) : 0
  const interactive = typeof onSeek === 'function'

  const seekToClientX = useCallback(
    (clientX: number) => {
      const el = trackRef.current
      if (!el || !duration || !onSeek) return
      const rect = el.getBoundingClientRect()
      if (rect.width === 0) return
      const ratio = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width))
      onSeek(ratio * duration)
    },
    [duration, onSeek],
  )

  const handlePointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (!onSeek) return
    e.preventDefault()
    seekToClientX(e.clientX)
    const move = (ev: globalThis.PointerEvent) => seekToClientX(ev.clientX)
    const up = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!onSeek || !duration) return
    if (e.key === 'ArrowRight') {
      e.preventDefault()
      onSeek(Math.min(duration, shown + 5))
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault()
      onSeek(Math.max(0, shown - 5))
    }
  }

  return (
    <div className={styles.playerSeek}>
      <span className={styles.seekTime}>{formatTime(Math.min(shown, duration || shown))}</span>
      <div
        ref={trackRef}
        className={styles.seekTrack}
        role={interactive ? 'slider' : 'meter'}
        aria-label="Playback position"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(shown)}
        tabIndex={interactive ? 0 : undefined}
        onPointerDown={interactive ? handlePointerDown : undefined}
        onKeyDown={interactive ? handleKeyDown : undefined}
      >
        <div className={styles.seekFill} style={{ width: `${pct}%` }} />
        <div className={styles.seekThumb} style={{ left: `${pct}%` }} />
      </div>
      <span className={styles.seekTime}>{formatTime(duration)}</span>
    </div>
  )
}

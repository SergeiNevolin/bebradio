import { useCallback, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { formatTime } from '../../lib/format'
import styles from './SeekBar.module.css'

interface SeekBarProps {
  /** Live playback position in seconds. */
  position: number
  /** Track length in seconds. */
  duration: number
  /**
   * Seconds of audio buffered ahead. When set, a lighter fill is drawn behind
   * the played portion. Optional, so the room Player (meter mode) is unaffected.
   */
  buffered?: number
  /**
   * When provided the bar becomes an interactive slider (click + drag + arrow
   * keys). Without it the bar stays a read-only `role="meter"` indicator, which
   * is how the room Player uses it.
   */
  onSeek?: (seconds: number) => void
}

export default function SeekBar({ position, duration, buffered, onSeek }: SeekBarProps) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [hoverPct, setHoverPct] = useState<number | null>(null)
  const [scrubbing, setScrubbing] = useState(false)
  const shown = position
  const pct = duration ? Math.min(100, Math.max(0, (shown / duration) * 100)) : 0
  const bufPct =
    duration && buffered ? Math.min(100, Math.max(0, (buffered / duration) * 100)) : 0
  const interactive = typeof onSeek === 'function'

  const ratioFromClientX = useCallback((clientX: number): number | null => {
    const el = trackRef.current
    if (!el) return null
    const rect = el.getBoundingClientRect()
    if (rect.width === 0) return null
    return Math.min(1, Math.max(0, (clientX - rect.left) / rect.width))
  }, [])

  const seekToClientX = useCallback(
    (clientX: number) => {
      if (!duration || !onSeek) return
      const ratio = ratioFromClientX(clientX)
      if (ratio == null) return
      onSeek(ratio * duration)
    },
    [duration, onSeek, ratioFromClientX],
  )

  const handlePointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (!onSeek) return
    e.preventDefault()
    setScrubbing(true)
    seekToClientX(e.clientX)
    const move = (ev: globalThis.PointerEvent) => seekToClientX(ev.clientX)
    const up = () => {
      setScrubbing(false)
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  const handleHover = (e: PointerEvent<HTMLDivElement>) => {
    if (!interactive) return
    const ratio = ratioFromClientX(e.clientX)
    if (ratio != null) setHoverPct(ratio * 100)
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

  const tipPct = scrubbing ? pct : hoverPct
  const showTip = interactive && duration > 0 && tipPct != null

  return (
    <div className={styles.playerSeek}>
      <span className={styles.seekTime}>{formatTime(Math.min(shown, duration || shown))}</span>
      <div
        ref={trackRef}
        className={`${styles.seekTrack}${scrubbing ? ` ${styles.seekTrackScrubbing}` : ''}`}
        role={interactive ? 'slider' : 'meter'}
        aria-label="Playback position"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(shown)}
        tabIndex={interactive ? 0 : undefined}
        onPointerDown={interactive ? handlePointerDown : undefined}
        onPointerMove={interactive ? handleHover : undefined}
        onPointerOver={interactive ? handleHover : undefined}
        onPointerLeave={interactive ? () => setHoverPct(null) : undefined}
        onKeyDown={interactive ? handleKeyDown : undefined}
      >
        {bufPct > 0 && (
          <div className={styles.seekBuffer} style={{ width: `${bufPct}%` }} />
        )}
        <div className={styles.seekFill} style={{ width: `${pct}%` }} />
        <div className={styles.seekThumb} style={{ left: `${pct}%` }} />
        {showTip && (
          <span className={styles.seekTooltip} style={{ left: `${tipPct}%` }}>
            {formatTime((tipPct / 100) * duration)}
          </span>
        )}
      </div>
      <span className={styles.seekTime}>{formatTime(duration)}</span>
    </div>
  )
}

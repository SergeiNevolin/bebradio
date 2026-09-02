import { useRef, useState } from 'react'

function formatTime(s: number): string {
  if (!s || isNaN(s) || s < 0) return '0:00'
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${sec.toString().padStart(2, '0')}`
}

interface SeekBarProps {
  /** Live playback position in seconds. */
  position: number
  /** Track length in seconds. */
  duration: number
  /** Called with the target position (seconds) when the user seeks. */
  onSeek: (seconds: number) => void
}

const KEY_STEP_S = 5

export default function SeekBar({ position, duration, onSeek }: SeekBarProps) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [preview, setPreview] = useState<number | null>(null)

  const ratioAt = (clientX: number): number => {
    const el = trackRef.current
    if (!el) return 0
    const { left, width } = el.getBoundingClientRect()
    if (width <= 0) return 0
    return Math.min(1, Math.max(0, (clientX - left) / width))
  }

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!duration) return
    e.currentTarget.setPointerCapture(e.pointerId)
    setPreview(ratioAt(e.clientX) * duration)
  }

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (preview === null) return
    setPreview(ratioAt(e.clientX) * duration)
  }

  const commit = (e: React.PointerEvent<HTMLDivElement>) => {
    if (preview === null) return
    const target = ratioAt(e.clientX) * duration
    e.currentTarget.releasePointerCapture?.(e.pointerId)
    setPreview(null)
    onSeek(target)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (!duration) return
    let next: number | null = null
    if (e.key === 'ArrowLeft') next = Math.max(0, position - KEY_STEP_S)
    else if (e.key === 'ArrowRight') next = Math.min(duration, position + KEY_STEP_S)
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = duration
    if (next === null) return
    e.preventDefault()
    e.stopPropagation()
    onSeek(next)
  }

  const shown = preview ?? position
  const pct = duration ? Math.min(100, Math.max(0, (shown / duration) * 100)) : 0

  return (
    <div className="player-seek">
      <span className="seek-time">{formatTime(Math.min(shown, duration || shown))}</span>
      <div
        ref={trackRef}
        className={`seek-track${preview !== null ? ' is-scrubbing' : ''}`}
        role="slider"
        tabIndex={0}
        aria-label="Seek"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(shown)}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={commit}
        onPointerCancel={commit}
        onKeyDown={handleKeyDown}
      >
        <div className="seek-fill" style={{ width: `${pct}%` }} />
        <div className="seek-thumb" style={{ left: `${pct}%` }} />
      </div>
      <span className="seek-time">{formatTime(duration)}</span>
    </div>
  )
}



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
}

export default function SeekBar({ position, duration }: SeekBarProps) {
  const shown = position
  const pct = duration ? Math.min(100, Math.max(0, (shown / duration) * 100)) : 0

  return (
    <div className="player-seek">
      <span className="seek-time">{formatTime(Math.min(shown, duration || shown))}</span>
      <div
        className="seek-track"
        role="meter"
        aria-label="Playback position"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(shown)}
      >
        <div className="seek-fill" style={{ width: `${pct}%` }} />
        <div className="seek-thumb" style={{ left: `${pct}%` }} />
      </div>
      <span className="seek-time">{formatTime(duration)}</span>
    </div>
  )
}

import { formatTime } from '../../lib/format'
import styles from './SeekBar.module.css'

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
    <div className={styles.playerSeek}>
      <span className={styles.seekTime}>{formatTime(Math.min(shown, duration || shown))}</span>
      <div
        className={styles.seekTrack}
        role="meter"
        aria-label="Playback position"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(shown)}
      >
        <div className={styles.seekFill} style={{ width: `${pct}%` }} />
        <div className={styles.seekThumb} style={{ left: `${pct}%` }} />
      </div>
      <span className={styles.seekTime}>{formatTime(duration)}</span>
    </div>
  )
}

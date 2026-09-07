import styles from './VolumeControl.module.css'
import { VolumeHighIcon, VolumeLowIcon, VolumeMutedIcon } from './icons'

interface VolumeControlProps {
  volume: number
  muted: boolean
  onVolume: (v: number) => void
  onToggleMute: () => void
}

export default function VolumeControl({ volume, muted, onVolume, onToggleMute }: VolumeControlProps) {
  const silent = muted || volume === 0
  const effective = muted ? 0 : volume

  return (
    <div className={styles.volumeControl}>
      <button
        type="button"
        className={styles.volumeBtn}
        onClick={onToggleMute}
        title={silent ? 'Unmute' : 'Mute'}
        aria-label={silent ? 'Unmute' : 'Mute'}
      >
        {silent ? <VolumeMutedIcon /> : volume < 0.5 ? <VolumeLowIcon /> : <VolumeHighIcon />}
      </button>
      <input
        type="range"
        className={styles.volumeSlider}
        min="0"
        max="1"
        step="0.01"
        value={effective}
        aria-label="Volume"
        onChange={(e) => onVolume(Number(e.target.value))}
      />
    </div>
  )
}

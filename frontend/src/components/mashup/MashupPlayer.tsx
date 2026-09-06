import SeekBar from '../player/SeekBar'
import VolumeControl from '../player/VolumeControl'
import type { MashupPlayer as MashupPlayerState } from '../../hooks/useMashupPlayer'
import styles from './MashupPlayer.module.css'

interface MashupPlayerProps {
  player: MashupPlayerState
}

export default function MashupPlayer({ player }: MashupPlayerProps) {
  const {
    audioRef,
    current,
    isPlaying,
    position,
    duration,
    volume,
    muted,
    setVolume,
    toggleMute,
    toggle,
    next,
    prev,
    seek,
    index,
    list,
  } = player

  return (
    <div className={styles.bar} hidden={!current}>
      {/* Always mounted so the hook's ref is available even with nothing playing. */}
      <audio ref={audioRef} preload="auto" />

      {current && (
        <div className={styles.inner}>
          <div className={styles.meta}>
            {current.cover_url ? (
              <img className={styles.cover} src={current.cover_url} alt="" />
            ) : (
              <div className={styles.coverFallback} aria-hidden="true">
                ♪
              </div>
            )}
            <div className={styles.text}>
              <div className={styles.title} title={current.title}>
                {current.title}
              </div>
              <div className={styles.artist}>{current.artist || 'Unknown artist'}</div>
            </div>
          </div>

          <div className={styles.center}>
            <div className={styles.controls}>
              <button
                type="button"
                className={styles.ctrlBtn}
                onClick={prev}
                disabled={index <= 0}
                aria-label="Previous mashup"
              >
                ⏮
              </button>
              <button
                type="button"
                className={`${styles.ctrlBtn} ${styles.playBtn}`}
                onClick={toggle}
                aria-label={isPlaying ? 'Pause' : 'Play'}
              >
                {isPlaying ? '⏸' : '▶'}
              </button>
              <button
                type="button"
                className={styles.ctrlBtn}
                onClick={next}
                disabled={index >= list.length - 1}
                aria-label="Next mashup"
              >
                ⏭
              </button>
            </div>
            <div className={styles.seek}>
              <SeekBar position={position} duration={duration} onSeek={seek} />
            </div>
          </div>

          <div className={styles.volume}>
            <VolumeControl
              volume={volume}
              muted={muted}
              onVolume={setVolume}
              onToggleMute={toggleMute}
            />
          </div>
        </div>
      )}
    </div>
  )
}

import SeekBar from '../player/SeekBar'
import VolumeControl from '../player/VolumeControl'
import {
  ChevronUpIcon,
  HeartFillIcon,
  HeartIcon,
  NextIcon,
  PauseIcon,
  PlayIcon,
  PrevIcon,
  QueueIcon,
  RepeatIcon,
  RepeatOneIcon,
  ShuffleIcon,
} from '../player/icons'
import type { Mashup } from '../../types'
import type { MashupPlayer as MashupPlayerState } from '../../hooks/useMashupPlayer'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import styles from './MashupPlayer.module.css'

interface MashupPlayerProps {
  player: MashupPlayerState
  onToggleLike?: (m: Mashup) => void
  /** Open the expanded "Now playing" overlay. */
  onExpand: () => void
  /** Whether the side "Now playing" panel is currently shown. */
  queueOpen: boolean
  onToggleQueue: () => void
}

const REPEAT_LABEL: Record<MashupPlayerState['repeat'], string> = {
  off: 'Repeat off',
  all: 'Repeat all',
  one: 'Repeat one',
}

export default function MashupPlayer({
  player,
  onToggleLike,
  onExpand,
  queueOpen,
  onToggleQueue,
}: MashupPlayerProps) {
  const {
    audioRef,
    current,
    isPlaying,
    position,
    duration,
    buffered,
    shuffle,
    repeat,
    volume,
    muted,
    setVolume,
    toggleMute,
    toggleShuffle,
    cycleRepeat,
    toggle,
    next,
    prev,
    seek,
    index,
    list,
  } = player

  const queueCount = index >= 0 ? Math.max(0, list.length - index - 1) : 0

  return (
    <div className={styles.bar} hidden={!current}>
      {/* Always mounted so the hook's ref is available even with nothing playing. */}
      <audio ref={audioRef} preload="auto" />

      {current && (
        <div className={styles.inner}>
          {/* ── left: now-playing meta ─────────────────────────────── */}
          <div className={styles.meta}>
            {current.cover_url ? (
              <img className={styles.cover} src={current.cover_url} alt="" />
            ) : (
              <div className={styles.cover} style={{ background: tintForId(current.id) }}>
                <span className={styles.glyph} aria-hidden="true">{monoGlyph(current.title)}</span>
              </div>
            )}
            <div className={styles.text}>
              <div className={styles.title} title={current.title}>
                {current.title}
              </div>
              <div className={styles.artist}>{current.artist || 'Unknown artist'}</div>
            </div>
            {onToggleLike && (
              <button
                type="button"
                className={`${styles.iconBtn} ${styles.likeBtn} ${current.liked ? styles.likeBtnOn : ''}`}
                onClick={() => onToggleLike(current)}
                aria-pressed={!!current.liked}
                aria-label={current.liked ? 'Unlike' : 'Like'}
              >
                {current.liked ? <HeartFillIcon /> : <HeartIcon />}
              </button>
            )}
          </div>

          {/* ── center: transport + seek ──────────────────────────── */}
          <div className={styles.center}>
            <div className={styles.controls}>
              <button
                type="button"
                className={`${styles.iconBtn} ${styles.optional} ${shuffle ? styles.on : ''}`}
                onClick={toggleShuffle}
                aria-pressed={shuffle}
                aria-label="Shuffle"
              >
                <ShuffleIcon />
                {shuffle && <span className={styles.dot} aria-hidden="true" />}
              </button>
              <button
                type="button"
                className={styles.iconBtn}
                onClick={prev}
                disabled={index <= 0}
                aria-label="Previous mashup"
              >
                <PrevIcon size={20} />
              </button>
              <button
                type="button"
                className={styles.playBtn}
                onClick={toggle}
                aria-label={isPlaying ? 'Pause' : 'Play'}
              >
                {isPlaying ? <PauseIcon size={16} /> : <PlayIcon size={16} />}
              </button>
              <button
                type="button"
                className={styles.iconBtn}
                onClick={next}
                disabled={index >= list.length - 1 && repeat !== 'all'}
                aria-label="Next mashup"
              >
                <NextIcon size={20} />
              </button>
              <button
                type="button"
                className={`${styles.iconBtn} ${styles.optional} ${repeat !== 'off' ? styles.on : ''}`}
                onClick={cycleRepeat}
                aria-pressed={repeat !== 'off'}
                aria-label={REPEAT_LABEL[repeat]}
              >
                {repeat === 'one' ? <RepeatOneIcon /> : <RepeatIcon />}
                {repeat !== 'off' && <span className={styles.dot} aria-hidden="true" />}
              </button>
            </div>
            <div className={styles.seek}>
              <SeekBar
                position={position}
                duration={duration}
                buffered={buffered}
                onSeek={seek}
              />
            </div>
          </div>

          {/* ── right: queue + volume + expand ────────────────────── */}
          <div className={styles.right}>
            <span className={styles.qwrap}>
              <button
                type="button"
                className={`${styles.iconBtn} ${queueOpen ? styles.on : ''}`}
                onClick={onToggleQueue}
                aria-pressed={queueOpen}
                aria-label="Queue"
              >
                <QueueIcon />
              </button>
              {queueCount > 0 && <span className={styles.qcount}>{queueCount}</span>}
            </span>
            <VolumeControl
              variant="bar"
              volume={volume}
              muted={muted}
              onVolume={setVolume}
              onToggleMute={toggleMute}
            />
            <button
              type="button"
              className={styles.iconBtn}
              onClick={onExpand}
              aria-label="Expand player"
            >
              <ChevronUpIcon />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

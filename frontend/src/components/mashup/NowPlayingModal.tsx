import { useEffect } from 'react'
import type { Mashup } from '../../types'
import type { MashupPlayer } from '../../hooks/useMashupPlayer'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import SeekBar from '../player/SeekBar'
import {
  ChevronDownIcon,
  HeartFillIcon,
  HeartIcon,
  NextIcon,
  PauseIcon,
  PlayIcon,
  PrevIcon,
  RepeatIcon,
  RepeatOneIcon,
  ShuffleIcon,
} from '../player/icons'
import styles from './NowPlayingModal.module.css'

interface NowPlayingModalProps {
  player: MashupPlayer
  queue: Mashup[]
  onToggleLike: (m: Mashup) => void
  onOpenProfile?: (userId: string) => void
  onClose: () => void
}

const REPEAT_LABEL = { off: 'Repeat off', all: 'Repeat all', one: 'Repeat one' } as const

function Art({ mashup, className }: { mashup: Mashup; className: string }) {
  if (mashup.cover_url) {
    return <img className={className} src={mashup.cover_url} alt="" />
  }
  return (
    <div className={className} style={{ background: tintForId(mashup.id) }}>
      <span className={styles.glyph} aria-hidden="true">{monoGlyph(mashup.title)}</span>
    </div>
  )
}

export default function NowPlayingModal({
  player,
  queue,
  onToggleLike,
  onOpenProfile,
  onClose,
}: NowPlayingModalProps) {
  const {
    current,
    isPlaying,
    position,
    duration,
    buffered,
    shuffle,
    repeat,
    toggle,
    next,
    prev,
    seek,
    index,
    list,
    toggleShuffle,
    cycleRepeat,
    play,
  } = player

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  if (!current) return null

  return (
    <div
      className="modal-overlay"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div className={styles.panel} role="dialog" aria-modal="true" aria-label="Now playing">
        <div className={styles.head}>
          <h2>Now playing</h2>
          <button
            type="button"
            className={styles.iconBtn}
            onClick={onClose}
            aria-label="Collapse player"
          >
            <ChevronDownIcon />
          </button>
        </div>

        <div className={styles.body}>
          <Art mashup={current} className={styles.art} />
          <div className={styles.info}>
            <div className={styles.title} title={current.title}>{current.title}</div>
            <div className={styles.meta}>
              {current.artist || 'Unknown artist'}
              {current.owner_name && current.owner_id && onOpenProfile ? (
                <>
                  {' · by '}
                  <button
                    type="button"
                    className={styles.owner}
                    onClick={() => onOpenProfile(current.owner_id)}
                  >
                    {current.owner_name}
                  </button>
                </>
              ) : current.owner_name ? (
                ` · by ${current.owner_name}`
              ) : null}
            </div>

            <div className={styles.statline}>
              <button
                type="button"
                className={`${styles.likeBtn} ${current.liked ? styles.likeBtnOn : ''}`}
                onClick={() => onToggleLike(current)}
                aria-pressed={!!current.liked}
                aria-label={current.liked ? `Unlike ${current.title}` : `Like ${current.title}`}
              >
                {current.liked ? <HeartFillIcon size={20} /> : <HeartIcon size={20} />}
                {current.likes}
              </button>
              {current.status === 'ready' && (
                <span className={styles.dur}>{formatTime(current.duration)}</span>
              )}
            </div>

            <div className={styles.seek}>
              <SeekBar position={position} duration={duration} buffered={buffered} onSeek={seek} />
            </div>
          </div>
        </div>

        <div className={styles.transport}>
          <button
            type="button"
            className={`${styles.tbtn} ${styles.tbtnSm} ${shuffle ? styles.on : ''}`}
            onClick={toggleShuffle}
            aria-pressed={shuffle}
            aria-label="Shuffle"
          >
            <ShuffleIcon size={20} />
            {shuffle && <span className={styles.dot} aria-hidden="true" />}
          </button>
          <button
            type="button"
            className={styles.tbtn}
            onClick={prev}
            disabled={index <= 0}
            aria-label="Previous mashup"
          >
            <PrevIcon size={26} />
          </button>
          <button
            type="button"
            className={styles.playLg}
            onClick={toggle}
            aria-label={isPlaying ? 'Pause' : 'Play'}
          >
            {isPlaying ? <PauseIcon size={24} /> : <PlayIcon size={24} />}
          </button>
          <button
            type="button"
            className={styles.tbtn}
            onClick={next}
            disabled={index >= list.length - 1 && repeat !== 'all'}
            aria-label="Next mashup"
          >
            <NextIcon size={26} />
          </button>
          <button
            type="button"
            className={`${styles.tbtn} ${styles.tbtnSm} ${repeat !== 'off' ? styles.on : ''}`}
            onClick={cycleRepeat}
            aria-pressed={repeat !== 'off'}
            aria-label={REPEAT_LABEL[repeat]}
          >
            {repeat === 'one' ? <RepeatOneIcon size={20} /> : <RepeatIcon size={20} />}
            {repeat !== 'off' && <span className={styles.dot} aria-hidden="true" />}
          </button>
        </div>

        <h3 className={styles.queueHead}>Next in queue</h3>
        {queue.length === 0 ? (
          <p className={styles.queueEmpty}>End of the list.</p>
        ) : (
          queue.map((m) => (
            <button key={m.id} type="button" className={styles.qrow} onClick={() => play(m)}>
              <Art mashup={m} className={styles.qart} />
              <span className={styles.qbody}>
                <span className={styles.qtitle} title={m.title}>{m.title}</span>
                <span className={styles.qsub}>{m.artist || 'Unknown artist'}</span>
              </span>
              {m.status === 'ready' && <span className={styles.qdur}>{formatTime(m.duration)}</span>}
            </button>
          ))
        )}
      </div>
    </div>
  )
}

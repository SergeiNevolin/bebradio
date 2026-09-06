import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../lib/api'
import styles from './Karaoke.module.css'

interface Cue {
  start: number
  dur: number
  text: string
}

type Status = 'loading' | 'ready' | 'empty' | 'error'

interface KaraokeProps {
  roomId: string
  /** Re-fetches whenever this changes (i.e. the current track advanced). */
  trackId: string
  /** Live playback position in seconds, used to highlight the current line. */
  currentTime: number
}

export default function Karaoke({ roomId, trackId, currentTime }: KaraokeProps) {
  const [status, setStatus] = useState<Status>('loading')
  const [cues, setCues] = useState<Cue[]>([])
  const [auto, setAuto] = useState(false)
  const activeRef = useRef<HTMLLIElement | null>(null)

  useEffect(() => {
    let cancelled = false
    setStatus('loading')
    setCues([])
    api.getLyrics(roomId)
      .then((data) => {
        if (cancelled) return
        const list = data.cues ?? []
        setCues(list)
        setAuto(data.auto ?? false)
        setStatus(list.length ? 'ready' : 'empty')
      })
      .catch(() => {
        if (!cancelled) setStatus('error')
      })
    return () => {
      cancelled = true
    }
  }, [roomId, trackId])

  const activeIndex = useMemo(() => {
    if (!cues.length) return -1
    // Binary search for the last cue that has already started.
    let lo = 0
    let hi = cues.length - 1
    let ans = -1
    while (lo <= hi) {
      const mid = (lo + hi) >> 1
      if (cues[mid].start <= currentTime + 0.15) {
        ans = mid
        lo = mid + 1
      } else {
        hi = mid - 1
      }
    }
    // Once the final line is well past, stop highlighting anything.
    if (ans === cues.length - 1 && ans >= 0) {
      const c = cues[ans]
      const end = c.dur > 0 ? c.start + c.dur : c.start + 6
      if (currentTime > end + 1) return -1
    }
    return ans
  }, [cues, currentTime])

  useEffect(() => {
    activeRef.current?.scrollIntoView?.({ block: 'center', behavior: 'smooth' })
  }, [activeIndex])

  if (status === 'loading') return <div className={`${styles.karaoke} ${styles.karaokeMsg}`}>Loading lyrics…</div>
  if (status === 'error') return <div className={`${styles.karaoke} ${styles.karaokeMsg}`}>Couldn't load lyrics.</div>
  if (status === 'empty') return <div className={`${styles.karaoke} ${styles.karaokeMsg}`}>No lyrics for this track.</div>

  return (
    <div className={styles.karaoke}>
      {auto && <div className={styles.karaokeNote}>Auto-generated captions — timing may drift</div>}
      <ul className={styles.karaokeLines}>
        {cues.map((c, i) => (
          <li
            key={`${i}-${c.start}`}
            ref={i === activeIndex ? activeRef : null}
            className={`${styles.karaokeLine}${i === activeIndex ? ` ${styles.karaokeLineActive}` : ''}${i < activeIndex ? ` ${styles.karaokeLinePast}` : ''}`}
          >
            {c.text}
          </li>
        ))}
      </ul>
    </div>
  )
}

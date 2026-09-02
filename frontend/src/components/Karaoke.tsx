import { useEffect, useMemo, useRef, useState } from 'react'

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
  /** Jump playback to a line's timestamp when it is clicked. */
  onSeek?: (seconds: number) => void
}

export default function Karaoke({ roomId, trackId, currentTime, onSeek }: KaraokeProps) {
  const [status, setStatus] = useState<Status>('loading')
  const [cues, setCues] = useState<Cue[]>([])
  const [auto, setAuto] = useState(false)
  const activeRef = useRef<HTMLLIElement | null>(null)

  useEffect(() => {
    let cancelled = false
    setStatus('loading')
    setCues([])
    fetch(`/api/rooms/${roomId}/lyrics`)
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error('request failed'))))
      .then((data: { available?: boolean; auto?: boolean; cues?: Cue[] }) => {
        if (cancelled) return
        const list = data.cues ?? []
        setCues(list)
        setAuto(Boolean(data.auto))
        setStatus(data.available && list.length ? 'ready' : 'empty')
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

  if (status === 'loading') return <div className="karaoke karaoke-msg">Loading lyrics…</div>
  if (status === 'error') return <div className="karaoke karaoke-msg">Couldn’t load lyrics.</div>
  if (status === 'empty') return <div className="karaoke karaoke-msg">No lyrics for this track.</div>

  return (
    <div className="karaoke">
      {auto && <div className="karaoke-note">Auto-generated captions — timing may drift</div>}
      <ul className="karaoke-lines">
        {cues.map((c, i) => (
          <li
            key={`${i}-${c.start}`}
            ref={i === activeIndex ? activeRef : null}
            className={
              'karaoke-line' +
              (i === activeIndex ? ' is-active' : '') +
              (i < activeIndex ? ' is-past' : '')
            }
            onClick={onSeek ? () => onSeek(c.start) : undefined}
          >
            {c.text}
          </li>
        ))}
      </ul>
    </div>
  )
}

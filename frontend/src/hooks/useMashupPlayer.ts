import { useCallback, useEffect, useRef, useState } from 'react'
import type { Mashup } from '../types'
import { useVolume } from './useVolume'

/**
 * A single-<audio> player for the standalone /mashup page. No dual decks and no
 * server timeline sync — useGaplessPlayer is built around a shared room clock and
 * would be overkill here. Volume comes from the shared useVolume() hook so it
 * stays in step with the room player and survives reloads.
 *
 * A compact snapshot (current track, playback offset, shuffle and repeat) is
 * mirrored to localStorage so a window reload resumes where it left off. Volume
 * is already persisted by useVolume().
 *
 * The component that consumes this must render `<audio ref={audioRef} />`.
 */

export type RepeatMode = 'off' | 'all' | 'one'

const STORAGE_KEY = 'mashup-player'

interface PersistedState {
  /** id of the mashup that was playing. */
  id: string
  /** playback offset in seconds. */
  position: number
  /** whether it was playing when the page was left. */
  playing: boolean
  shuffle: boolean
  repeat: RepeatMode
}

/** Read and validate the saved snapshot; any problem yields null. */
function loadPersisted(): PersistedState | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const p = JSON.parse(raw) as Partial<PersistedState>
    if (!p || typeof p.id !== 'string') return null
    return {
      id: p.id,
      position: typeof p.position === 'number' && p.position > 0 ? p.position : 0,
      playing: !!p.playing,
      shuffle: !!p.shuffle,
      repeat: p.repeat === 'all' || p.repeat === 'one' ? p.repeat : 'off',
    }
  } catch {
    return null
  }
}

/**
 * The walk order over `list`, as indices. Without shuffle it is `[0..n)`. With
 * shuffle it is a Fisher–Yates permutation with the current track pulled to the
 * front, so `next`/`prev` step through every track once before wrapping.
 */
function buildOrder(n: number, shuffled: boolean, currentIndex: number): number[] {
  const order = Array.from({ length: n }, (_, i) => i)
  if (!shuffled || n <= 1) return order
  for (let i = order.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[order[i], order[j]] = [order[j], order[i]]
  }
  if (currentIndex >= 0 && currentIndex < n) {
    const pos = order.indexOf(currentIndex)
    if (pos > 0) {
      order.splice(pos, 1)
      order.unshift(currentIndex)
    }
  }
  return order
}

export function useMashupPlayer() {
  const audioRef = useRef<HTMLAudioElement>(null)
  // Read the saved snapshot once, on mount.
  const [boot] = useState(loadPersisted)
  const [list, setList] = useState<Mashup[]>([])
  const [index, setIndex] = useState(-1)
  const [isPlaying, setIsPlaying] = useState(false)
  const [position, setPosition] = useState(boot?.position ?? 0)
  const [duration, setDuration] = useState(0)
  const [buffered, setBuffered] = useState(0)
  const [shuffle, setShuffle] = useState(boot?.shuffle ?? false)
  const [repeat, setRepeat] = useState<RepeatMode>(boot?.repeat ?? 'off')
  const [order, setOrder] = useState<number[]>([])
  const { volume, muted, setVolume, toggleMute } = useVolume()

  const current = index >= 0 && index < list.length ? list[index] : null

  // Refs mirror the latest values so the stable callbacks below never close over
  // stale state (they are handed to <audio> listeners once and kept).
  const orderRef = useRef(order)
  orderRef.current = order
  const indexRef = useRef(index)
  indexRef.current = index
  const repeatRef = useRef(repeat)
  repeatRef.current = repeat
  const shuffleRef = useRef(shuffle)
  shuffleRef.current = shuffle
  const currentRef = useRef(current)
  currentRef.current = current

  // Reload-restore plumbing. `pendingRestoreRef` holds the saved snapshot until
  // the walkable list arrives with the matching track; the two `restore*` refs
  // then carry the offset / resume flag into the load effect below.
  const pendingRestoreRef = useRef<PersistedState | null>(boot)
  const restorePosRef = useRef<number | null>(null)
  const restorePlayRef = useRef(false)

  // Rebuild the walk order whenever the list grows or shrinks. The current track
  // is preserved: buildOrder keeps it at the head under shuffle, and plain order
  // is index-stable anyway.
  useEffect(() => {
    setOrder(buildOrder(list.length, shuffle, indexRef.current))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [list.length])

  const toggleShuffle = useCallback(() => {
    setShuffle((on) => {
      const nextOn = !on
      setOrder(buildOrder(list.length, nextOn, indexRef.current))
      return nextOn
    })
  }, [list.length])

  const cycleRepeat = useCallback(() => {
    setRepeat((r) => (r === 'off' ? 'all' : r === 'all' ? 'one' : 'off'))
  }, [])

  const next = useCallback(() => {
    setIndex((i) => {
      const ord = orderRef.current
      const pos = ord.indexOf(i)
      if (pos === -1) return i
      if (pos + 1 < ord.length) return ord[pos + 1]
      // End of the walk: wrap only when repeating the whole list.
      return repeatRef.current === 'all' && ord.length > 0 ? ord[0] : i
    })
  }, [])

  const prev = useCallback(() => {
    setIndex((i) => {
      const ord = orderRef.current
      const pos = ord.indexOf(i)
      if (pos <= 0) return i
      return ord[pos - 1]
    })
  }, [])

  const playAt = useCallback(
    (i: number) => {
      if (i >= 0 && i < list.length) setIndex(i)
    },
    [list.length],
  )

  const play = useCallback(
    (m: Mashup) => {
      const i = list.findIndex((x) => x.id === m.id)
      if (i >= 0) setIndex(i)
    },
    [list],
  )

  const toggle = useCallback(() => {
    const el = audioRef.current
    if (!el || !current) return
    if (el.paused) el.play().catch(() => setIsPlaying(false))
    else el.pause()
  }, [current])

  const seek = useCallback((seconds: number) => {
    const el = audioRef.current
    if (!el) return
    el.currentTime = seconds
    setPosition(seconds)
  }, [])

  // Keep the media element volume in sync.
  useEffect(() => {
    if (audioRef.current) audioRef.current.volume = muted ? 0 : volume
  }, [volume, muted, current])

  // Once the walkable list arrives, jump back to the track and offset saved from
  // the previous session. Fires once: the pending snapshot is dropped as soon as
  // the track is found, the user picks something first, or it proves gone.
  useEffect(() => {
    const pending = pendingRestoreRef.current
    if (!pending) return
    if (indexRef.current >= 0) {
      pendingRestoreRef.current = null
      return
    }
    if (list.length === 0) return
    const i = list.findIndex((x) => x.id === pending.id)
    if (i < 0) return // maybe in a section that has not loaded yet — keep waiting
    pendingRestoreRef.current = null
    restorePosRef.current = pending.position
    restorePlayRef.current = pending.playing
    setIndex(i)
  }, [list])

  // Load and auto-play whenever the selected mashup changes.
  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    setBuffered(0)
    if (!current || !current.stream_url) {
      el.pause()
      el.removeAttribute('src')
      el.load()
      setIsPlaying(false)
      setPosition(0)
      setDuration(0)
      return
    }

    // A reload-restore seeks to the saved offset once metadata is in, and only
    // resumes playback if it was playing when the page was left (the browser may
    // still block that until the first interaction).
    const restorePos = restorePosRef.current
    const restorePlay = restorePlayRef.current
    restorePosRef.current = null
    restorePlayRef.current = false

    el.src = current.stream_url
    el.load()
    el.volume = muted ? 0 : volume

    if (restorePos != null) {
      setPosition(restorePos)
      const applyOffset = () => {
        try {
          el.currentTime = restorePos
        } catch {
          /* not seekable yet */
        }
        el.removeEventListener('loadedmetadata', applyOffset)
      }
      el.addEventListener('loadedmetadata', applyOffset)
      if (restorePlay) {
        el.play().then(() => setIsPlaying(true)).catch(() => setIsPlaying(false))
      } else {
        setIsPlaying(false)
      }
      return
    }

    el.play().then(() => setIsPlaying(true)).catch(() => setIsPlaying(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id])

  // Wire element events -> state.
  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    const onTime = () => setPosition(el.currentTime)
    const onMeta = () => setDuration(Number.isFinite(el.duration) ? el.duration : 0)
    const onProgress = () => {
      const b = el.buffered
      setBuffered(b.length ? b.end(b.length - 1) : 0)
    }
    const onEnded = () => {
      if (repeatRef.current === 'one') {
        el.currentTime = 0
        setPosition(0)
        el.play().catch(() => setIsPlaying(false))
        return
      }
      next()
    }
    const onPlay = () => setIsPlaying(true)
    const onPause = () => setIsPlaying(false)
    el.addEventListener('timeupdate', onTime)
    el.addEventListener('loadedmetadata', onMeta)
    el.addEventListener('durationchange', onMeta)
    el.addEventListener('progress', onProgress)
    el.addEventListener('ended', onEnded)
    el.addEventListener('play', onPlay)
    el.addEventListener('pause', onPause)
    return () => {
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('loadedmetadata', onMeta)
      el.removeEventListener('durationchange', onMeta)
      el.removeEventListener('progress', onProgress)
      el.removeEventListener('ended', onEnded)
      el.removeEventListener('play', onPlay)
      el.removeEventListener('pause', onPause)
    }
  }, [next])

  // Write the current snapshot to localStorage. Cheap enough to call often.
  // With nothing playing we leave any existing snapshot untouched — on a reload
  // `current` is briefly null before the restore lands, and clearing here would
  // throw the saved position away.
  const persist = useCallback(() => {
    try {
      const cur = currentRef.current
      const el = audioRef.current
      if (!cur) return
      const snapshot: PersistedState = {
        id: cur.id,
        position: el && Number.isFinite(el.currentTime) ? el.currentTime : 0,
        playing: !!el && !el.paused,
        shuffle: shuffleRef.current,
        repeat: repeatRef.current,
      }
      localStorage.setItem(STORAGE_KEY, JSON.stringify(snapshot))
    } catch {
      /* storage unavailable — skip */
    }
  }, [])

  // Save on the events that change what we'd want to restore…
  useEffect(() => {
    persist()
  }, [current?.id, shuffle, repeat, persist])

  // …and periodically / when the tab goes away, to capture the moving offset.
  useEffect(() => {
    const save = () => persist()
    const timer = window.setInterval(save, 5000)
    window.addEventListener('pagehide', save)
    window.addEventListener('beforeunload', save)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('pagehide', save)
      window.removeEventListener('beforeunload', save)
    }
  }, [persist])

  // Stop playback when the page unmounts (leaving /mashup), saving first so a
  // later return to /mashup still resumes.
  useEffect(() => {
    return () => {
      persist()
      const el = audioRef.current
      if (el) {
        el.pause()
        el.removeAttribute('src')
      }
    }
  }, [persist])

  // If the list shrinks past the current index (e.g. after a delete), clamp it.
  useEffect(() => {
    if (index >= list.length) setIndex(list.length - 1)
  }, [list.length, index])

  return {
    audioRef,
    list,
    setList,
    index,
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
    play,
    playAt,
    toggle,
    next,
    prev,
    seek,
  }
}

export type MashupPlayer = ReturnType<typeof useMashupPlayer>

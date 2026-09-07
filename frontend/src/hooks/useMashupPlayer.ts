import { useCallback, useEffect, useRef, useState } from 'react'
import type { Mashup } from '../types'
import { useVolume } from './useVolume'

/**
 * A single-<audio> player for the standalone /mashup page. No dual decks and no
 * server timeline sync — useGaplessPlayer is built around a shared room clock and
 * would be overkill here. Volume comes from the shared useVolume() hook so it
 * stays in step with the room player and survives reloads.
 *
 * The component that consumes this must render `<audio ref={audioRef} />`.
 */

export type RepeatMode = 'off' | 'all' | 'one'

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
  const [list, setList] = useState<Mashup[]>([])
  const [index, setIndex] = useState(-1)
  const [isPlaying, setIsPlaying] = useState(false)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [buffered, setBuffered] = useState(0)
  const [shuffle, setShuffle] = useState(false)
  const [repeat, setRepeat] = useState<RepeatMode>('off')
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
    el.src = current.stream_url
    el.load()
    el.volume = muted ? 0 : volume
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

  // Stop playback when the page unmounts (leaving /mashup).
  useEffect(() => {
    return () => {
      const el = audioRef.current
      if (el) {
        el.pause()
        el.removeAttribute('src')
      }
    }
  }, [])

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

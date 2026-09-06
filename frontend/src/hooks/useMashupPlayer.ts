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
export function useMashupPlayer() {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [list, setList] = useState<Mashup[]>([])
  const [index, setIndex] = useState(-1)
  const [isPlaying, setIsPlaying] = useState(false)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const { volume, muted, setVolume, toggleMute } = useVolume()

  const current = index >= 0 && index < list.length ? list[index] : null

  const next = useCallback(() => {
    setIndex((i) => (i + 1 < list.length ? i + 1 : i))
  }, [list.length])

  const prev = useCallback(() => {
    setIndex((i) => (i - 1 >= 0 ? i - 1 : i))
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
    const onEnded = () => next()
    const onPlay = () => setIsPlaying(true)
    const onPause = () => setIsPlaying(false)
    el.addEventListener('timeupdate', onTime)
    el.addEventListener('loadedmetadata', onMeta)
    el.addEventListener('durationchange', onMeta)
    el.addEventListener('ended', onEnded)
    el.addEventListener('play', onPlay)
    el.addEventListener('pause', onPause)
    return () => {
      el.removeEventListener('timeupdate', onTime)
      el.removeEventListener('loadedmetadata', onMeta)
      el.removeEventListener('durationchange', onMeta)
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
    volume,
    muted,
    setVolume,
    toggleMute,
    play,
    playAt,
    toggle,
    next,
    prev,
    seek,
  }
}

export type MashupPlayer = ReturnType<typeof useMashupPlayer>

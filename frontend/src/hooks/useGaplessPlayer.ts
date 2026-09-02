import { useCallback, useEffect, useRef, useState } from 'react'

interface GaplessOptions {
  /** Current track id; a change swaps the audible source. */
  trackId: string | undefined
  /** Playable stream URL for the current track. */
  src: string | undefined
  /** Next queued track id, prefetched into the idle deck. */
  nextTrackId: string | undefined
  /** Playable stream URL for the next track. */
  nextSrc: string | undefined
  /** Whether the room's shared playback is running. */
  isPlaying: boolean
  /** Authoritative position (seconds) from the server, for drift correction. */
  serverPosition: number
  /** Target volume 0–1. */
  volume: number
  /** Whether playback is muted. */
  muted: boolean
  /** Overlap length in seconds; 0 disables crossfading. */
  crossfadeSeconds: number
  /** Advance the queue (natural end, or the moment a crossfade begins). */
  onEnded: () => void
  /** Report position to the server every few seconds while playing. */
  onSync: (position: number) => void
}

interface GaplessPlayer {
  /** Two <audio> elements to render; deck A and deck B. */
  deckRefs: [React.RefObject<HTMLAudioElement>, React.RefObject<HTMLAudioElement>]
  position: number
  duration: number
  needsGesture: boolean
  /** True while two tracks are overlapping. */
  crossfading: boolean
  unlock: () => void
  seek: (seconds: number) => void
}

const SYNC_INTERVAL_MS = 5000
const TICK_INTERVAL_MS = 250
const FADE_STEP_MS = 50
const DRIFT_TOLERANCE_S = 1.5
/** Don't crossfade tracks that are barely longer than the overlap itself. */
const MIN_TRACK_FACTOR = 1.5

/**
 * Two-deck audio player for seamless track transitions:
 *
 * - **Prefetch** — the next queued track is loaded into the idle deck as soon
 *   as it is known, so the hand-off never waits on the network.
 * - **Crossfade** — a few seconds before the current track ends the idle deck
 *   starts and the two decks' volumes are ramped past each other. The server
 *   is told to advance at that moment so every client's shared clock moves
 *   together (non-crossfading clients simply cut a few seconds early).
 */
export function useGaplessPlayer({
  trackId,
  src,
  nextTrackId,
  nextSrc,
  isPlaying,
  serverPosition,
  volume,
  muted,
  crossfadeSeconds,
  onEnded,
  onSync,
}: GaplessOptions): GaplessPlayer {
  const deckA = useRef<HTMLAudioElement>(null)
  const deckB = useRef<HTMLAudioElement>(null)
  const deckRefs: GaplessPlayer['deckRefs'] = [deckA, deckB]

  const [activeIdx, setActiveIdx] = useState(0)
  const activeIdxRef = useRef(0)
  activeIdxRef.current = activeIdx

  // trackId currently loaded into each deck ('' = nothing).
  const loadedRef = useRef<[string, string]>(['', ''])
  // trackId of the outgoing deck while a crossfade is in flight (else null).
  const fadingFromRef = useRef<string | null>(null)
  const fadeTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [needsGesture, setNeedsGesture] = useState(false)
  const [crossfading, setCrossfading] = useState(false)

  const effVol = muted ? 0 : volume

  // Live values for the timer callbacks, kept off the effect dependency lists.
  const paramsRef = useRef({ isPlaying, crossfadeSeconds, nextTrackId, nextSrc, effVol, onEnded, onSync })
  paramsRef.current = { isPlaying, crossfadeSeconds, nextTrackId, nextSrc, effVol, onEnded, onSync }

  const cancelFade = useCallback(() => {
    if (fadeTimerRef.current) {
      clearInterval(fadeTimerRef.current)
      fadeTimerRef.current = null
    }
    fadingFromRef.current = null
    setCrossfading(false)
  }, [])

  // --- Prefetch the next track into whichever deck isn't playing ---
  const prefetchIdle = useCallback(() => {
    if (fadingFromRef.current) return
    const idleIdx = 1 - activeIdxRef.current
    const idle = deckRefs[idleIdx].current
    if (!idle) return
    const { nextTrackId: nid, nextSrc: nsrc } = paramsRef.current
    if (!nid || !nsrc) {
      if (loadedRef.current[idleIdx]) {
        loadedRef.current[idleIdx] = ''
        idle.removeAttribute('src')
        idle.load()
      }
      return
    }
    if (loadedRef.current[idleIdx] !== nid) {
      loadedRef.current[idleIdx] = nid
      idle.src = nsrc
      idle.preload = 'auto'
      idle.load()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    prefetchIdle()
  }, [nextTrackId, nextSrc, activeIdx, prefetchIdle])

  // --- Load the current track into the active deck (or promote the idle one) ---
  useEffect(() => {
    const idx = activeIdxRef.current
    const active = deckRefs[idx].current
    if (!active) return

    if (!src) {
      // Queue emptied. Tear both decks down so nothing keeps playing.
      cancelFade()
      for (let i = 0; i < 2; i++) {
        const d = deckRefs[i].current
        if (d) {
          d.pause()
          d.removeAttribute('src')
          d.load()
        }
        loadedRef.current[i] = ''
      }
      setPosition(0)
      setDuration(0)
      setNeedsGesture(false)
      return
    }

    if (loadedRef.current[idx] === trackId) return

    // The idle deck is already playing this track (crossfade hand-off) — just
    // make it the active one instead of reloading.
    const idleIdx = 1 - idx
    if (loadedRef.current[idleIdx] === trackId) {
      activeIdxRef.current = idleIdx
      setActiveIdx(idleIdx)
      return
    }

    loadedRef.current[idx] = trackId ?? ''
    active.src = src
    active.load()
    setPosition(0)
    setDuration(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trackId, src, activeIdx])

  // --- Track length, read from the active deck ---
  useEffect(() => {
    const active = deckRefs[activeIdx].current
    if (!active) return
    const onMeta = () => setDuration(active.duration || 0)
    onMeta()
    active.addEventListener('loadedmetadata', onMeta)
    return () => active.removeEventListener('loadedmetadata', onMeta)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeIdx])

  // --- Play / pause the active deck to match the shared state ---
  useEffect(() => {
    const active = deckRefs[activeIdx].current
    if (!active || !src || loadedRef.current[activeIdx] !== trackId) return

    if (isPlaying) {
      active.play().then(() => setNeedsGesture(false)).catch(() => setNeedsGesture(true))
    } else {
      cancelFade()
      deckRefs[0].current?.pause()
      deckRefs[1].current?.pause()
      setNeedsGesture(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isPlaying, trackId, src, activeIdx])

  // --- Apply volume (unless a ramp currently owns the decks) ---
  useEffect(() => {
    if (fadeTimerRef.current) return
    for (const ref of deckRefs) {
      if (ref.current) ref.current.volume = effVol
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effVol, activeIdx, crossfading])

  // --- After autoplay was blocked, the first interaction unlocks it ---
  useEffect(() => {
    if (!needsGesture || !isPlaying) return
    const resume = () => {
      deckRefs[activeIdxRef.current].current?.play().then(() => setNeedsGesture(false)).catch(() => {})
    }
    window.addEventListener('pointerdown', resume)
    window.addEventListener('keydown', resume)
    return () => {
      window.removeEventListener('pointerdown', resume)
      window.removeEventListener('keydown', resume)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [needsGesture, isPlaying])

  // --- Correct drift against the server (skipped mid-crossfade) ---
  useEffect(() => {
    const active = deckRefs[activeIdx].current
    if (!active || !src || fadeTimerRef.current) return
    if (Math.abs(active.currentTime - serverPosition) > DRIFT_TOLERANCE_S) {
      active.currentTime = serverPosition
      setPosition(serverPosition)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverPosition, src, activeIdx])

  // --- Start a crossfade when the active deck nears its end ---
  const maybeStartCrossfade = useCallback((active: HTMLAudioElement) => {
    const p = paramsRef.current
    if (fadeTimerRef.current) return
    if (!p.isPlaying || p.crossfadeSeconds <= 0) return
    if (!p.nextTrackId || !p.nextSrc) return

    const dur = active.duration
    if (!dur || !isFinite(dur) || dur <= p.crossfadeSeconds * MIN_TRACK_FACTOR) return
    const remaining = dur - active.currentTime
    if (remaining > p.crossfadeSeconds || remaining <= 0) return

    const idleIdx = 1 - activeIdxRef.current
    const idle = deckRefs[idleIdx].current
    if (!idle || loadedRef.current[idleIdx] !== p.nextTrackId) return
    if (idle.readyState < 3) return // HAVE_FUTURE_DATA

    const ceil = p.effVol
    const seconds = Math.max(remaining, 0.3)
    fadingFromRef.current = loadedRef.current[activeIdxRef.current]
    setCrossfading(true)

    try {
      idle.currentTime = 0
    } catch {
      /* ignore */
    }
    idle.volume = 0
    idle.play().catch(() => {})

    const steps = Math.max(1, Math.round((seconds * 1000) / FADE_STEP_MS))
    let i = 0
    fadeTimerRef.current = setInterval(() => {
      i += 1
      const k = Math.min(1, i / steps)
      active.volume = Math.max(0, ceil * (1 - k))
      idle.volume = Math.min(ceil, ceil * k)
      if (k >= 1) {
        clearInterval(fadeTimerRef.current!)
        fadeTimerRef.current = null
        fadingFromRef.current = null
        active.pause()
        setCrossfading(false)
        prefetchIdle()
      }
    }, FADE_STEP_MS)

    // Move the shared clock now, so the room advances with the incoming deck.
    p.onEnded()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [prefetchIdle])

  // --- Position ticker + crossfade check + periodic server sync ---
  useEffect(() => {
    const tick = setInterval(() => {
      const active = deckRefs[activeIdxRef.current].current
      if (!active) return
      if (!active.paused) setPosition(active.currentTime)
      maybeStartCrossfade(active)
    }, TICK_INTERVAL_MS)
    return () => clearInterval(tick)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [maybeStartCrossfade])

  useEffect(() => {
    if (!isPlaying) return
    const id = setInterval(() => {
      const active = deckRefs[activeIdxRef.current].current
      if (active && !active.paused) onSync(active.currentTime)
    }, SYNC_INTERVAL_MS)
    return () => clearInterval(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isPlaying, onSync])

  // --- End-of-track on either deck ---
  useEffect(() => {
    const handlers = deckRefs.map((ref, idx) => {
      const el = ref.current
      if (!el) return null
      const handler = () => {
        const endedId = loadedRef.current[idx]
        // The outgoing deck of a crossfade already advanced the queue.
        if (fadingFromRef.current && fadingFromRef.current === endedId) return
        if (idx === activeIdxRef.current) paramsRef.current.onEnded()
      }
      el.addEventListener('ended', handler)
      return { el, handler }
    })
    return () => {
      handlers.forEach((h) => h && h.el.removeEventListener('ended', h.handler))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeIdx])

  // --- Stop everything when the player leaves the screen ---
  useEffect(() => {
    const a = deckA.current
    const b = deckB.current
    return () => {
      if (fadeTimerRef.current) clearInterval(fadeTimerRef.current)
      for (const d of [a, b]) {
        if (!d) continue
        d.pause()
        d.removeAttribute('src')
        d.load()
      }
    }
  }, [])

  const seek = useCallback((seconds: number) => {
    cancelFade()
    const active = deckRefs[activeIdxRef.current].current
    if (!active) return
    active.currentTime = seconds
    setPosition(seconds)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cancelFade])

  const unlock = useCallback(() => {
    deckRefs[activeIdxRef.current].current?.play().then(() => setNeedsGesture(false)).catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return { deckRefs, position, duration, needsGesture, crossfading, unlock, seek }
}

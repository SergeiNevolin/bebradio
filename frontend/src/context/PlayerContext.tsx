import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useLocation } from 'react-router-dom'
import { useMashupPlayer, type MashupPlayer as MashupPlayerState } from '../hooks/useMashupPlayer'
import { useAuth } from './AuthContext'
import { useToast } from './ToastContext'
import { api } from '../lib/api'
import type { KaraokePreview } from '../lib/karaokePreview'
import type { Track } from '../types'

interface PlayerContextValue {
  /** Shared player state. `toggle` is room-guarded (see below). */
  player: MashupPlayerState
  queueOpen: boolean
  setQueueOpen: (v: boolean | ((prev: boolean) => boolean)) => void
  expanded: boolean
  setExpanded: (v: boolean) => void
  openExpanded: () => void
  /** User id whose profile overlay is open (track owners). */
  profileUserId: string | null
  setProfileUserId: (id: string | null) => void
  /** Песня караоке, открытая в панели превью (справа). */
  previewSong: KaraokePreview | null
  setPreviewSong: (song: KaraokePreview | null) => void
  /** Merge tracks into the shared walkable list (dedupe by id). */
  registerTracks: (tracks: Track[]) => void
  /** Play, adding to the list first when missing. Blocked inside rooms. */
  playTrack: (track: Track, opts?: { blockedMessage?: string }) => void
  /** Toggle when current, otherwise play. */
  toggleTrack: (track: Track, opts?: { blockedMessage?: string }) => void
  /** Optimistic like/unlike with a single API call. Null when it failed. */
  toggleLike: (track: Track) => Promise<{ liked: boolean; likes: number } | null>
  /** Apply like overrides to a page-local track object for display. */
  applyLike: (track: Track) => Track
}

function stubPlayer(): MashupPlayerState {
  const noop = () => {}
  return {
    audioRef: { current: null },
    list: [],
    setList: () => {},
    index: -1,
    current: null,
    isPlaying: false,
    position: 0,
    duration: 0,
    buffered: 0,
    shuffle: false,
    repeat: 'off',
    volume: 1,
    muted: false,
    setVolume: noop,
    toggleMute: noop,
    toggleShuffle: noop,
    cycleRepeat: noop,
    play: noop,
    playAt: noop,
    toggle: noop,
    pause: noop,
    next: noop,
    prev: noop,
    seek: noop,
  }
}

const PlayerContext = createContext<PlayerContextValue>({
  player: stubPlayer(),
  queueOpen: false,
  setQueueOpen: () => {},
  expanded: false,
  setExpanded: () => {},
  openExpanded: () => {},
  profileUserId: null,
  setProfileUserId: () => {},
  previewSong: null,
  setPreviewSong: () => {},
  registerTracks: () => {},
  playTrack: () => {},
  toggleTrack: () => {},
  toggleLike: noopAsyncNull,
  applyLike: (t) => t,
})

async function noopAsyncNull(): Promise<{ liked: boolean; likes: number } | null> {
  return null
}

export function usePlayer(): PlayerContextValue {
  return useContext(PlayerContext)
}

/**
 * Маршруты со своим звуком, где общий плеер прячется: комнаты (эфир)
 * и караоке (своё приложение в iframe).
 */
export function isPlayerHiddenPath(pathname: string): boolean {
  return (
    pathname.startsWith('/room/') ||
    pathname === '/karaoke' ||
    pathname.startsWith('/karaoke/')
  )
}

function defaultBlockedMessage(pathname: string): string {
  return pathname.startsWith('/room/')
    ? 'В комнате играет эфир — мешапы слушайте вне комнат'
    : 'В караоке свой звук — мешапы слушайте вне караоке'
}

/**
 * Single global player. Mounted once in App so playback survives
 * navigation between Home, Mashups and everywhere else. Pages feed their
 * tracks via registerTracks() and start playback via playTrack().
 */
export function PlayerProvider({ children }: { children: ReactNode }) {
  const player = useMashupPlayer()
  const { user } = useAuth()
  const { showToast } = useToast()
  const location = useLocation()
  const [queueOpen, setQueueOpen] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const [profileUserId, setProfileUserId] = useState<string | null>(null)
  const [previewSong, setPreviewSong] = useState<KaraokePreview | null>(null)
  const [likeOverrides, setLikeOverrides] = useState<Record<string, { liked: boolean; likes: number }>>({})
  const { setList } = player
  const overridesRef = useRef(likeOverrides)
  overridesRef.current = likeOverrides

  const registerTracks = useCallback(
    (tracks: Track[]) => {
      if (tracks.length === 0) return
      setList((prev) => {
        const seen = new Set(prev.map((t) => t.id))
        const fresh = tracks.filter((t) => !seen.has(t.id))
        return fresh.length > 0 ? [...prev, ...fresh] : prev
      })
    },
    [setList],
  )

  // A play requested for a track that is not in the shared list yet.
  const pendingIdRef = useRef<string | null>(null)
  const listRef = useRef<Track[]>([])
  const playFnRef = useRef(player.play)
  playFnRef.current = player.play
  useEffect(() => {
    listRef.current = player.list
  }, [player.list])

  const playTrack = useCallback(
    (track: Track, opts?: { blockedMessage?: string }) => {
      if (isPlayerHiddenPath(location.pathname)) {
        showToast(
          opts?.blockedMessage ?? defaultBlockedMessage(location.pathname),
          'error',
        )
        return
      }
      if (listRef.current.some((x) => x.id === track.id)) {
        playFnRef.current(track)
      } else {
        pendingIdRef.current = track.id
        setList((prev) => (prev.some((x) => x.id === track.id) ? prev : [...prev, track]))
      }
    },
    [location.pathname, setList, showToast],
  )

  // Once the pending track lands in the list, hand it to the player.
  // `player.play` looks the track up by id and the load effect auto-plays.
  const listLength = player.list.length
  useEffect(() => {
    const id = pendingIdRef.current
    if (!id) return
    const found = player.list.find((x) => x.id === id)
    if (found) {
      pendingIdRef.current = null
      playFnRef.current(found)
    }
  }, [player.list, listLength])

  const guardedToggle = useCallback(() => {
    if (!player.isPlaying && player.current && isPlayerHiddenPath(location.pathname)) {
      showToast(defaultBlockedMessage(location.pathname), 'error')
      return
    }
    player.toggle()
  }, [player, location.pathname, showToast])

  const toggleTrack = useCallback(
    (track: Track, opts?: { blockedMessage?: string }) => {
      if (player.current?.id === track.id) guardedToggle()
      else playTrack(track, opts)
    },
    [player, guardedToggle, playTrack],
  )

  const toggleLike = useCallback(
    async (track: Track): Promise<{ liked: boolean; likes: number } | null> => {
      if (!user) {
        showToast('Sign in to like mashups', 'error')
        return null
      }
      const base = overridesRef.current[track.id] ?? { liked: !!track.liked, likes: track.likes }
      const next = { liked: !base.liked, likes: Math.max(0, base.likes + (base.liked ? -1 : 1)) }
      setLikeOverrides((prev) => ({ ...prev, [track.id]: next }))
      setList((prev) => prev.map((x) => (x.id === track.id ? { ...x, ...next } : x)))
      try {
        const res = next.liked ? await api.likeTrack(track.id) : await api.unlikeTrack(track.id)
        const settled = { liked: res.liked, likes: res.likes }
        setLikeOverrides((prev) => ({ ...prev, [track.id]: settled }))
        setList((prev) => prev.map((x) => (x.id === track.id ? { ...x, ...settled } : x)))
        return settled
      } catch (err) {
        setLikeOverrides((prev) => ({ ...prev, [track.id]: base }))
        setList((prev) => prev.map((x) => (x.id === track.id ? { ...x, ...base } : x)))
        showToast(err instanceof Error ? err.message : 'Could not update like', 'error')
        return null
      }
    },
    [user, setList, showToast],
  )

  const applyLike = useCallback(
    (track: Track): Track => {
      const o = likeOverrides[track.id]
      return o ? { ...track, ...o } : track
    },
    [likeOverrides],
  )

  const openExpanded = useCallback(() => {
    setExpanded(true)
  }, [])

  const valuePlayer = useMemo(
    () => ({ ...player, toggle: guardedToggle }),
    [player, guardedToggle],
  )

  const value = useMemo<PlayerContextValue>(
    () => ({
      player: valuePlayer,
      queueOpen,
      setQueueOpen,
      expanded,
      setExpanded,
      openExpanded,
      profileUserId,
      setProfileUserId,
      previewSong,
      setPreviewSong,
      registerTracks,
      playTrack,
      toggleTrack,
      toggleLike,
      applyLike,
    }),
    [
      valuePlayer,
      queueOpen,
      expanded,
      openExpanded,
      profileUserId,
      registerTracks,
      previewSong,
      playTrack,
      toggleTrack,
      toggleLike,
      applyLike,
    ],
  )

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>
}

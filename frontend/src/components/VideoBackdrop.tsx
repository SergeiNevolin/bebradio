import { useEffect, useRef, useState } from 'react'

interface YTPlayer {
  loadVideoById: (id: string) => void
  playVideo: () => void
  pauseVideo: () => void
  mute: () => void
  seekTo: (seconds: number, allowSeekAhead: boolean) => void
  getCurrentTime: () => number
  destroy: () => void
}

interface YTApi {
  Player: new (
    el: HTMLElement | string,
    opts: Record<string, unknown>,
  ) => YTPlayer
}

declare global {
  interface Window {
    YT?: YTApi
    onYouTubeIframeAPIReady?: () => void
  }
}

let apiPromise: Promise<YTApi | undefined> | null = null

/** Load the YouTube IFrame Player API once, shared across mounts. */
function loadYouTubeApi(): Promise<YTApi | undefined> {
  if (apiPromise) return apiPromise
  apiPromise = new Promise((resolve) => {
    if (window.YT?.Player) {
      resolve(window.YT)
      return
    }
    const previous = window.onYouTubeIframeAPIReady
    window.onYouTubeIframeAPIReady = () => {
      previous?.()
      resolve(window.YT)
    }
    if (!document.querySelector('script[src*="youtube.com/iframe_api"]')) {
      const tag = document.createElement('script')
      tag.src = 'https://www.youtube.com/iframe_api'
      document.head.appendChild(tag)
    }
  })
  return apiPromise
}

const DRIFT_TOLERANCE_S = 2.5
const SYNC_INTERVAL_MS = 4000

interface VideoBackdropProps {
  /** 11-char YouTube id of the current track, or '' for none. */
  videoId: string
  isPlaying: boolean
  /** Authoritative playback position in seconds. */
  position: number
}

/**
 * The current track's YouTube video, muted and looping, rendered full-bleed
 * behind the room as a blurred, dimmed backdrop. Audio still comes from the
 * synced <audio> decks; this is purely decorative and never captures input.
 */
export default function VideoBackdrop({ videoId, isPlaying, position }: VideoBackdropProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const playerRef = useRef<YTPlayer | null>(null)
  const loadedIdRef = useRef('')
  const [failed, setFailed] = useState(false)

  // Latest values for async / interval callbacks.
  const liveRef = useRef({ isPlaying, position })
  liveRef.current = { isPlaying, position }

  // Create the player once the API and a video id are available.
  useEffect(() => {
    if (!videoId) return
    let cancelled = false

    loadYouTubeApi().then((YT) => {
      if (cancelled || !YT || !hostRef.current || playerRef.current) return
      playerRef.current = new YT.Player(hostRef.current, {
        videoId,
        host: 'https://www.youtube-nocookie.com',
        playerVars: {
          autoplay: 1,
          controls: 0,
          disablekb: 1,
          fs: 0,
          modestbranding: 1,
          playsinline: 1,
          rel: 0,
          iv_load_policy: 3,
          loop: 1,
          playlist: videoId,
        },
        events: {
          onReady: (e: { target: YTPlayer }) => {
            loadedIdRef.current = videoId
            e.target.mute()
            if (liveRef.current.isPlaying) e.target.playVideo()
            else e.target.pauseVideo()
          },
          onError: () => setFailed(true),
        },
      })
    })

    return () => {
      cancelled = true
    }
  }, [videoId])

  // Swap the clip when the track changes.
  useEffect(() => {
    const player = playerRef.current
    if (!player || !videoId || loadedIdRef.current === videoId) return
    setFailed(false)
    loadedIdRef.current = videoId
    player.loadVideoById(videoId)
    player.mute()
  }, [videoId])

  // Follow the room's play / pause.
  useEffect(() => {
    const player = playerRef.current
    if (!player) return
    if (isPlaying) player.playVideo()
    else player.pauseVideo()
  }, [isPlaying, videoId])

  // Coarse drift correction against the server position.
  useEffect(() => {
    const id = setInterval(() => {
      const player = playerRef.current
      if (!player || !liveRef.current.isPlaying) return
      try {
        if (Math.abs(player.getCurrentTime() - liveRef.current.position) > DRIFT_TOLERANCE_S) {
          player.seekTo(liveRef.current.position, true)
        }
      } catch {
        /* player not ready yet */
      }
    }, SYNC_INTERVAL_MS)
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    return () => {
      try {
        playerRef.current?.destroy()
      } catch {
        /* ignore */
      }
      playerRef.current = null
      loadedIdRef.current = ''
    }
  }, [])

  if (!videoId || failed) return null

  return (
    <div className="video-backdrop" aria-hidden="true">
      <div className="video-backdrop-frame" ref={hostRef} />
    </div>
  )
}
